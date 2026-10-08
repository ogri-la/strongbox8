package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// installing addons: every install is planned as a pure function of the addons on disk
// and the zip to install, then the plan is carried out.
// clj: `addon.clj/install-addon`, `core.clj/install-addon-guard`

// further options to tweak installation behaviour
type InstallOpts struct {
	OverwriteIgnored bool // install over an ignored addon, keeping it ignored
	UnpinPinned      bool // install over a pinned addon, removing the pin
}

// what installing an addon will do, decided before anything on disk changes.
type InstallPlan struct {
	Uninstall []Addon            // the previous version and any addons the zip completely replaces
	NFOWrites map[string]NFOFile // top-level directory => its nfo data once installed
	Primary   string             // the top-level directory that is the addon's main one, or ""
	Messages  []string           // things the user should be told
}

// --- addons dir locks

// one lock per addons dir: installs and removals in an addons dir run one at a time.
// a map from path to lock, so different addons dirs do not wait on each other.
var addons_dir_locks = struct {
	sync.Mutex
	m map[PathToDir]*sync.Mutex
}{m: map[PathToDir]*sync.Mutex{}}

// locks the addons dir at `path`, returning the function that unlocks it.
func lock_addons_dir(path PathToDir) func() {
	addons_dir_locks.Lock()
	l, present := addons_dir_locks.m[path]
	if !present {
		l = &sync.Mutex{}
		addons_dir_locks.m[path] = l
	}
	addons_dir_locks.Unlock()
	l.Lock()
	return l.Unlock
}

// --- planning

// returns the group ID of `a`, or "" when strongbox did not install it.
func addon_group_id(a Addon) string {
	if a.NFO == nil {
		return ""
	}
	return a.NFO.GroupID
}

// returns the directory names of the members of `a`.
func addon_dir_names(a Addon) mapset.Set[string] {
	names := mapset.NewSet[string]()
	for _, ia := range a.InstalledAddonGroup {
		names.Add(ia.DirName)
	}
	return names
}

// returns the nfo data of every directory in `installed`, directory name => nfo file.
func installed_nfo_files(installed []Addon) map[string]NFOFile {
	idx := map[string]NFOFile{}
	for _, a := range installed {
		for _, ia := range a.InstalledAddonGroup {
			idx[ia.DirName] = ia.NFOFile
		}
	}
	return idx
}

// returns what installing `target` from the zip described by `report` into an addons dir
// holding `installed` will do, or an error explaining why it is refused.
// refused: an invalid zip, or a zip whose directories belong to an ignored or pinned
// addon, unless `opts` allows it. updating a pinned addon to its pinned version is
// allowed and keeps the pin.
// uninstalled first: the previous version of `target`, so directories it no longer
// ships are removed, and every other non-ignored addon all of whose directories the zip
// replaces, which would otherwise become a shared directory for nothing.
// clj: `addon.clj/install-addon`, `remove-completely-overwritten-addons`
func plan_install(installed []Addon, target Addon, report ZipReport, opts InstallOpts) (InstallPlan, error) {
	if err := valid_addon_zip_file(report); err != nil {
		return InstallPlan{}, err
	}
	target_group := addon_group_id(target)
	if target_group == "" {
		return InstallPlan{}, errors.New("addon to install has no group ID")
	}
	top := report.TopLevelDirs

	plan := InstallPlan{NFOWrites: map[string]NFOFile{}}
	keep_ignored := false
	keep_pin := ""
	removed_groups := mapset.NewSet[string]()

	for _, a := range installed {
		dirs := addon_dir_names(a)
		overlaps := dirs.Intersect(top).Cardinality() > 0
		is_previous := target_group != "" && addon_group_id(a) == target_group

		if overlaps && a.IsIgnored {
			if !opts.OverwriteIgnored {
				return InstallPlan{}, fmt.Errorf("refusing to install addon that will overwrite an ignored addon: %s", a.Label)
			}
			keep_ignored = true
		}

		if overlaps && a.IsPinned {
			updating_to_pin := is_previous && target.SourceUpdate != nil && target.SourceUpdate.Version == a.PinnedVersion
			switch {
			case updating_to_pin:
				keep_pin = a.PinnedVersion
			case !opts.UnpinPinned:
				return InstallPlan{}, fmt.Errorf("refusing to install addon that will overwrite a pinned addon: %s", a.Label)
			}
		}

		completely_replaced := !a.IsIgnored && dirs.Cardinality() > 0 && dirs.IsSubset(top)
		if is_previous || (overlaps && completely_replaced) {
			plan.Uninstall = append(plan.Uninstall, a)
			if g := addon_group_id(a); g != "" {
				removed_groups.Add(g)
			}
		}
	}

	primary, err := determine_primary_subdir(top)
	if err != nil {
		slog.Debug("no primary directory", "toplevel-dirs", top.ToSlice(), "error", err)
	}
	plan.Primary = primary

	dir_nfo := installed_nfo_files(installed)
	dir_list := top.ToSlice()
	slices.Sort(dir_list)
	for _, dir := range dir_list {
		nfo := derive_nfo(target, dir == primary)
		if keep_ignored {
			nfo.Ignored = new(true)
		}
		nfo.PinnedVersion = keep_pin

		// the directory's remaining owners, without the addons being uninstalled
		base := dir_nfo[dir]
		for _, g := range removed_groups.ToSlice() {
			base = nfo_file_rm(base, g)
		}
		base.IgnoreFlag = nil
		f, msg := nfo_file_add(base, nfo, dir)
		if msg != "" {
			plan.Messages = append(plan.Messages, msg)
		}
		plan.NFOWrites[dir] = f
	}

	if extra := inconsistently_prefixed(top); len(extra) > 0 {
		plan.Messages = append(plan.Messages, fmt.Sprintf("%s will also install these addons: %s", target.Label, strings.Join(extra, ", ")))
	}

	return plan, nil
}

