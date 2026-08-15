package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/sourcegraph/conc/pool"
)

func default_config_dir() string {
	return core.HomePath("/.config/strongbox8")
}

func default_data_dir() string {
	return core.HomePath("/.local/share/strongbox8")
}

// todo: => bw.utils.Join perhaps
func join(a string, b string) string {
	c, _ := filepath.Abs(filepath.Join(a, b))
	return c
}

// returns the absolute path held in the given XDG `envvar`, suffixed with 'strongbox'
// unless it already ends in it.
// returns an empty string when the variable is unset or cannot be made absolute.
// the error is always nil.
func xdg_path(envvar string) (string, error) {
	xdg_path_str := os.Getenv(envvar)
	if xdg_path_str == "" {
		return xdg_path_str, nil
	}
	xdg_path_str, err := filepath.Abs(xdg_path_str)
	if err != nil {
		slog.Error("error parsing envvar", "envvar", envvar, "error", err)
		return "", nil
	}
	if !strings.HasPrefix(filepath.Base(xdg_path_str), "strongbox") {
		xdg_path_str = join(xdg_path_str, "strongbox") // "/home/.config" => "/home/.config/strongbox"
	}
	return xdg_path_str, nil
}

// returns every filesystem path strongbox uses, keyed by name.
// an empty `config_dir` or `data_dir` falls back to its XDG default.
// every key ends in '-file', '-dir' or '-url'; `init_dirs` creates the '-dir' ones.
// `XDG_CONFIG_DIRS` and `XDG_DATA_DIRS` are not consulted.
// - https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
func generate_path_map(config_dir PathToDir, data_dir PathToDir) map[string]string {
	if config_dir == "" {
		config_dir = default_config_dir()
	}
	if data_dir == "" {
		data_dir = default_data_dir()
	}
	log_dir := join(data_dir, "logs")

	return map[string]string{
		"app.config-dir":                config_dir,
		"app.data-dir":                  data_dir,
		"strongbox.paths.catalogue-dir": data_dir,

		// "/home/$you/.local/share/strongbox/logs"
		"strongbox.paths.log-data-dir": log_dir,
		"strongbox.paths.log-file":     join(log_dir, "debug.log"),

		// "/home/$you/.local/share/strongbox/cache"
		"strongbox.paths.cache-dir": join(data_dir, "cache"),

		// "/home/$you/.config/strongbox/config.json"
		"strongbox.paths.cfg-file": join(config_dir, "config.json"),

		// "/home/$you/.local/share/strongbox/etag-db.json"
		"strongbox.paths.etag-db-file": join(data_dir, "etag-db.json"),

		// todo: move user catalogue to data dir?
		// "/home/$you/.config/strongbox/user-catalogue.json"
		"strongbox.paths.user-catalogue-file": join(config_dir, "user-catalogue.json"),
	}
}

func set_paths(app *core.App, config_dir PathToDir, data_dir PathToDir) map[string]string {
	path_map := generate_path_map(config_dir, data_dir)
	for key, val := range path_map {
		app.State.SetKeyAnyVal(key, val)
	}
	return path_map
}

func get_paths(app *core.App) map[string]string {
	return app.State.SomeKeyVals("strongbox.paths")
}

// creates every '-dir' suffixed path in app state that does not exist yet.
// returns an error when the data directory is missing and cannot be created, when it
// exists but is not writeable, or when a directory cannot be created.
// depends on the paths set by `set_paths`, so it must run after the app has started.
func init_dirs(app *core.App) error {
	data_dir := app.DataDir()

	if !core.PathExists(data_dir) && core.LastWriteableDir(data_dir) == "" {
		// data directory doesn't exist and no parent directory is writable.
		// nowhere to create data dir, nowhere to store download catalogue. non-starter.
		return fmt.Errorf("data directory doesn't exist and it cannot be created: %v", data_dir)
	}

	if core.PathExists(data_dir) && !core.PathIsWriteable(data_dir) {
		// state directory *does* exist but isn't writeable.
		// another non-starter.
		return fmt.Errorf("data directory isn't writeable: %s", data_dir)
	}

	// ensure all '-dir' suffixed paths exist, creating them if necessary.
	for key, val := range app.State.SomeKeyAnyVals("strongbox.paths") {
		val := val.(string)
		if strings.HasSuffix(key, "-dir") && !core.DirExists(val) {
			// "creating directory(s)", "key=data-dir", "val=/path/to/data/dir"
			slog.Debug("creating directory(s)", "key", key, "val", val)
			err := core.MakeDirs(val)
			if err != nil {
				return fmt.Errorf("failed to create '%s' directory: %s", key, val)
			}
		}
	}
	return nil
}

