package strongbox

import (
	"bw/core"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/sourcegraph/conc/pool"
)

// provider.go pulls together the logic from the rest of the strongbox logic and presents an
// interface to the rest of the app.
// it shouldn't do much more than describe services, call logic and stick results into state.

// ---

// separate 'strongbox' state for these services to act on?
// mmm ... I think I'd rather have a 'strongbox-config' type that can be saved and loaded.
// so, we 'load settings' from a file, creating addons dirs, preferences, etc, create a result, stick it on the heap.
// then we 'save settings' by ... finding the settings in the results and saving them?
// - no, we find all of the addon dirs, preferences (inc selected things, etc), catalogues, and then create a config.json file and save to file.
// use fixed ids to prevent loading duplicates
// for example, the short catalogue would have the ID 'strongbox/short-catalogue' or something. loading it from settings twice simply replaces the existing one.

func default_config_dir() string {
	return core.HomePath("/.config/strongbox")
}

func default_data_dir() string {
	return core.HomePath("/.local/share/strongbox")
}

func join(a string, b string) string {
	c, _ := filepath.Abs(filepath.Join(a, b))
	return c
}

// filesystem paths whose location may vary based on the current working directory, environment variables, etc.
// this map of paths is generated during `start`, checked during `init-dirs` and then fixed in application state as ... TODO
// during testing, ensure the correct environment variables and cwd are set prior to init for proper isolation.
func xdg_path(envvar string, default_val string) string {
	xdg_path_val := os.Getenv(envvar)
	if xdg_path_val == "" {
		xdg_path_val = default_val
	}
	xdg_path_val, err := filepath.Abs(xdg_path_val)
	if err != nil {
		panic(fmt.Errorf("failed to expand XDG_ path: %w", err))
	}
	if !strings.HasSuffix(xdg_path_val, "/strongbox") {
		xdg_path_val = join(xdg_path_val, "strongbox")
	}
	return xdg_path_val
}