// --- executing

// carries out `plan`, installing `zipfile` into `addons_dir`, stopping at the first
// failure: nothing is written to nfo data after a failed extraction.
func execute_install(addons_dir AddonsDir, plan InstallPlan, zipfile PathToFile) error {
	for _, a := range plan.Uninstall {
		if err := remove_addon(a, addons_dir); err != nil {
			return fmt.Errorf("failed to remove %s before installing: %w", a.Label, err)
		}
	}
	if _, err := unzip_file(zipfile, addons_dir.Path); err != nil {
		return fmt.Errorf("failed to extract %s: %w", filepath.Base(zipfile), err)
	}
	dir_list := []string{}
	for dir := range plan.NFOWrites {
		dir_list = append(dir_list, dir)
	}
	slices.Sort(dir_list)
	for _, dir := range dir_list {
		if err := write_nfo_file(filepath.Join(addons_dir.Path, dir), plan.NFOWrites[dir]); err != nil {
			return err
		}
	}
	for _, msg := range plan.Messages {
		slog.Info(msg)
	}
	return nil
}

// installs `zipfile` into `addons_dir` for `target`, under the addons dir's lock.
// refuses, changing nothing, anything `plan_install` refuses.
func install_zip(addons_dir AddonsDir, target Addon, zipfile PathToFile, opts InstallOpts) error {
	unlock := lock_addons_dir(addons_dir.Path)
	defer unlock()

	report, err := inspect_zipfile(zipfile)
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	installed, err := LoadAllInstalledAddons(addons_dir)
	if err != nil {
		return fmt.Errorf("failed to read addons directory: %w", err)
	}
	plan, err := plan_install(installed, target, report, opts)
	if err != nil {
		return fmt.Errorf("refusing to install: %w", err)
	}
	return execute_install(addons_dir, plan, zipfile)
}

// --- downloads

// returns the file name a download of `name` at `version` is saved as, both made safe
// for file names. "EveryAddon", "1.2.3" => "everyaddon--1-2-3.zip"
func downloaded_addon_fname(name string, version string) string {
	return fmt.Sprintf("%s--%s.zip", slugify(name), slugify(version))
}

// downloads the update chosen for `a` into `addons_dir`, returning where it was saved.
// returns an error when `a` has no update or the download fails.
func download_addon_update(app *core.App, addons_dir AddonsDir, a Addon) (PathToFile, error) {
	if a.SourceUpdate == nil {
		return "", fmt.Errorf("%s has no update to download", a.Label)
	}
	output_path := filepath.Join(addons_dir.Path, downloaded_addon_fname(a.Name, a.SourceUpdate.Version))
	if err := app.DownloadFile(a.SourceUpdate.DownloadURL, output_path); err != nil {
		return "", fmt.Errorf("failed to download %s: %w", a.Label, err)
	}
	return output_path, nil
}

// returns an error when the downloaded `zipfile` is not a valid addon zip, deleting it:
// a host may answer with a web page instead of a zip.
func check_downloaded_zip(zipfile PathToFile) error {
	report, err := inspect_zipfile(zipfile)
	if err == nil {
		err = valid_addon_zip_file(report)
	} else {
		err = fmt.Errorf("the download was not a valid zip file: %w", err)
	}
	if err != nil {
		os.Remove(zipfile)
	}
	return err
}

// downloads the update chosen for `target` and installs it into `addons_dir`, then
// prunes downloaded zips. a download that is not a valid addon zip is deleted.
func download_and_install(app *core.App, addons_dir AddonsDir, target Addon, opts InstallOpts) error {
	zipfile, err := download_addon_update(app, addons_dir, target)
	if err != nil {
		return err
	}
	if err := check_downloaded_zip(zipfile); err != nil {
		return err
	}
	if err := install_zip(addons_dir, target, zipfile, opts); err != nil {
		return err
	}
	prune_zip_files(addons_dir, target.Name, FindSettings(app).Preferences.AddonZipsToKeep)
	return nil
}

