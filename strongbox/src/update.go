package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// updating, re-installing and installing specific releases of installed addons.
// clj: `cli.clj/update-selected`, `update-all`, `re-install-or-update`, `set-version`

// set while an update all is running, so another is refused rather than queued.
func update_all_running(app *core.App) *atomic.Bool { return app_flag(app, "update-all-running") }

// returns `a` ready to install: strongbox-installed addons keep their group ID, an addon
// strongbox did not install takes its catalogue URL, or its directory name, as one.
func install_target(a Addon) Addon {
	if a.NFO != nil && a.NFO.GroupID != "" {
		return a
	}
	group_id := a.URL
	if group_id == "" {
		group_id = "dir:" + a.DirName
	}
	nfo := NFO{GroupID: group_id, Ignored: a.Ignored}
	installed := MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, &nfo, a.CatalogueAddon, a.SourceUpdateList)
	installed.SourceUpdate = a.SourceUpdate
	return installed
}

// returns the releases of `a` offered for its addons dir's game track and strictness,
// newest first.
func addon_releases(a Addon) []SourceUpdate {
	if a.AddonsDir == nil {
		return []SourceUpdate{}
	}
	return _make_addon__filter_source_updates(a.SourceUpdateList, a.AddonsDir.GameTrackID, a.AddonsDir.Strict)
}

// installs the release `release` of the addon in the result `id`, marking it busy.
func install_release(app *core.App, id string, a Addon, release SourceUpdate) error {
	target := install_target(a)
	target.SourceUpdate = &release
	set_busy(app, id, true)
	err := download_and_install(app, *a.AddonsDir, target, InstallOpts{})
	if err != nil {
		set_busy(app, id, false)
		return err
	}
	if err := ReloadAddonsDir(app, *a.AddonsDir); err != nil {
		slog.Warn("updated addon but failed to reload addons dir", "error", err)
	}
	return nil
}

// returns the addon in the result `id`, or an error when there is none.
func addon_by_id(app *core.App, id string) (Addon, error) {
	r := app.GetResult(id)
	if r == nil {
		return Addon{}, fmt.Errorf("addon is gone: %s", id)
	}
	a, ok := r.Item.(Addon)
	if !ok || a.AddonsDir == nil {
		return Addon{}, fmt.Errorf("not an installed addon: %s", id)
	}
	return a, nil
}

// updates the addons in the results `id_list` that can be updated, one at a time, as a
// job. the others are skipped. a failure is reported and the rest are still updated.
// returns the failures joined.
func UpdateAddons(app *core.App, id_list []string) error {
	job := app.StartJob("updating", len(id_list))
	defer job.Finish()
	err_list := []error{}
	for _, id := range id_list {
		func() {
			defer job.Tick(1)
			a, err := addon_by_id(app, id)
			if err != nil {
				err_list = append(err_list, err)
				return
			}
			if !Updateable(a) {
				return
			}
			if err := install_release(app, id, a, *a.SourceUpdate); err != nil {
				slog.Warn("failed to update addon", "addon", a.Label, "error", err)
				err_list = append(err_list, fmt.Errorf("%s: %w", a.Label, err))
				return
			}
			slog.Info("updated addon", "addon", a.Label, "version", a.SourceUpdate.Version)
		}()
	}
	return errors.Join(err_list...)
}

var ErrUpdatesInProgress = errors.New("updates in progress, 'update all' ignored")

// updates every updateable addon in the selected addons dir.
// refused while another update all is running.
// clj: `cli.clj/update-all`
func UpdateAll(app *core.App) error {
	if !update_all_running(app).CompareAndSwap(false, true) {
		return ErrUpdatesInProgress
	}
	defer update_all_running(app).Store(false)
	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		return err
	}
	id_list := []string{}
	for _, r := range updateable_addons(app, addons_dir) {
		id_list = append(id_list, r.ID)
	}
	return UpdateAddons(app, id_list)
}

// returns `true` when `a` can be re-installed: it is installed, not ignored, and has a
// source to install from.
// clj: `addon.clj/re-installable?`
func re_installable(a Addon) bool {
	return len(a.InstalledAddonGroup) > 0 && !a.IsIgnored && a.Source != "" && a.SourceID != ""
}

// returns the release to re-install `a` from: the one at its installed version when the
// host still offers it, else its chosen update, and whether the installed version was
// found.
// clj: `cli.clj/-find-replace-release`
func reinstall_release(a Addon) (SourceUpdate, bool, bool) {
	for _, su := range addon_releases(a) {
		if su.Version == a.InstalledVersion {
			return su, true, true
		}
	}
	if a.SourceUpdate != nil {
		return *a.SourceUpdate, false, true
	}
	return SourceUpdate{}, false, false
}

// re-installs the addons in the results `id_list`, each at its installed version when
// the host still offers it. addons that cannot be re-installed are skipped.
// returns the failures joined.
func ReinstallAddons(app *core.App, id_list []string) error {
	job := app.StartJob("re-installing", len(id_list))
	defer job.Finish()
	err_list := []error{}
	for _, id := range id_list {
		func() {
			defer job.Tick(1)
			a, err := addon_by_id(app, id)
			if err != nil || !re_installable(a) {
				return
			}
			release, same_version, ok := reinstall_release(a)
			if !ok {
				err_list = append(err_list, fmt.Errorf("%s: no release to re-install", a.Label))
				return
			}
			if !same_version {
				slog.Warn("installed version is no longer available, installing the latest instead", "addon", a.Label, "installed-version", a.InstalledVersion, "version", release.Version)
			}
			if err := install_release(app, id, a, release); err != nil {
				err_list = append(err_list, fmt.Errorf("%s: %w", a.Label, err))
			}
		}()
	}
	return errors.Join(err_list...)
}

// installs the release `version` of the addon in the result `id`.
// refused for pinned and ignored addons, and when the release is not offered for the
// addons dir's game track and strictness.
// clj: `cli.clj/set-version`
func InstallAddonRelease(app *core.App, id string, version string) error {
	a, err := addon_by_id(app, id)
	if err != nil {
		return err
	}
	if a.IsIgnored || a.IsPinned {
		return fmt.Errorf("%s is ignored or pinned, its release cannot be changed", a.Label)
	}
	for _, su := range addon_releases(a) {
		if su.Version == version {
			return install_release(app, id, a, su)
		}
	}
	return fmt.Errorf("%s has no release %s for this addons directory", a.Label, version)
}