// returns the addons dir whose path is `selected_addon_dir`, or the first available
// addons dir when that path matches none.
// returns an error when no path is given, or when there are no addons dirs at all.
// with several entries for the same path, the first found is returned.
func find_selected_addon_dir(app *core.App, selected_addon_dir string) (AddonsDir, error) {
	empty_result := AddonsDir{}

	if selected_addon_dir == "" {
		return empty_result, fmt.Errorf("no addon directories are selected")
	}

	var selected_addon_dir_ptr *AddonsDir
	results_list := app.FilterResultList(func(result core.Result) bool {
		addon_dir, is_addon_dir := result.Item.(AddonsDir)
		if is_addon_dir && addon_dir.Path == selected_addon_dir {
			selected_addon_dir_ptr = &addon_dir
			return true
		}
		return is_addon_dir
	})

	if len(results_list) == 0 {
		return AddonsDir{}, errors.New("no addon directories found")
	}

	if selected_addon_dir_ptr == nil {
		// there are addon dirs but no addon dir has been selected.
		first_addon_dir := results_list[0].Item.(AddonsDir)

		// todo: update preferences

		return first_addon_dir, nil
	}

	return *selected_addon_dir_ptr, nil
}

// returns the addons dir selected in the settings, see `find_selected_addon_dir`.
func selected_addon_dir(app *core.App) (AddonsDir, error) {
	return find_selected_addon_dir(app, FindSettings(app).Preferences.SelectedAddonsDir)
}

// adds the given `addon_list` to app state, replacing any results with the same IDs.
// blocks until state has been updated.
// clj: `core/update-installed-addon-list!`
func update_installed_addon_list(app *core.App, addon_list []core.Result) {
	app.AddReplaceResults(addon_list...).Wait()
}

// ----

// matches each addon in `installed_addon_list` against the catalogue `db`, attaching the
// catalogue addon to those that match.
// returns every addon given, matched ones first.
// five matchers are tried in order, most reliable first: source+source-id, then source
// against catalogue name, then name, then label, then directory name against label.
// the first matcher to hit wins, so a weaker match is only used when the stronger ones
// find nothing.
// todo: closely coupled to boardwalk `core.Result`.
func _reconcile(db []CatalogueAddon, addons_dir AddonsDir, installed_addon_list []core.Result) []core.Result {

	matched := []core.Result{}
	unmatched := []core.Result{}

	// --- [[:source :source-id] [:source :source-id]] ;; source+source-id, perfect case

	catalogue_addon_source_and_source_id_keyfn := func(catalogue_addon CatalogueAddon) string {
		// "github--AdiBags"
		return fmt.Sprintf("%s--%s", catalogue_addon.Source, catalogue_addon.SourceID)
	}

	addon_source_and_source_id_keyfn := func(addon Addon) string {
		// "github--AdiBags"
		return fmt.Sprintf("%s--%s", addon.Source, addon.SourceID)
	}

	// --- [:source :name] ;; source+name, we have a source but no source-id (nfo v1 files)

	addon_source_keyfn := func(a Addon) string {
		return a.Source
	}

	catalogue_addon_name_keyfn := func(ca CatalogueAddon) string {
		return ca.Name
	}

	// --- [:name :name]

	addon_name_keyfn := func(a Addon) string {
		return a.Name
	}

	// --- [:label :label]

	addon_label_keyfn := func(a Addon) string {
		return a.Label
	}

	catalogue_addon_label_keyfn := func(ca CatalogueAddon) string {
		return ca.Label
	}

	// --- [:dirname :label] ;; dirname == label, eg ./AdiBags == AdiBags

	addon_dirname_keyfn := func(a Addon) string {
		return a.DirName
	}

	// ---

	source_and_source_id_idx := core.Index(db, catalogue_addon_source_and_source_id_keyfn)
	name_idx := core.Index(db, catalogue_addon_name_keyfn)
	label_idx := core.Index(db, catalogue_addon_label_keyfn)

	// ---

	type catalogue_matcher struct {
		idx                   map[string]CatalogueAddon
		addon_keyfn           func(Addon) string
		catalogue_addon_keyfn func(CatalogueAddon) string
	}

	matcher_list := []catalogue_matcher{
		{source_and_source_id_idx, addon_source_and_source_id_keyfn, catalogue_addon_source_and_source_id_keyfn},
		{name_idx, addon_source_keyfn, catalogue_addon_name_keyfn},
		{name_idx, addon_name_keyfn, catalogue_addon_name_keyfn},
		{label_idx, addon_label_keyfn, catalogue_addon_label_keyfn},
		{label_idx, addon_dirname_keyfn, catalogue_addon_label_keyfn},
	}

	// ---

	// todo: this can be done in parallel per-addon
	for _, result := range installed_addon_list {
		addon := result.Item.(Addon)
		success := false
		for _, matcher := range matcher_list {
			addon_key := matcher.addon_keyfn(addon)
			if addon_key == "" {
				continue // try next index
			}
			catalogue_addon, has_match := matcher.idx[addon_key]
			if has_match {
				addon = MakeAddon(addons_dir, addon.InstalledAddonGroup, addon.Primary, addon.NFO, &catalogue_addon, addon.SourceUpdateList)
				result.Item = addon
				matched = append(matched, result)
				success = true
				break // match! move on to next addon
			}
		}
		if !success {
			unmatched = append(unmatched, result)
		}
	}

	if len(unmatched) > 0 {
		slog.Info("not all items reconciled", "len-installed-addon-list", len(installed_addon_list), "len-unmatched", len(unmatched))
	}

	return append(matched, unmatched...)
}