// --- zip pruning

// a downloaded zip as found in an addons dir.
type zip_file_info struct {
	Name    string
	ModTime int64
}

// returns the names of the downloaded zips of the addon `name` to delete, keeping the
// `keep` most recently modified. nil `keep` keeps them all.
// only files named as strongbox names downloads of that addon are considered.
// clj: `addon.clj/remove-zip-files!`
func zips_to_prune(file_list []zip_file_info, name string, keep *int) []string {
	if keep == nil {
		return nil
	}
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(slugify(name)) + `--.+\.zip$`)
	matching := []zip_file_info{}
	for _, f := range file_list {
		if pattern.MatchString(f.Name) {
			matching = append(matching, f)
		}
	}
	slices.SortFunc(matching, func(a, b zip_file_info) int { return int(b.ModTime - a.ModTime) })
	prune := []string{}
	for i, f := range matching {
		if i >= *keep {
			prune = append(prune, f.Name)
		}
	}
	return prune
}

// deletes the downloaded zips of the addon `name` in `addons_dir` beyond the newest
// `keep`. a failure is logged, never returned: the install already succeeded.
func prune_zip_files(addons_dir AddonsDir, name string, keep *int) {
	if keep == nil {
		return
	}
	entry_list, err := os.ReadDir(addons_dir.Path)
	if err != nil {
		slog.Warn("failed to list addons dir for pruning zips", "error", err)
		return
	}
	file_list := []zip_file_info{}
	for _, entry := range entry_list {
		if entry.Type().IsRegular() {
			if info, err := entry.Info(); err == nil {
				file_list = append(file_list, zip_file_info{Name: entry.Name(), ModTime: info.ModTime().UnixNano()})
			}
		}
	}
	for _, file_name := range zips_to_prune(file_list, name, keep) {
		path := filepath.Join(addons_dir.Path, file_name)
		if err := os.Remove(path); err != nil {
			slog.Warn("failed to delete downloaded zip", "path", path, "error", err)
		}
	}
}

// --- installing from the catalogue, a URL or a zip file

// returns the installed addon in the selected addons dir matched to the catalogue addon
// `ca`, if any.
func installed_match(app *core.App, addons_dir AddonsDir, ca CatalogueAddon) (core.Result, bool) {
	for _, r := range installed_addons(app, addons_dir) {
		a := r.Item.(Addon)
		matched := a.CatalogueAddon != nil && a.CatalogueAddon.Key() == ca.Key()
		same_source := a.Source == ca.Source && a.SourceID == string(ca.SourceID)
		if matched || same_source {
			return r, true
		}
	}
	return core.Result{}, false
}

// returns the available addons dir to install into, or an error telling the user to add
// one.
func install_target_dir(app *core.App) (AddonsDir, error) {
	ad, err := selected_addon_dir(app)
	if err != nil || !ad.Available() {
		return AddonsDir{}, errors.New("no addons directory to install into, add one with 'File' > 'New addons directory'")
	}
	return ad, nil
}

// reloads `addons_dir` after an install, matches it and gives the addon installed from
// `ca` the updates already found.
func after_install(app *core.App, addons_dir AddonsDir, group_id string, sul []SourceUpdate) {
	if err := ReloadAddonsDir(app, addons_dir); err != nil {
		slog.Warn("installed addon but failed to reload addons dir", "error", err)
		return
	}
	if err := Reconcile(app); err != nil {
		slog.Debug("not matched after install", "error", err)
	}
	update_idx := map[string]Addon{}
	for _, r := range installed_addons(app, addons_dir) {
		a := r.Item.(Addon)
		if addon_group_id(a) == group_id && len(sul) > 0 {
			update_idx[r.ID] = MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, a.NFO, a.CatalogueAddon, sul)
		}
	}
	replace_addons_in_state(app, update_idx)
}

// installs the catalogue addon `ca` into the selected addons dir: its update for the
// addons dir's game track and strictness is downloaded and installed. an addon already
// installed and matched to `ca` is updated instead.
// clj: `cli.clj/install-addon`
func InstallCatalogueAddon(app *core.App, ca CatalogueAddon) error {
	addons_dir, err := install_target_dir(app)
	if err != nil {
		return err
	}
	if r, installed := installed_match(app, addons_dir, ca); installed {
		slog.Info("addon is already installed, updating it instead", "addon", ca.Label)
		return UpdateAddons(app, []string{r.ID})
	}
	if !SUPPORTED_HOSTS.Contains(ca.Source) {
		return fmt.Errorf("addon '%s' is from an unsupported source '%s'", ca.Label, ca.Source)
	}

	sul, err := ExpandSummary(app, ca.Source, expand_request_for(ca))
	if err != nil {
		return fmt.Errorf("failed to find updates for %s: %w", ca.Label, err)
	}
	target := MakeAddonFromCatalogueAddon(addons_dir, ca, sul)
	if target.SourceUpdate == nil {
		return errors.New(no_release_message(addons_dir, ca.Source))
	}

	if err := download_and_install(app, addons_dir, target, InstallOpts{}); err != nil {
		return err
	}
	slog.Info("installed addon", "addon", target.Label, "version", target.SourceUpdate.Version)
	after_install(app, addons_dir, addon_group_id(target), sul)
	return nil
}

