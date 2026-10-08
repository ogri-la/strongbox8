package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// sets every path in `path_map` in app state, plus strongbox 7's paths for the one-time
// import. the data dir is set through `App.SetDataDir`, which also places the HTTP cache.
func set_paths(app *core.App, path_map map[string]string, v7 V7Paths) {
	for key, val := range path_map {
		app.State().SetKeyAnyVal(key, val)
	}
	app.State().SetKeyAnyVal("strongbox.paths.v7-cfg-file", v7.CfgFile)
	app.State().SetKeyAnyVal("strongbox.paths.v7-user-catalogue-file", v7.UserCatalogueFile)
	app.SetDataDir(path_map["app.data-dir"])
}

func get_paths(app *core.App) map[string]string {
	return app.State().SomeKeyVals("strongbox.paths")
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
	for key, val := range app.State().SomeKeyAnyVals("strongbox.paths") {
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

// ----

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