// matches the installed addons in the selected addons dir against the catalogue and
// updates app state with the merged results.
// the user catalogue is searched as well as the main one.
// returns an error when no addons dir is selected or no catalogue is loaded.
// clj: `core.clj/match-all-installed-addons-with-catalogue`
func Reconcile(app *core.App) error {
	slog.Info("Reconcile")
	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		return errors.New("failed to reconcile addons in addons directory: no addons directory selected")
	}

	// --- todo: GetAddons()  or something
	db_result := app.GetResult(ID_CATALOGUE)
	if db_result == nil {
		return errors.New("failed to reconcile addons in addons directory: no catalogue to match installed addons against")
	}

	db := db_result.Item.(Catalogue).AddonSummaryList

	user_db_result := app.GetResult(ID_USER_CATALOGUE)
	if user_db_result != nil {
		user_db := user_db_result.Item.(Catalogue).AddonSummaryList
		db = append(db, user_db...)
	}

	addon_list := installed_addons(app, addons_dir)

	reconciled_addon_list := _reconcile(db, addons_dir, addon_list)

	update_installed_addon_list(app, reconciled_addon_list)

	return nil
}

// returns all `Addon` results attached to the given `addons_dir` in application state.
// panics if an addon in state has no addons dir.
func installed_addons(app *core.App, addons_dir AddonsDir) []core.Result {
	return app.FilterResultList(func(r core.Result) bool {
		if r.NS == NS_ADDON {
			a := r.Item.(Addon)
			if a.AddonsDir == nil {
				slog.Error("Addon in state without an addons dir", "a", a)
				panic("programming error")
			}
			return a.AddonsDir.Path == addons_dir.Path
		}
		return false
	})
}

// returns the `Addon` results in the given `addons_dir` that have an update available.
func updateable_addons(app *core.App, addons_dir AddonsDir) []core.Result {
	updateable_addons_list := []core.Result{}
	for _, r := range installed_addons(app, addons_dir) {
		a := r.Item.(Addon)
		if Updateable(a) {
			updateable_addons_list = append(updateable_addons_list, r)
		}
	}
	return updateable_addons_list
}

// returns the updates available for `source_id` on the given `source`.
// an unsupported source yields no updates and no error.
func ExpandSummary(app *core.App, source Source, source_id string) ([]SourceUpdate, error) {
	empty_response := []SourceUpdate{}
	api_map := map[Source]AddonSource{
		SOURCE_GITHUB: &GithubAPI{},
		SOURCE_WOWI:   &WowinterfaceAPI{},
	}
	api, present := api_map[source]
	if !present {
		return empty_response, nil
	}
	return api.ExpandSummary(app, source_id)
}