// installs each of the catalogue addons `ca_list`, one at a time, as a job.
// a failure is reported and the rest are still installed. returns the failures joined.
// clj: `cli.clj/install-many`
func InstallCatalogueAddons(app *core.App, ca_list []CatalogueAddon) error {
	job := app.StartJob("installing", len(ca_list))
	defer job.Finish()
	err_list := []error{}
	for _, ca := range ca_list {
		if err := InstallCatalogueAddon(app, ca); err != nil {
			slog.Warn("failed to install addon", "addon", ca.Label, "error", err)
			err_list = append(err_list, fmt.Errorf("%s: %w", ca.Label, err))
		}
		job.Tick(1)
	}
	return errors.Join(err_list...)
}

// installs the addon at the GitHub, GitLab or WoWInterface URL `raw_url` into the selected
// addons dir, adding it to the user catalogue.
// its update is chosen relaxed for the addons dir's game track. the download is checked
// to be a valid addon zip before anything changes.
// clj: `cli.clj/import-addon`, `core.clj/find-addon`
func InstallAddonFromURL(app *core.App, raw_url string) error {
	addons_dir, err := install_target_dir(app)
	if err != nil {
		return err
	}
	source, source_id, err := ParseAddonURL(raw_url)
	if err != nil {
		return err
	}
	host, err := addon_source(source)
	if err != nil {
		return err
	}
	ca, err := host.FindAddon(app, source_id)
	if err != nil {
		return fmt.Errorf("failed to find addon: %w", err)
	}
	sul, err := ExpandSummary(app, ca.Source, expand_request_for(ca))
	if err != nil {
		return fmt.Errorf("failed to find updates for %s: %w", ca.Label, err)
	}

	relaxed := addons_dir
	relaxed.Strict = false
	target := MakeAddonFromCatalogueAddon(relaxed, ca, sul)
	if target.SourceUpdate == nil {
		return errors.New(no_release_message(relaxed, ca.Source))
	}

	zipfile, err := download_addon_update(app, addons_dir, target)
	if err != nil {
		return err
	}
	if err := check_downloaded_zip(zipfile); err != nil {
		return err
	}
	if err := StarCatalogueAddon(app, ca); err != nil {
		return err
	}
	if err := install_zip(addons_dir, target, zipfile, InstallOpts{}); err != nil {
		return err
	}
	prune_zip_files(addons_dir, target.Name, FindSettings(app).Preferences.AddonZipsToKeep)
	slog.Info("installed addon from URL", "addon", target.Label, "url", raw_url)
	after_install(app, addons_dir, addon_group_id(target), sul)
	return nil
}

// installs the local zip file `zipfile` into the selected addons dir.
// installing over an ignored addon keeps it ignored, over a pinned addon removes the pin.
// the zip file is never deleted.
// clj: `cli.clj/install-addons-from-file-in-parallel`
func InstallAddonFromZip(app *core.App, zipfile PathToFile) error {
	addons_dir, err := install_target_dir(app)
	if err != nil {
		return err
	}
	target, err := MakeAddonFromZipfile(addons_dir, zipfile)
	if err != nil {
		return err
	}
	if err := install_zip(addons_dir, target, zipfile, InstallOpts{OverwriteIgnored: true, UnpinPinned: true}); err != nil {
		return err
	}
	slog.Info("installed addon from file", "zipfile", zipfile)
	after_install(app, addons_dir, addon_group_id(target), nil)
	return nil
}

// installs each of the local zip files in `zipfile_list`, one at a time, as a job.
// returns the failures joined.
func InstallAddonsFromZips(app *core.App, zipfile_list []PathToFile) error {
	job := app.StartJob("installing from file", len(zipfile_list))
	defer job.Finish()
	err_list := []error{}
	for _, zipfile := range zipfile_list {
		if err := InstallAddonFromZip(app, zipfile); err != nil {
			slog.Warn("failed to install addon from file", "zipfile", zipfile, "error", err)
			err_list = append(err_list, fmt.Errorf("%s: %w", filepath.Base(zipfile), err))
		}
		job.Tick(1)
	}
	return errors.Join(err_list...)
}
