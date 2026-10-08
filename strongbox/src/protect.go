package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
)

// ignoring, pinning and switching the source of installed addons: each edits the nfo
// data of every directory of an addon.
// clj: `addon.clj/ignore!`, `clear-ignore!`, `pin!`, `unpin!`, `switch-source!`

// changes the nfo data of every directory of `a` with `fn`, under its addons dir's lock,
// then reloads the addons dir.
// `fn` is given each directory's nfo data and whether the directory is implicitly ignored.
func edit_addon_nfo(app *core.App, a Addon, fn func(f NFOFile, implicit bool) NFOFile) error {
	err := func() error {
		unlock := lock_addons_dir(a.AddonsDir.Path)
		defer unlock()
		for _, ia := range a.InstalledAddonGroup {
			dir := filepath.Join(a.AddonsDir.Path, ia.DirName)
			f, err := read_nfo_file(dir)
			if err != nil && !errors.Is(err, ErrNFODNE) {
				return err
			}
			if err := write_nfo_file(dir, fn(f, ia.ImplicitlyIgnored())); err != nil {
				return err
			}
		}
		return nil
	}()
	if rerr := ReloadAddonsDir(app, *a.AddonsDir); rerr != nil {
		slog.Warn("failed to reload addons dir", "error", rerr)
	}
	return err
}

// applies `edit` to each addon in `result_list` for which `allowed` is true, skipping the
// rest. returns the failures joined.
func edit_addons(app *core.App, result_list []*core.Result, allowed func(Addon) bool, edit func(Addon) error) error {
	err_list := []error{}
	for _, r := range result_list {
		a, ok := r.Item.(Addon)
		if !ok || a.AddonsDir == nil || !allowed(a) {
			continue
		}
		if err := edit(a); err != nil {
			err_list = append(err_list, fmt.Errorf("%s: %w", a.Label, err))
		}
	}
	return errors.Join(err_list...)
}

// returns `true` when `a` can be ignored: it is installed and not ignored.
// clj: `addon.clj/ignorable?`
func ignorable(a Addon) bool {
	return len(a.InstalledAddonGroup) > 0 && !a.IsIgnored
}

// ignores the addons in `result_list` that are not already ignored.
func IgnoreAddons(app *core.App, result_list []*core.Result) error {
	return edit_addons(app, result_list, ignorable, func(a Addon) error {
		return edit_addon_nfo(app, a, nfo_file_ignore)
	})
}

// stops ignoring the ignored addons in `result_list`. a directory that would be ignored
// anyway is explicitly un-ignored.
func StopIgnoringAddons(app *core.App, result_list []*core.Result) error {
	return edit_addons(app, result_list, func(a Addon) bool { return a.IsIgnored }, func(a Addon) error {
		return edit_addon_nfo(app, a, nfo_file_stop_ignoring)
	})
}

// returns `true` when `a` can be pinned: strongbox installed it, it has an installed
// version, and it is neither ignored nor pinned. a pin is kept in nfo data, which an
// addon strongbox did not install does not have.
// clj: `addon.clj/pinnable?`
func pinnable(a Addon) bool {
	return addon_group_id(a) != "" && a.InstalledVersion != "" && !a.IsIgnored && !a.IsPinned
}

// returns `true` when `a` can be unpinned: it is pinned and not ignored.
// clj: `addon.clj/unpinnable?`
func unpinnable(a Addon) bool {
	return a.IsPinned && !a.IsIgnored
}

// pins the addons in `result_list` that can be pinned at their installed versions.
func PinAddons(app *core.App, result_list []*core.Result) error {
	return edit_addons(app, result_list, pinnable, func(a Addon) error {
		return edit_addon_nfo(app, a, func(f NFOFile, _ bool) NFOFile { return nfo_file_pin(f, a.InstalledVersion) })
	})
}

// unpins the pinned addons in `result_list`.
func UnpinAddons(app *core.App, result_list []*core.Result) error {
	return edit_addons(app, result_list, unpinnable, func(a Addon) error {
		return edit_addon_nfo(app, a, func(f NFOFile, _ bool) NFOFile { return nfo_file_pin(f, "") })
	})
}

// returns `true` when `a` can be switched to another source: it lists more than one, and
// is neither ignored nor pinned.
func source_switchable(a Addon) bool {
	return len(a.SourceMapList) >= 2 && !a.IsIgnored && !a.IsPinned && addon_group_id(a) != ""
}

// switches the addon in the result `r` to `sm`, one of the sources it lists, keeping the
// full source map list, then matches it and checks it for updates from the new source.
// clj: `addon.clj/switch-source!`
func SwitchSource(app *core.App, r *core.Result, sm SourceMap) error {
	a, ok := r.Item.(Addon)
	if !ok || a.AddonsDir == nil {
		return fmt.Errorf("not an installed addon: %s", r.ID)
	}
	if !source_switchable(a) {
		return fmt.Errorf("%s cannot switch source", a.Label)
	}
	if !slices.Contains(a.SourceMapList, sm) {
		return fmt.Errorf("%s is not available from %s %s", a.Label, sm.Source, sm.SourceID)
	}
	full_list := slices.Clone(a.SourceMapList)
	err := edit_addon_nfo(app, a, func(f NFOFile, _ bool) NFOFile {
		return nfo_file_update_top(f, func(n NFO) NFO {
			n.Source = sm.Source
			n.SourceID = sm.SourceID
			n.SourceMapList = full_list
			return n
		})
	})
	if err != nil {
		return err
	}
	if err := Reconcile(app); err != nil {
		slog.Debug("not matched after switching source", "error", err)
	}
	return check_addon(app, r.ID)
}