// checks every installed addon in the selected addons dir for updates, in parallel,
// tagging those with an update available.
// an addon can only be checked once it has a source, which it gets from catalogue
// matching, so run `Reconcile` first.
// blocks until every addon has been checked. failures are swallowed per-addon.
// clj: `core.clj/check-for-updates`, `core.clj/check-for-updates-in-parallel`
func CheckForUpdates(app *core.App) {
	slog.Info("checking addons for updates")

	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		slog.Warn("no addons directory selected, not checking for updates")
		return
	}

	installed_addon_list := installed_addons(app, addons_dir)

	p := pool.New()
	for _, r := range installed_addon_list {
		p.Go(func() {
			a := r.Item.(Addon)
			source_update_list, err := ExpandSummary(app, a.Source, a.SourceID)
			if err == nil {
				app.UpdateResult(r.ID, func(x core.Result) core.Result {
					a = MakeAddon(addons_dir, a.InstalledAddonGroup, a.Primary, a.NFO, a.CatalogueAddon, source_update_list)
					r.Item = a
					if Updateable(a) {
						r.Tags.Add(core.TAG_HAS_UPDATE)
					}
					return r
				})
			}
		})
	}
	p.Wait() // necessary?
}

// checks the single addon in `r` for updates, tagging it when one is available.
// a failure to reach the addon's source leaves the result untouched.
// blocks until the result has been updated.
func CheckAddon(app *core.App, r *core.Result) {
	a := r.Item.(Addon)
	source_update_list, err := ExpandSummary(app, a.Source, a.SourceID)
	if err != nil {
		return
	}
	wg := app.UpdateResult(r.ID, func(x core.Result) core.Result {
		a = MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, a.NFO, a.CatalogueAddon, source_update_list)
		x.Item = a
		if Updateable(a) {
			x.Tags.Add(core.TAG_HAS_UPDATE)
		}
		return x
	})
	wg.Wait()
}

// downloads the addon at `url` into the given addons dir `ad`, returning the path it was
// written to.
// the file name is derived from `addon_name` and `addon_version`, both of which are
// slugified, so the result is safe to write.
func DownloadAddon(app *core.App, ad AddonsDir, addon_name string, addon_version string, url URL) (string, error) {
	empty_response := ""
	data_dir := ad.Path
	output_file := downloaded_addon_fname(addon_name, addon_version)
	output_path := filepath.Join(data_dir, output_file)
	err := app.DownloadFile(url, output_path)
	if err != nil {
		return empty_response, err
	}
	return output_path, nil
}

// downloads the update chosen for the addon `a` into `addons_dir`, returning the path it
// was written to.
// the update was already chosen when the `Addon` was created, so nothing is picked here.
// returns an error when the addon's source is disabled or it has no update to download.
// does not acquire locks, callers coordinate their own access.
func download_addon_update(app *core.App, addons_dir AddonsDir, a Addon) (PathToFile, error) {
	empty_response := ""

	if DISABLED_HOSTS.Contains(a.Source) {
		return empty_response, fmt.Errorf("source is unsupported: %s", a.Source)
	}

	if a.SourceUpdate == nil {
		return empty_response, fmt.Errorf("no update to download")
	}

	return DownloadAddon(app, addons_dir, a.Name, a.SourceUpdate.Version, a.SourceUpdate.DownloadURL)
}

// downloads the newest update for the catalogue addon `ca` into the addons dir `ad`,
// returning the path it was written to.
// unlike an installed addon, a catalogue addon has no chosen update, so its updates are
// fetched here and the first is taken.
// does not acquire locks, callers coordinate their own access.
// panics if the source reports no updates at all.
func download_catalogue_addon(app *core.App, ad AddonsDir, ca CatalogueAddon) (PathToFile, error) {
	empty_response := ""
	summary_list, err := ExpandSummary(app, ca.Source, string(ca.SourceID))
	if err != nil {
		// problem downloading list of available updates. bail.
		return empty_response, err
	}
	summary := summary_list[0]
	addon_name := ca.Name
	addon_version := summary.Version
	return DownloadAddon(app, ad, addon_name, addon_version, summary.DownloadURL)
}

