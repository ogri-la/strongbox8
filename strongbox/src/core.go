package strongbox

import (
	"bw/core"
	"cmp"
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

// filesystem paths whose location may vary based on the current working directory, environment variables, etc.
// this map of paths is generated during `start`, checked during `init-dirs` and then fixed in application state as ... TODO
// during testing, ensure the correct environment variables and cwd are set prior to init for proper isolation.
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

func generate_path_map(config_dir PathToDir, data_dir PathToDir) map[string]string {

	// XDG_DATA_HOME=/foo/bar => /foo/bar/strongbox
	// XDG_CONFIG_HOME=/baz/bup => /baz/bup/strongbox
	// - https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	// ignoring XDG_CONFIG_DIRS and XDG_DATA_DIRS for now
	if config_dir == "" {
		config_dir = default_config_dir()
	}
	if data_dir == "" {
		data_dir = default_data_dir()
	}
	log_dir := join(data_dir, "logs")

	// ensure path ends with `-file` or `-dir` or `-url`.
	return map[string]string{
		"config-dir":    config_dir,
		"data-dir":      data_dir,
		"catalogue-dir": data_dir,

		// "/home/$you/.local/share/strongbox/logs"
		"log-data-dir": log_dir,
		"log-file":     join(log_dir, "debug.log"),

		// "/home/$you/.local/share/strongbox/cache"
		"cache-dir": join(data_dir, "cache"),

		// "/home/$you/.config/strongbox/config.json"
		"cfg-file": join(config_dir, "config.json"),

		// "/home/$you/.local/share/strongbox/etag-db.json"
		"etag-db-file": join(data_dir, "etag-db.json"),

		// todo: move user catalogue to data dir?
		// "/home/$you/.config/strongbox/user-catalogue.json"
		"user-catalogue-file": join(config_dir, "user-catalogue.json"),
	}
}

func set_paths(app *core.App, config_dir PathToDir, data_dir PathToDir) map[string]string {
	path_map := generate_path_map(config_dir, data_dir)
	app.State.SetKeyVals("strongbox.paths", path_map)
	return path_map
}

func get_paths(app *core.App) map[string]string {
	return app.State.SomeKeyVals("strongbox.paths")
}

// ensure all directories in `generate-path-map` exist and are writable, creating them if necessary.
// this logic depends on paths that are not generated until the application has been started."
func init_dirs(app *core.App) error {
	data_dir := app.State.KeyVal("strongbox.paths.data-dir")

	if !core.PathExists(data_dir) && core.LastWriteableDir(data_dir) == "" {
		// data directory doesn't exist and no parent directory is writable.
		// nowhere to create data dir, nowhere to store download catalogue. non-starter.
		return fmt.Errorf("data directory doesn't exist and it cannot be created: %v", data_dir)
	}

	if core.PathExists(data_dir) && !core.PathIsWriteable(data_dir) {
		// state directory *does* exist but isn't writeable.
		// another non-starter.
		return fmt.Errorf("Data directory isn't writeable: %s", data_dir)
	}

	// ensure all '-dir' suffixed paths exist, creating them if necessary.
	for key, val := range app.State.SomeKeyAnyVals("strongbox.paths") {
		val := val.(string)
		if strings.HasSuffix(key, "-dir") && !core.DirExists(val) {
			// "creating directory(s)", "key=data-dir", "val=/path/to/data/dir"
			slog.Debug("creating directory(s)", "key", key, "val", val)
			err := core.MakeDirs(val)
			if err != nil {
				return fmt.Errorf("Failed to create '%s' directory: %s", key, val)
			}
		}
	}
	return nil
}

// fetches the `AddonsDir` matching `selected_addon_dir`.
// because there may be many entries for the given value, it returns the first it finds.
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

func selected_addon_dir(app *core.App) (AddonsDir, error) {
	return find_selected_addon_dir(app, FindSettings(app).Preferences.SelectedAddonsDir)
}

// core/update-installed-addon-list!
// updates the application state with any new addons in `addon_list`.
func update_installed_addon_list(app *core.App, addon_list []core.Result) {
	app.UpdateState(func(old_state core.State) core.State {
		idx := core.Index(addon_list, func(r core.Result) string { return r.ID }) // TODO: core.Index => bw.utils.Index or bw_utils.Index ?
		new_root := []core.Result{}
		for _, old_result := range old_state.Root.Item.([]core.Result) {
			new_result, present := idx[old_result.ID]
			if present {
				new_root = append(new_root, new_result)
			} else {
				new_root = append(new_root, old_result)
			}
		}
		old_state.Root.Item = new_root
		return old_state
	}).Wait()
}

// ----

// for each addon in `installed_addon_list`,
// looks for a match in `db` and, if found, attaches a pointer to the `addon.CatalogueAddon`.
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

// core.clj/match-all-installed-addons-with-catalogue
// compare the addons in app state with the catalogue of known addons, match the two up,
// merge the two together and update the list of installed addons.
// Skipped when no catalogue loaded or no addon directory selected.
// todo: => Reconile
func Reconcile(app *core.App) error {
	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		return errors.New("failed to reconcile addons in addons directory: no addons directory selected")
	}

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

// returns all `Addon` results attached to the given `AddonsDir`.
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

// returns all `Addon` results attached to the given `AddonsDir`.
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

// wraps the individual .ExpandSummary() methods of an addon source implementing the AddonSource interface
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

// core.clj/check-for-updates
// core.clj/check-for-updates-in-parallel
// fetches updates for all installed addons from addon hosts, in parallel.
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
		r := r
		p.Go(func() {
			a := r.Item.(Addon)
			// an ADDON can only be checked for updates if it is attached to a SOURCE.
			// this happens during catalogue matching.
			// a single SOURCE is chosen during the creation of an ADDON struct

			source_update_list, err := ExpandSummary(app, a.Source, a.SourceID)

			// if no errors, update addon result
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

// given an addon name (normalised/slugified), a version and a url,
// constructs a safe output filename and downloads the addon to the data dir.
func DownloadAddon(app *core.App, ad AddonsDir, addon_name string, addon_version string, url URL) (string, error) {
	empty_response := ""
	//data_dir := app.State.KeyVal("bw.app.data-dir") // um ... no, we're downloading it to the AddonsDir.
	data_dir := ad.Path
	output_file := downloaded_addon_fname(addon_name, addon_version)
	output_path := filepath.Join(data_dir, output_file)
	err := app.DownloadFile(url, output_path)
	if err != nil {
		return empty_response, err
	}
	return output_path, nil
}

// downloads an update for an Addon to the given `addons_dir`.
// Addons have already chosen their preferred SourceUpdate during their creation,
// so this is really simple.
// returns the path to the downloaded file.
// NOTE: does not acquire locks, execution should be coordinated.
// FUTURE: single zip repository
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

// downloads an update for a CatalogueAddon to the given `addons_dir`.
// CatalogueAddons need to be found, inspected and matched against the addons dir game track and any user preferences.
// returns the path to the downloaded file.
// NOTE: does not acquire locks, execution should be coordinated.
// FUTURE: single zip repository
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

func remove_completely_overwritten_addons(addons_dir AddonsDir, addon Addon, toplevel_dirs mapset.Set[string]) error {
	slog.Error("not implemented")
	return nil
}

// write the nfo files, return a list of all nfo files written
func update_nfo_files(addons_dir AddonsDir, addon Addon, toplevel_dirs mapset.Set[string], primary_subdir string, ignored bool, pinned bool) {
	for _, toplevel_dir := range toplevel_dirs.ToSlice() {
		final_addon_path := filepath.Join(addons_dir.Path, toplevel_dir)
		is_primary := toplevel_dir == primary_subdir
		new_nfo := derive_nfo(addon, is_primary)

		// if any of the addons this addon is replacing are being ignored,
		// the new nfo will be ignored too.
		if ignored {
			new_nfo.Ignored = Ptr(true)
		}

		// if any of the addons this addon is replacing are pinned,
		// the pin is removed. We've just modified them and they are no longer at that version.
		if pinned {
			new_nfo = nfo_unpin(new_nfo)
		}

		new_nfo_list, user_msg, err := add_nfo(final_addon_path, new_nfo)
		if err != nil {
			// failed to add/update NFO data ...?
			// what to do?
			slog.Error("failed to update nfo data", "error", err)
		}

		if user_msg != "" {
			slog.Info(user_msg)
		}

		// so ... unzipping will just write over the top, preserving any extant nfo files
		// add_nfo reads the file from the disk and adds new data, but doesn't write it
		// write_nfo then takes all of this and writes to back to disk.

		// we're replacing nfo data.
		// if nfo data already existed, it wouldn't have made it
		write_nfo(final_addon_path, new_nfo_list)
	}
}

// further options to tweak installation behaviour
type InstallOpts struct {
	OverwriteIgnored bool
	UnpinPinned      bool
}

// `addon.clj/install-addon`.
// file checks, addon checks, state checks, locks, cleanup all happen *elsewhere*.
// at this point the only thing that will stop this function from installing an addon is:
// * zipfile dne
// * zipfile corrupt and cannot be read
// * destination cannot be written to
//
// 'installs' the `zipfile` file in to the `addons_dir` for the given `addon`,
// handles suspicious looking bundles, conflicts with other addons, uninstalling previous addon version and updating nfo files.
func install_addon(addons_dir AddonsDir, addon Addon, zipfile string, opts InstallOpts) error {
	report, err := inspect_zipfile(zipfile)
	if err != nil {
		return fmt.Errorf("failed to install addon: error inspecting .zip file: %w", err)
	}

	ignored := false
	pinned := false
	for _, toplevel_dir := range report.TopLevelDirs.ToSlice() {
		nfo_data, err := read_nfo_file(filepath.Join(addons_dir.Path, toplevel_dir))
		if err != nil {
			if errors.Is(err, ERROR_NFO_DNE) {
				// new addon dir, all good
			} else {
				// nfo data exists but it cannot be read, bad json, whatever.
				// what to do? for now: fail fast.
				// previously we deleted the data if it was invalid/corrupt I think?
				//return empty_response, fmt.Errorf("failed to install addon: failed to read .nfo data: %w", err)
				fmt.Println(fmt.Errorf("failed to install addon: failed to read .nfo data: %w", err))
			}
		}
		nfo, _ := pick_nfo(nfo_data)

		pinned = pinned || nfo.PinnedVersion != ""
		ignored = ignored || nfo_ignored(nfo)
	}

	// primary-dirname (determine-primary-subdir toplevel-dirs)
	primary_subdir, err := determine_primary_subdir(report.TopLevelDirs)
	if err != nil {
		slog.Warn("failed to determine a primary subdir", "toplevel-dirs", report.TopLevelDirs.ToSlice(), "error", err)
	}

	// sus addon check
	// . check zip paths for additional addons that will be installed and warn user
	// . zip bomb check? always wanted to

	err = remove_addon(addon, addons_dir)
	if err != nil {
		slog.Error("failed to properly uninstall previously installed version of addon", "error", err)
	}

	err = remove_completely_overwritten_addons(addons_dir, addon, report.TopLevelDirs)
	if err != nil {
		slog.Error("failed to properly uninstall completely overwritten addons", "error", err)
	}

	// unzip addon in addons dir
	extracted_files, err := unzip_file(zipfile, addons_dir.Path)
	if err != nil {
		slog.Error("failed to unzip file", "output-dir", addons_dir.Path, "zipfile", zipfile, "error", err, "extracted-files", extracted_files)
	}

	// write nfo files

	update_nfo_files(addons_dir, addon, report.TopLevelDirs, primary_subdir, ignored, pinned)

	return nil
}

// addons/remove-zip-files!
func remove_zip_files(addons_dir AddonsDir, addon_name string, num_zips_to_keep *uint8) error {
	if num_zips_to_keep == nil {
		return nil
	}

	slog.Info("pruning zip files", "addon-name", addon_name)

	// ...

	return nil
}

// zip/valid-addon-zip-file?
// returns an error if there are addon-related problems with the zip file
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

// returns `true` if given archive file would unpack over *any* ignored addon.
// this includes already installed versions of itself and is another check against modifying ignored addons.
func will_overwrite_ignored(al []Addon, report ZipReport) bool {
	for _, a := range al {
		if a.IsIgnored && report.TopLevelDirs.Contains(a.DirName+"/") {
			return true
		}
	}
	return false
}

func will_overwrite_pinned(al []Addon, report ZipReport) bool {
	for _, a := range al {
		dir_name := a.DirName + "/" // dirs in zip files have trailing slash
		if a.IsPinned && report.TopLevelDirs.Contains(dir_name) {
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

// wraps the addon installation process and checks files, other addons, state, locks and cleans up the whole thing afterwards.
func install_addon_guard(app *core.App, addons_dir AddonsDir, addon Addon, zipfile string, opts InstallOpts) error {
	report, err := inspect_zipfile(zipfile)
	if err != nil {
		return fmt.Errorf("failed to install addon: error inspecting .zip file: %w", err)
	}

	err = valid_addon_zip_file(report)
	if err != nil {
		return fmt.Errorf("refusing to install: %w", err)
	}

	al, err := load_addons_dir(addons_dir)
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
		remove_zip_files(addons_dir, addon.Name, Ptr(uint8(3))) //user_prefs.AddonZipsToKeep)
	}()

	return install_addon(addons_dir, addon, zipfile, opts)

}

// cli/install-addon, cli/install-many
// downloads and installs an addon from the catalogue.
// NOTE: does not acquire locks, execution should be coordinated.
func install_addon_from_catalogue(app *core.App, addons_dir AddonsDir, ca CatalogueAddon) error {
	source_update_list, err := ExpandSummary(app, ca.Source, string(ca.SourceID))
	if err != nil {
		// problem downloading list of available updates. bail.
		return err
	}

	// we need to check if any installed addon has been matched against the catalogue addon we're trying to install.

	// to do that, we need to do the opposite of reconcilation (matching installed addons against the catalogue),
	// and match a catalogue entry against installed addons.

	// in all likelihood that isn't going to be the case.
	// almost all of the time it will be a fresh addon with, potentially, overlapping (mutual) dependencies.

	a := MakeAddonFromCatalogueAddon(addons_dir, ca, source_update_list)

	zipfile, err := download_addon_update(app, addons_dir, a)
	if err != nil {
		return err
	}

	opts := InstallOpts{}
	install_addon_guard(app, addons_dir, a, zipfile, opts)

	return nil
}

// cli/update-all
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
}

// loads the addons found in a specific directory
func load_addons_dir(selected_addons_dir AddonsDir) ([]Addon, error) {
	addon_list, err := LoadAllInstalledAddons(selected_addons_dir)
	if err != nil {
		slog.Warn("failed to load addons from selected addon dir", "selected-addon-dir", selected_addons_dir, "error", err)
		return nil, errors.New("failed to load addons from selected addon dir")
	}

	// deterministic order.
	slices.SortStableFunc(addon_list, func(a Addon, b Addon) int {
		return cmp.Compare(a.Label, b.Label)
	})

	slog.Info("addons dir read", "addon-dir", selected_addons_dir, "num-addons", len(addon_list))

	return addon_list, nil
}

// ---

// todo: can I fold this into `init` ?
// I don't like the idea of hitting a 'Refresh' any more
func Refresh(app *core.App) {
	slog.Info("refreshing")

	// this only loads installed addons for the currently selected addons dir.
	// I'm changing this so that all addon dirs will be present at the top level,
	// all addon dirs will be lazily loaded,
	// the selected addon dir will have be automatically realised,
	// and that multiple addon dirs can be 'selected' at once.
	// for now: all addon dirs are eagerly loaded
	//load_all_installed_addons(app) // disabled because the loading of addons happens as children to addon dirs

	DownloadCurrentCatalogue(app)

	//DBLoadUserCatalogue(app) // disabled during dev because state output is large

	DBLoadCatalogue(app)

	err := Reconcile(app) // previously: match-all-installed-addons-with-catalogue
	if err != nil {
		slog.Error("failed to reconcile addons", "error", err)
	}

	CheckForUpdates(app)

	SaveSettings(app)

	// scheduled-user-catalogue-refresh
}

// note: idempotent. all providers can be started and stopped by the user.
// when a provider fails to start, it's services become unavailable
func Start(app *core.App) error {
	slog.Debug("starting strongbox")

	// todo: check app state for loaded provider instead of checking for key
	val := app.State.KeyVal("bw.app.name")
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
		"name":       "strongbox",
		"version":    version,
		"about":      about_str,
		"data-dir":   paths["data-dir"],
		"config-dir": paths["config-dir"],
	}
	app.State.SetKeyVals("bw.app", config)

	// reset-logging!

	// detect-repl!

	err = init_dirs(app)
	if err != nil {
		return err
	}

	// prune-http-cache

	LoadSettings(app)

	// ---

	Refresh(app)

	slog.Debug("strongbox started", "config", config, "paths", paths)

	return nil
}

func Stop(app *core.App) {
	slog.Debug("stopping strongbox")
	// call cleanup fns
	// when debug-mode,
	//   dump-useful-info
	//   slog.info 'wrote debug log to: ...'
	// reset-state!
}