func generate_path_map() map[string]string {

	// XDG_DATA_HOME=/foo/bar => /foo/bar/strongbox
	// XDG_CONFIG_HOME=/baz/bup => /baz/bup/strongbox
	// - https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	// ignoring XDG_CONFIG_DIRS and XDG_DATA_DIRS for now

	config_dir := xdg_path("XDG_CONFIG_HOME", default_config_dir())
	data_dir := xdg_path("XDG_DATA_HOME", default_data_dir())
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

// panics with `msg` when `cond` is not true.
func ready_check(cond bool, msg string) {
	if !cond {
		panic("failed ready check: " + msg)
	}
}

// reads: immutable cli args and immutable env args from app into state, then mutable config file from disk into state
// opens file, reads contents, validates it, creates any addon-dirs, catalogues, preferences,
func load_settings(app *core.App) {
	settings_file := app.KeyAnyVal("strongbox.paths.cfg-file")
	ready_check(settings_file != nil, "path 'strongbox.paths.cfg-file' not found in app state")
	settings, err := load_settings_file(settings_file.(string))
	if err != nil {
		slog.Error("failed to loading settings", "err", err)
		return
	}

	// add each of the catalogue locations.
	// these are needed when loading the selected catalogue.
	result_list := []core.Result{}
	for _, catalogue_loc := range settings.CatalogueLocationList {
		result_list = append(result_list, core.NewResult(NS_CATALOGUE_LOC, catalogue_loc, core.UniqueID()))
	}
	app.SetResults(result_list...).Wait()

	// add each of the preferences
	result := core.NewResult(NS_PREFS, settings.Preferences, ID_PREFERENCES)
	app.SetResults(result).Wait()

	// add each of the addon directories
	result_list = []core.Result{}
	for _, addon_dir := range settings.AddonDirList {
		result_list = append(result_list, core.NewResult(NS_ADDON_DIR, addon_dir, addon_dir.Path)) //core.UniqueID()))
	}
	app.SetResults(result_list...).Wait()
}

func set_paths(app *core.App) {
	app.SetKeyVals("strongbox.paths", generate_path_map()).Wait()
}

// ensure all directories in `generate-path-map` exist and are writable, creating them if necessary.
// this logic depends on paths that are not generated until the application has been started."
func init_dirs(app *core.App) {
	data_dir := app.KeyVal("strongbox.paths.data-dir")

	if !core.PathExists(data_dir) && core.LastWriteableDir(data_dir) == "" {
		// data directory doesn't exist and no parent directory is writable.
		// nowhere to create data dir, nowhere to store download catalogue. non-starter.
		panic(fmt.Sprintf("Data directory doesn't exist and it cannot be created: %s", data_dir))
	}

	if core.PathExists(data_dir) && !core.PathIsWriteable(data_dir) {
		// state directory *does* exist but isn't writeable.
		// another non-starter.
		panic(fmt.Sprintf("Data directory isn't writeable: %s", data_dir))
	}

	// ensure all '-dir' suffixed paths exist, creating them if necessary.
	for key, val := range app.SomeKeyVals("strongbox.paths") {
		val := val.(string)
		if strings.HasSuffix(key, "-dir") && !core.DirExists(val) {
			// "creating directory(s)", "key=data-dir", "val=/path/to/data/dir"
			slog.Debug("creating directory(s)", "key", key, "val", val)
			err := core.MakeDirs(val)
			if err != nil {
				panic(fmt.Sprintf("Failed to create '%s' directory: %s", key, val))
			}
		}
	}
}

// fetch the preferences stored in state
func find_preferences(app *core.App) (Preferences, error) {
	empty_prefs := Preferences{}
	result_ptr := app.GetResult(ID_PREFERENCES)
	if result_ptr == nil {
		return empty_prefs, errors.New("strongbox preferences not found")
	}
	prefs, is_prefs := result_ptr.Item.(Preferences)
	if !is_prefs {
		slog.Error(fmt.Sprintf("something other than strongbox preferences stored at '%s': %s", ID_PREFERENCES, reflect.TypeOf(result_ptr.Item)))
		panic("programming error")
	}
	return prefs, nil
}

// fetches the `AddonsDir` matching `selected_addon_dir_str_ptr`.
// because there may be many entries for the given value, it returns the first it finds.
func find_selected_addon_dir(app *core.App, selected_addon_dir_str_ptr *string) (AddonsDir, error) {
	var selected_addon_dir_ptr *AddonsDir
	results_list := app.FilterResultList(func(result core.Result) bool {
		addon_dir, is_addon_dir := result.Item.(AddonsDir)
		if is_addon_dir && selected_addon_dir_str_ptr != nil && addon_dir.Path == *selected_addon_dir_str_ptr {
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
	var selected_addon_dir *string

	prefs, err := find_preferences(app)
	if err != nil {
		slog.Error("error looking for selected addon dir", "error", err)
	} else {
		selected_addon_dir = prefs.SelectedAddonDir
	}

	// fetch the selected addon dir
	return find_selected_addon_dir(app, selected_addon_dir)

}

// core/update-installed-addon-list!
// the list of installed addons with `installed-addon-list`"
func update_installed_addon_list(app *core.App, addon_list []core.Result) {
	app.UpdateState(func(old_state core.State) core.State {
		idx := core.Index(addon_list, func(r core.Result) string { return r.ID })
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

/*
// core.clj/load-all-installed-addons
// "offloads the hard work to `addon/load-all-installed-addons` then updates application state"
func load_all_installed_addons(app *core.App) {

	// fetch the settings
	prefs, err := find_preferences(app)
	if err != nil {
		slog.Error("error looking for selected addon dir", "error", err)
		return // load nothing.
	}

	// fetch the selected addon dir
	selected_addon_dir, err := find_selected_addon_dir(app, prefs.SelectedAddonDir)
	if err != nil {
		slog.Error("error selecting an addon dir", "error", err)

		// if no addon directory selected, ensure list of installed addons is empty
		// todo: do we want to do this in v8?
		update_installed_addon_list(app, []Addon{})
		return
	}

	// load all of the addons found in the selected addon dir

	addon_list, err := LoadAllInstalledAddons(selected_addon_dir)
	if err != nil {
		slog.Warn("failed to load addons from selected addon dir", "selected-addon-dir", selected_addon_dir, "error", err)

		// if no addon directory selected, ensure list of installed addons is empty
		// todo: do we want to do this in v8?
		return
	}

	// switch game tracks of loaded addons. separate step in v8 to avoid toc/nfo from knowing about *selected* game tracks
	addon_list = SetInstalledAddonGameTrack(selected_addon_dir, addon_list)

	// update installed addon list!
	slog.Info("loading installed addons", "num-addons", len(addon_list), "addon-dir", selected_addon_dir.Path)
	update_installed_addon_list(app, addon_list)
}
*/

// core.clj/db-catalogue-loaded?
// returns `true` if the database has a catalogue loaded.
// A database may be `nil` if it simply hasn't been loaded yet or we attempted to load it and it failed to load.
// A database may fail to load if it simply isn't there, can't be downloaded or, once downloaded, the data is invalid.
// An empty database (`Catalogue{}`) is distinct from an unloaded database (nil pointer), see `db_catalogue_empty`.
func db_catalogue_loaded(app *core.App) bool {
	return app.HasResult(ID_CATALOGUE)
}

// new in v8
// returns `true` if a database has been loaded but the database is empty.
// A database may be empty *only* if the `addon-summary-list` key of a catalogue is empty.
// An empty database (`Catalogue{}`) is distinct from an unloaded database (nil pointer), see `db_catalogue_loaded`.
func db_catalogue_empty(app *core.App) bool {
	res := app.GetResult(ID_CATALOGUE)
	if res == nil {
		return true
	}
	cat, _ := res.Item.(Catalogue)
	return len(cat.AddonSummaryList) > 0
}

func _db_load_catalogue(app *core.App) (Catalogue, error) {

	var empty_catalogue Catalogue

	if db_catalogue_loaded(app) {
		return empty_catalogue, errors.New("catalogue already loaded")
	}

	cat_loc, err := current_catalogue_location(app)
	if err != nil {
		return empty_catalogue, errors.New("no catalogue selected or selectable")
	}

	slog.Info("loading catalogue", "name", cat_loc.Label)
	catalogue_path := catalogue_local_path(app.KeyVal("strongbox.paths.catalogue-dir"), cat_loc.Name)

	cat, err := ReadCatalogue(cat_loc, catalogue_path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to read catalogue: %w", err)
	}

	return cat, nil
}

// core.clj/db-load-catalogue
// core.clj/load-current-catalogue
// loads a catalogue from disk, assuming it has already been downloaded.
func db_load_catalogue(app *core.App) {
	catalogue, err := _db_load_catalogue(app)
	if err != nil {
		slog.Warn("failed to load catalogue", "error", err)
		return
	}
	wg := app.SetResults(core.NewResult(NS_CATALOGUE, catalogue, ID_CATALOGUE))
	wg.Wait()

	r := app.GetResult(ID_CATALOGUE)
	if r == nil {
		panic("programming error, catalogue should have loaded")
	}
}

// core.clj/get-user-catalogue
// returns the contents of the user catalogue as a `Catalogue`, removing any disable hosts.
// returns an error when the catalogue is not found,
// or the catalogue cannot be read,
// or the catalogue data is bad json.
func get_user_catalogue(app *core.App) (Catalogue, error) {

	empty_catalogue := Catalogue{}

	path := app.KeyVal("strongbox.paths.user-catalogue-file")
	if !core.FileExists(path) {
		return empty_catalogue, errors.New("user-catalogue not found")
	}

	data, err := core.SlurpBytes(path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to reading user-catalogue-file: %w", err)
	}

	var cat Catalogue
	err = json.Unmarshal(data, &cat)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to unmarshal user-catalogue-file: %w", err)
	}

	new_addon_list := []CatalogueAddon{}
	for _, addon := range cat.AddonSummaryList {
		if HostDisabled(addon.Source) {
			continue
		}
		new_addon_list = append(new_addon_list, addon)
	}

	// todo: fix Catalogue.Total
	cat.AddonSummaryList = new_addon_list
	return cat, nil
}

// core.clj/db-load-user-catalogue
// loads the user catalogue into state, but only if it hasn't already been loaded.
func db_load_user_catalogue(app *core.App) {
	if app.HasResult(ID_USER_CATALOGUE) {
		return
	}
	user_cat, err := get_user_catalogue(app)
	if err != nil {
		slog.Warn("error loading user catalogue", "error", err)
	}

	// see: core.clj/set-user-catalogue!
	// todo: create an idx
	app.SetResults(core.Result{ID: ID_USER_CATALOGUE, NS: NS_CATALOGUE_USER, Item: user_cat})
}

// ----

// for each addon in `installed_addon_list`,
// looks for a match in `db` and, if found, attaches a pointer to the `addon.CatalogueAddon`.
func _reconcile(db []CatalogueAddon, installed_addon_list []core.Result) []core.Result {

	matched := []core.Result{}
	unmatched := []core.Result{}

	// --- [[:source :source-id] [:source :source-id]] ;; source+source-id, perfect case

	catalogue_addon_source_and_source_id_keyfn := func(catalogue_addon CatalogueAddon) string {
		// "github--AdiBags"
		return fmt.Sprintf("%s--%s", catalogue_addon.Source, catalogue_addon.SourceID)
	}

	addon_source_and_source_id_keyfn := func(addon Addon) string {
		// "github--AdiBags"
		return fmt.Sprintf("%s--%s", addon.Attr("source"), addon.Attr("source-id"))
	}

	// --- [:source :name] ;; source+name, we have a source but no source-id (nfo v1 files)

	addon_source_keyfn := func(a Addon) string {
		return a.Attr("source")
	}

	catalogue_addon_name_keyfn := func(ca CatalogueAddon) string {
		return ca.Name
	}

	// --- [:name :name]

	addon_name_keyfn := func(a Addon) string {
		return a.Attr("name")
	}

	// --- [:label :label]

	addon_label_keyfn := func(a Addon) string {
		return a.Attr("label")
	}

	catalogue_addon_label_keyfn := func(ca CatalogueAddon) string {
		return ca.Label
	}

	// --- [:dirname :label] ;; dirname == label, eg ./AdiBags == AdiBags

	addon_dirname_keyfn := func(a Addon) string {
		return a.Attr("dirname")
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
				addon.CatalogueAddon = &catalogue_addon
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
func reconcile(app *core.App) error {
	db_result := app.GetResult(ID_CATALOGUE)
	if db_result == nil {
		return errors.New("no catalogue to match installed addons against")
	}

	db := db_result.Item.(Catalogue).AddonSummaryList

	user_db_result := app.GetResult(ID_USER_CATALOGUE)
	if user_db_result != nil {
		user_db := user_db_result.Item.(Catalogue).AddonSummaryList
		db = append(db, user_db...)
	}

	installed_addon_list := app.FilterResultListByNS(NS_ADDON)

	reconciled_addon_list := _reconcile(db, installed_addon_list)

	update_installed_addon_list(app, reconciled_addon_list)

	return nil
}

func catalogue_loc_map(app *core.App) map[string]CatalogueLocation {
	idx := map[string]CatalogueLocation{}
	for _, result := range app.GetResultList() {
		if result.NS == NS_CATALOGUE_LOC {
			idx[result.Item.(CatalogueLocation).Name] = result.Item.(CatalogueLocation)
		}
	}
	return idx
}

// returns the CatalogueLocation of the currently selected catalogue.
// returns an error if the preferences are not loaded yet.
// returns an error if the selected catalogue is not present in the list of loaded catalogues.
func find_selected_catalogue(app *core.App) (CatalogueLocation, error) {
	empty_loc := CatalogueLocation{}
	catalogue_loc_idx := catalogue_loc_map(app) // {"full": CatalogueLocation{...}, ...}

	prefs, err := find_preferences(app)
	if err != nil {
		slog.Error("error loading preferences", "error", err)
		return empty_loc, err
	}

	selected_catalogue_name := prefs.SelectedCatalogue
	selected_catalogue, present := catalogue_loc_idx[selected_catalogue_name]
	if !present {
		slog.Error("selected catalogue not available in list of known catalogues", "selected-catalogue", selected_catalogue_name, "known-catalogue-list", core.MapKeys[string, CatalogueLocation](catalogue_loc_idx))
		return empty_loc, errors.New("selected catalogue not available in list of known catalogues")
	}

	return selected_catalogue, nil
}

// core.clj/default-catalogue
// the 'default' catalogue is the first catalogue in the list of available catalogues.
// using the original set of catalogues that come with strongbox, this is the 'short' catalogue,
// however the user can specify their own catalogues so this isn't guaranteed.
// returns an error if no catalogues available.
func default_catalogue(app *core.App) (CatalogueLocation, error) {
	empty_cat_loc := CatalogueLocation{}
	cat_loc_list := app.FilterResultListByNS(NS_CATALOGUE_LOC)
	if len(cat_loc_list) < 1 {
		return empty_cat_loc, errors.New("cannot select a default catalogue, no catalogues available")
	}
	return cat_loc_list[0].Item.(CatalogueLocation), nil
}

// core.clj/get-catalogue-location, second arity
// returns the `CatalogueLocation` for the given `cat_loc_name`.
// returns and error if catalogue location not found or no catalogue location selected.
func get_catalogue_location(app *core.App, cat_loc_name string) (CatalogueLocation, error) {
	empty_cat_loc := CatalogueLocation{}
	idx := catalogue_loc_map(app)
	cat_loc, is_present := idx[cat_loc_name]
	if !is_present {
		return empty_cat_loc, fmt.Errorf("catalogue '%s' not present in index", cat_loc_name)
	}
	return cat_loc, nil
}

// core.clj/current-catalogue
// returns the currently selected catalogue location or the first catalogue location it can find.
// returns an error if no catalogue selected or none available to choose from.
func current_catalogue_location(app *core.App) (CatalogueLocation, error) {
	cat_loc, err := find_selected_catalogue(app)
	if err != nil {
		cat_loc, err = default_catalogue(app)
		if err != nil {
			// no catalogue selected, no default catalogue available, cannot contine
			return CatalogueLocation{}, err
		}
	}
	return cat_loc, nil
}

// todo: needs to be a task that can be cancelled and cleaned up
// core.clj/download-catalogue
// downloads catalogue to expected location, nothing more
func download_catalogue(catalogue_loc CatalogueLocation, data_dir PathToDir) error {
	remote_catalogue := catalogue_loc.Source
	local_catalogue := catalogue_local_path(data_dir, catalogue_loc.Name)
	if core.FileExists(local_catalogue) {
		// todo: freshness check
		slog.Debug("catalogue exists, not downloading")
		return nil
	}
	err := core.DownloadFile(remote_catalogue, local_catalogue)
	if err != nil {
		slog.Error("failed to download catalogue", "remote-catalogue", remote_catalogue, "local-catalogue", local_catalogue, "error", err)
		return err
	}
	return nil
}

// core.clj/download-current-catalogue
// "downloads the currently selected (or default) catalogue."
func download_current_catalogue(app *core.App) {

	// get catalogue location from currently selected catalogue
	//catalogue_loc, err := find_selected_catalogue(app)
	catalogue_loc, err := current_catalogue_location(app)
	if err != nil {
		slog.Warn("failed to find a downloadable catalogue", "error", err)
		return
	}

	catalogue_dir := app.KeyVal("strongbox.paths.catalogue-dir")
	if catalogue_dir == "" {
		slog.Warn("'catalogue-dir' location not found, cannot download catalogue")
		return
	}
	_ = download_catalogue(catalogue_loc, catalogue_dir)

}

/*

(defn-spec check-for-updates-in-parallel nil?
  "fetches updates for all installed addons from addon hosts, in parallel."
  []
  (when (selected-addon-dir)
    (let [installed-addon-list (get-state :installed-addon-list)]
      (info "checking for updates")
      (let [queue-atm (get-state :job-queue)
            update-jobs (fn [installed-addon]
                          (joblib/create-addon-job! queue-atm, installed-addon, check-for-update-affective))
            _ (run! update-jobs installed-addon-list)

            expanded-addon-list (joblib/run-jobs! queue-atm num-concurrent-downloads)

            num-matched (->> expanded-addon-list (filterv :matched?) count)
            num-updates (->> expanded-addon-list (filterv :update?) count)]

        (update-installed-addon-list! expanded-addon-list)
        (info (format "%s addons checked, %s updates available" num-matched num-updates))))))

*/

// core.clj/check-for-updates
// core.clj/check-for-updates-in-parallel
// fetches updates for all installed addons from addon hosts, in parallel.
func check_for_updates(app *core.App) {

	return

	_, err := selected_addon_dir(app)
	if err != nil {
		slog.Debug("no addons directory selected, not checking for updates")
		return
	}

	installed_addon_list := core.ItemList[Addon](app.FilterResultListByNS(NS_ADDON)...)
	slog.Info("checking for updates")

	p := pool.NewWithResults[Addon]()
	for _, a := range installed_addon_list {
		a := a
		p.Go(func() Addon {
			return a
		})
	}

	p.Wait()

}

// takes the results of reading the settings and adds them to the app's state
func strongbox_settings_service_load(settings_file string) ([]core.Result, error) {
	settings, err := load_settings_file(settings_file)
	if err != nil {
		return nil, err
	}

	result_list := []core.Result{}

	// add the raw settings file contents to app state. we may need it later?
	//config_ns := core.NS{Major: "strongbox", Minor: "settings", Type: "config"}
	//result_list = append(result_list, core.NewResult(config_ns, settings))

	// add each of the catalogue locations
	for _, catalogue_loc := range settings.CatalogueLocationList {
		result_list = append(result_list, core.NewResult(NS_CATALOGUE_LOC, catalogue_loc, core.UniqueID()))
	}

	// add each of the addon directories
	for _, addon_dir := range settings.AddonDirList {
		result_list = append(result_list, core.NewResult(NS_ADDON_DIR, addon_dir, addon_dir.Path)) //core.UniqueID()))
	}

	// add each of the preferences
	result_list = append(result_list, core.NewResult(NS_PREFS, settings.Preferences, ID_PREFERENCES))

	return result_list, nil
}

// loads the addons found in a specific directory
func load_addons_dir(addons_dir string) ([]core.Result, error) {

	// fetch the selected addon dir
	// todo: this should all be pushed into service validation
	// disabled: ordering issue with settings not in state when realise_children loads addon dirs
	/*
		selected_addon_dir, err := find_selected_addon_dir(app, &addon_dir)
		if err != nil {
			slog.Error("error selecting an addon dir", "error", err)
			return core.FnResult{
				Err: fmt.Errorf("error selecting an addon dir: %w", err),
			}
		}
	*/

	// todo: temporary to fix ordering issue
	selected_addon_dir := AddonsDir{Path: addons_dir, Strict: false, GameTrackID: GAMETRACK_RETAIL}

	// load all of the addons found in the selected addon dir
	addon_list, err := LoadAllInstalledAddons(selected_addon_dir)
	if err != nil {
		slog.Warn("failed to load addons from selected addon dir", "selected-addon-dir", selected_addon_dir, "error", err)
		return nil, errors.New("failed to load addons from selected addon dir")
	}

	// switch game tracks of loaded addons. separate step in v8 to avoid toc/nfo from knowing about *selected* game tracks
	addon_list = SetInstalledAddonGameTrack(selected_addon_dir, addon_list)

	// update installed addon list!
	slog.Info("loading installed addons", "num-addons", len(addon_list), "addon-dir", selected_addon_dir.Path)
	//update_installed_addon_list(app, addon_list)

	result_list := []core.Result{}
	for _, addon := range addon_list {
		result_list = append(result_list, core.NewResult(NS_ADDON, addon, addon.Attr("id")))
	}

	return result_list, nil
}

// ---

// todo: can I fold this into `init` ?
// I don't like the idea of hitting a 'refresh' any more
func refresh(app *core.App) {

	// this only loads installed addons for the currently selected addons dir.
	// I'm changing this so that all addon dirs will be present at the top level,
	// all addon dirs will be lazily loaded,
	// the selected addon dir will have be automatically realised,
	// and that multiple addon dirs can be 'selected' at once.
	// for now: all addon dirs are eagerly loaded
	//load_all_installed_addons(app) // disabled because the loading of addons happens as children to addon dirs
	download_current_catalogue(app)
	//db_load_user_catalogue(app) // disabled because output is large

	db_load_catalogue(app) // this should .Wait()

	err := reconcile(app) // match-all-installed-addons-with-catalogue
	if err != nil {
		slog.Error("failed to reconcile addons", "error", err)
	}

	check_for_updates(app)
	// save-settings
	// scheduled-user-catalogue-refresh

}

func Start(app *core.App) core.FnResult {
	slog.Debug("starting strongbox")

	version := "8.0.0-unreleased" // todo: pull version from ... ?
	about_str := fmt.Sprintf(`version: %s\nhttps://github.com/ogri-la/strongbox\nAGPL v3`, version)
	app.SetKeyVals("bw.app", map[string]string{
		"name":    "strongbox",
		"version": version,
		"about":   about_str,
	})

	// reset-logging!

	set_paths(app)
	// detect-repl!
	init_dirs(app)
	// prune-http-cache
	load_settings(app)

	// ---

	refresh(app)

	return core.FnResult{}
}

func Stop(app *core.App) core.FnResult {
	slog.Debug("stopping strongbox")
	// call cleanup fns
	// when debug-mode,
	//   dump-useful-info
	//   slog.info 'wrote debug log to: ...'
	// reset-state!

	return core.FnResult{}

}

// ---

type StrongboxProvider struct{}

func (sp *StrongboxProvider) ServiceList() []core.Service {
	return provider()
}

var _ core.Provider = (*StrongboxProvider)(nil)

// ---

func Provider(app *core.App) *StrongboxProvider {
	return &StrongboxProvider{}
}