// not implemented yet: does nothing and returns nil.
func remove_completely_overwritten_addons(addons_dir AddonsDir, addon Addon, toplevel_dirs mapset.Set[string]) error {
	slog.Error("not implemented")
	return nil
}

// writes an nfo file into each of the given `toplevel_dirs`.
// the directory matching `primary_subdir` is marked as the addon's primary one.
// `ignored` and `pinned` describe the addons being replaced: an ignored addon stays
// ignored, and a pin is dropped because the addon is no longer at the pinned version.
// nothing is written when any of the derived nfo data is invalid: a partly-written set of
// nfo files would leave the group inconsistent.
func update_nfo_files(addons_dir AddonsDir, addon Addon, toplevel_dirs mapset.Set[string], primary_subdir string, ignored bool, pinned bool) {
	to_be_written := map[PathToDir][]NFO{}
	error_list := []error{}

	for _, toplevel_dir := range toplevel_dirs.ToSlice() {
		final_addon_path := filepath.Join(addons_dir.Path, toplevel_dir)
		is_primary := toplevel_dir == primary_subdir
		new_nfo := derive_nfo(addon, is_primary)
		issues := new_nfo.Valid()
		if issues != nil {
			PrintSpecErr(issues, new_nfo)
			error_list = append(error_list, errors.New("derived nfo data is invalid"))
			continue
		}

		if ignored {
			new_nfo.Ignored = new(true)
		}

		if pinned {
			new_nfo = nfo_unpin(new_nfo)
		}

		new_nfo_list, user_msg, err := add_nfo(final_addon_path, new_nfo)
		if err != nil {
			// failed to add/update NFO data ...?
			// what to do?
			slog.Error("failed to update nfo data", "error", err)
			error_list = append(error_list, err)
			continue
		}

		if user_msg != "" {
			slog.Info(user_msg)
		}

		to_be_written[toplevel_dir] = new_nfo_list
	}

	if len(error_list) != 0 {
		slog.Error("refusing to update nfo files, errors encountered", "error-list", error_list)
		return
	}

	for toplevel_dir, new_nfo_list := range to_be_written {
		final_addon_path := filepath.Join(addons_dir.Path, toplevel_dir)
		write_nfo(final_addon_path, new_nfo_list)
	}
}

// further options to tweak installation behaviour
type InstallOpts struct {
	OverwriteIgnored bool
	UnpinPinned      bool
}

// installs `zipfile` into `addons_dir` for the given `addon`, uninstalling any previous
// version first and writing the nfo files afterwards.
// returns an error only when the .zip cannot be read: a failure to uninstall, unzip or
// write nfo data is logged and installation continues.
// an addon that was ignored or pinned before is re-ignored, and unpinned, afterwards.
// file, addon and state checks, locks and cleanup happen in `install_addon_guard`.
// clj: `addon.clj/install-addon`
func install_addon(addons_dir AddonsDir, addon Addon, zipfile string) error {
	report, err := inspect_zipfile(zipfile)
	if err != nil {
		return fmt.Errorf("failed to install addon: error inspecting .zip file: %w", err)
	}

	// read any existing nfo data.
	// note! this data is *not* preserved and is derived again from the new state of the given `Addon`.
	ignored := false
	pinned := false
	for _, toplevel_dir := range report.TopLevelDirs.ToSlice() {
		nfo_data, err := read_nfo_file(filepath.Join(addons_dir.Path, toplevel_dir))
		if err != nil {
			if errors.Is(err, ErrNFODNE) {
				// new addon dir, all good
			} else {
				// nfo data exists but it cannot be read, bad json, whatever.
				slog.Error("failed to read .nfo data", "err", err)
			}
		}
		nfo, _ := pick_nfo(nfo_data)
		pinned = pinned || nfo.PinnedVersion != ""
		ignored = ignored || nfo_ignored(nfo)
	}

	primary_subdir, err := determine_primary_subdir(report.TopLevelDirs)
	if err != nil {
		slog.Warn("failed to determine a primary subdir", "toplevel-dirs", report.TopLevelDirs.ToSlice(), "error", err)
	}

	// todo: warn the user when the .zip holds additional addons. zip bomb check.

	err = remove_addon(addon, addons_dir)
	if err != nil {
		slog.Error("failed to properly uninstall previously installed version of addon", "error", err)
	}

	err = remove_completely_overwritten_addons(addons_dir, addon, report.TopLevelDirs)
	if err != nil {
		slog.Error("failed to properly uninstall completely overwritten addons", "error", err)
	}

	extracted_files, err := unzip_file(zipfile, addons_dir.Path)
	if err != nil {
		slog.Error("failed to unzip file", "output-dir", addons_dir.Path, "zipfile", zipfile, "error", err, "extracted-files", extracted_files)
	}

	update_nfo_files(addons_dir, addon, report.TopLevelDirs, primary_subdir, ignored, pinned)

	return nil
}

// not implemented yet: removes no files and returns nil.
// a nil `num_zips_to_keep` means 'keep all'.
// clj: `addons/remove-zip-files!`
func remove_zip_files(addons_dir AddonsDir, addon_name string, num_zips_to_keep *uint8) error {
	if num_zips_to_keep == nil {
		return nil
	}

	slog.Info("pruning zip files", "addon-name", addon_name)

	// ...

	return nil
}

// returns an error when the .zip described by `report` is not a usable addon archive.
// an addon archive must have no top-level files, at least one top-level directory, and a
// .toc file directly inside every top-level directory.
// at most three offending entries are named in the error.
// clj: `zip/valid-addon-zip-file?`
func valid_addon_zip_file(report ZipReport) error {
	if report.TopLevelFiles.Cardinality() > 0 {
		tlf := report.TopLevelFiles.ToSlice()
		slices.Sort(tlf)
		max_tlf := 3
		errstr := strings.Join(core.Take(max_tlf, tlf), ", ")
		if report.TopLevelFiles.Cardinality() > max_tlf {
			// "addon zip file contains top level files: foo.ext, bar.ext, baz.ext, ..."
			errstr += ", ..."
		}
		return fmt.Errorf("addon zip file contains top level files: %s", errstr)
	}

	if report.TopLevelDirs.Cardinality() == 0 {
		return fmt.Errorf("addon zip file contains no directories")
	}

	second_level_file_parents := mapset.NewSet[string]()
	for _, f := range report.Contents {
		bits := strings.Split(f, "/") // "EveryAddon/EveryAddon.toc" => ["EveryAddon", "EveryAddon.toc"]
		if len(bits) == 2 && strings.HasSuffix(f, ".toc") {
			slfp := filepath.Dir(f)
			second_level_file_parents.Add(slfp)
		}
	}

	diff := report.TopLevelDirs.Difference(second_level_file_parents)
	if diff.Cardinality() != 0 {
		dirs := diff.ToSlice()
		slices.Sort(dirs)
		max_slfp := 3
		errstr := strings.Join(core.Take(max_slfp, dirs), ", ")
		if diff.Cardinality() > max_slfp {
			errstr += ", ..."
		}
		return fmt.Errorf("addon zip file contains top-level one or more top level directories missing a .toc file: %s", errstr)
	}

	return nil
}

// returns `true` when the archive would unpack over any ignored addon in `al`.
// includes already installed versions of the addon itself, as a further guard against
// modifying an ignored addon.
func will_overwrite_ignored(al []Addon, report ZipReport) bool {
	for _, a := range al {
		if a.IsIgnored && report.TopLevelDirs.Contains(a.DirName) {
			return true
		}
	}
	return false
}

// returns `true` when the archive would unpack over any pinned addon in `al`.
func will_overwrite_pinned(al []Addon, report ZipReport) bool {
	for _, a := range al {
		if a.IsPinned && report.TopLevelDirs.Contains(a.DirName) {
			return true
		}
	}
	return false
}

/*
func post_install(addons_dir AddonsDir, addon Addon, user_prefs Preferences) {
	remove_zip_files(addons_dir, addon.Name, user_prefs.AddonZipsToKeep)
}
*/

// installs `zipfile` into `addons_dir`, refusing when the archive is not a valid addon
// archive, or when it would overwrite an ignored or pinned addon.
// `opts` allows the ignored and pinned refusals to be overridden.
// app state is reloaded from the addons dir on success.
// zip files are pruned afterwards whether the install succeeded or not.
func install_addon_guard(app *core.App, addons_dir AddonsDir, addon Addon, zipfile string, opts InstallOpts) error {
	report, err := inspect_zipfile(zipfile)
	if err != nil {
		return fmt.Errorf("failed to install addon: error inspecting .zip file: %w", err)
	}

	err = valid_addon_zip_file(report)
	if err != nil {
		return fmt.Errorf("refusing to install: %w", err)
	}

	al, err := LoadAllInstalledAddons(addons_dir)
	if err != nil {
		return fmt.Errorf("failed to install addon: error inspecting addons directory for ignored addons: %w", err)
	}

	if will_overwrite_ignored(al, report) && !opts.OverwriteIgnored {
		return fmt.Errorf("refusing to install addon that will overwrite an ignored addon")
	}

	if will_overwrite_pinned(al, report) && !opts.UnpinPinned {
		return fmt.Errorf("refusing to install addon that will overwrite a pinned addon")
	}

	defer func() {
		// problem here: `FindSettings` requires settings to have been loaded. this requires extra setup in testing
		//post_install(addons_dir, addon, FindSettings(app).Preferences)
		remove_zip_files(addons_dir, addon.Name, new(uint8(3))) //user_prefs.AddonZipsToKeep)
	}()

	err = install_addon(addons_dir, addon, zipfile)
	if err != nil {
		return fmt.Errorf("failed to install addon: %w", err)
	}

	// update state. note: this might be causing flashing in the results
	LoadAllInstalledAddonsToState(app, addons_dir)

	return nil
}

// downloads and installs the catalogue addon `ca` into `addons_dir`.
// returns an error only when the updates or the download fail: a failure to install is
// swallowed.
// does not acquire locks, callers coordinate their own access.
// todo: an installed addon already matched to `ca` is not detected, so it is installed as
// though it were new.
// clj: `cli/install-addon`, `cli/install-many`
func install_addon_from_catalogue(app *core.App, addons_dir AddonsDir, ca CatalogueAddon) error {
	source_update_list, err := ExpandSummary(app, ca.Source, string(ca.SourceID))
	if err != nil {
		// problem downloading list of available updates. bail.
		return err
	}

	a := MakeAddonFromCatalogueAddon(addons_dir, ca, source_update_list)

	zipfile, err := download_addon_update(app, addons_dir, a)
	if err != nil {
		return err
	}

	opts := InstallOpts{}
	install_addon_guard(app, addons_dir, a, zipfile, opts)

	return nil
}

// installs each catalogue addon in `cal` into `addons_dir`, one at a time.
// a failure is logged and the remaining addons are still installed.
func install_many_addons_from_catalogue(app *core.App, addons_dir AddonsDir, cal []CatalogueAddon) {
	for _, ca := range cal {
		err := install_addon_from_catalogue(app, addons_dir, ca)
		if err != nil {
			slog.Error("failed to install addon", "error", err)
		}
	}
}

// removes the addon in `r` from the filesystem and from application state.
// refuses to remove an addon that is being ignored.
// blocks until state has been updated.
func RemoveAddon(app *core.App, r *core.Result) error {
	a := r.Item.(Addon)

	if a.IsIgnored {
		return fmt.Errorf("refusing to remove addon, addon is being ignored")
	}

	err := remove_addon(a, *a.AddonsDir)
	if err != nil {
		return fmt.Errorf("failed to remove addon: %w", err)
	}

	app.RemoveResult(r.ID).Wait()
	return nil
}

// cli/update-all
/*
func update_all_addons(app *core.App) {
	slog.Info("updating addons")

	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		slog.Warn("no addons directory selected, cannot update any addons")
		return
	}

	p := pool.NewWithResults[string]()
	for _, r := range updateable_addons(app, addons_dir) {
		r := r
		p.Go(func() string {
			a := r.Item.(Addon)
			file, err := download_addon_update(app, addons_dir, a)
			if err != nil {
				slog.Error("failed to download addon update", "error", err)
			}
			return file
		})
	}

	p.Wait()

	panic("not implemented")
}
*/

// loads the addons found in a specific directory
// use LoadAllInstalledAddons instead
/*
func load_addons_dir(ad AddonsDir) ([]Addon, error) {
	addon_list, err := LoadAllInstalledAddons(ad)
	if err != nil {
		slog.Warn("failed to load addons from selected addon dir", "addons-dir", ad, "error", err)
		return []Addon{}, errors.New("failed to load addons from selected addon dir")
	}

	slog.Info("addons dir read", "addons-dir", ad, "num-addons", len(addon_list))

	return addon_list, nil
}
*/

// loads the addons in the given addons dir `ad` into app state, each parented to the
// addons dir result.
// returns an error when the directory cannot be read, or when it is not in app state.
// not idempotent: each call adds a fresh set of results with new IDs, so calling it twice
// duplicates every addon.
func LoadAllInstalledAddonsToState(app *core.App, ad AddonsDir) error {
	slog.Info("loading addons dir")

	addon_list, err := LoadAllInstalledAddons(ad)
	if err != nil {
		return fmt.Errorf("failed to load addons dir: %w", err)
	}

	// 2025-09-07: weird failure in main_test here when moving this section above `LoadAllInstalledAddons`
	r := app.FindResultByItem(ad)
	if r == nil {
		return fmt.Errorf("failed to find addons directory in application state: %s", ad.Path)
	}

	result_list := []core.Result{}
	for _, addon := range addon_list {
		addon_r := core.MakeResult(NS_ADDON, addon, core.UniqueID())
		// note: I think the inverse of this logic exists with AddonDirs yielding Addon children in `AddonsDir.ItemChildren()` ... investigate.
		addon_r.ParentID = r.ID // an addon's parent is the addons directory it lives in.
		result_list = append(result_list, addon_r)
	}

	update_installed_addon_list(app, result_list)

	return nil
}

// ---

// downloads and loads the catalogue, matches it against the installed addons, checks
// them for updates and saves the settings.
// installed addons are not loaded here: they are loaded lazily as children of their
// addons dir.
// the user catalogue is not loaded, it is disabled during development.
// every step logs its own failures, nothing is returned.
// todo: fold into `Start` so a manual 'Refresh' isn't needed.
func Refresh(app *core.App) {
	slog.Info("refreshing")

	//load_all_installed_addons(app) // disabled, addons load as children of their addons dir

	DownloadCurrentCatalogue(app)

	//DBLoadUserCatalogue(app) // disabled during dev because state output is large

	DBLoadCatalogue(app)

	err := Reconcile(app)
	if err != nil {
		slog.Error("failed to reconcile addons", "error", err)
	}

	CheckForUpdates(app)

	SaveSettings(app)
}

// starts strongbox: reads the XDG environment, fixes the paths in app state, creates the
// directories, loads the settings and refreshes.
// returns an error when strongbox is already running in this app, when a path cannot be
// derived, or when the directories cannot be created.
// idempotent, providers can be started and stopped by the user. a provider that fails to
// start has no services available.
func Start(app *core.App) error {
	slog.Debug("starting strongbox")

	// todo: check app state for loaded provider instead of checking for key
	val := app.State.GetKeyVal("app.name")
	if val == "strongbox" {
		return errors.New("only one instance of strongbox can be running at a time")
	}

	// parse some envvars

	config_dir, err := xdg_path("XDG_CONFIG_HOME")
	if err != nil {
		return err
	}
	data_dir, err := xdg_path("XDG_DATA_HOME")
	if err != nil {
		return err
	}

	// derive some paths

	paths := set_paths(app, config_dir, data_dir)

	// set some vars

	version := "8.0.0-unreleased" // todo: pull version from ... ?
	about_str := fmt.Sprintf(`version: %s\nhttps://github.com/ogri-la/strongbox\nAGPL v3`, version)
	config := map[string]string{
		"app.name":    "strongbox",
		"app.version": version,
		"app.about":   about_str,
		//"app.data-dir":   paths["data-dir"],
		//"app.config-dir": paths["config-dir"],
	}
	for key, val := range config {
		app.State.SetKeyAnyVal(key, val)
	}

	// reset-logging!

	// detect-repl!

	err = init_dirs(app)
	if err != nil {
		return err
	}

	// prune-http-cache

	LoadSettings(app) // get/create/migrate app config
	SaveSettings(app)

	// ---

	Refresh(app)

	slog.Debug("strongbox started", "config", config, "paths", paths)

	return nil
}

// not implemented yet: performs no cleanup.
// todo: call cleanup fns, dump useful info when in debug mode, reset state.
func Stop(app *core.App) {
	slog.Debug("stopping strongbox")
}
