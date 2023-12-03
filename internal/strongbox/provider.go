package strongbox

import (
	"bw/internal/core"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// separate 'strongbox' state for these services to act on?
// mmm ... I think I'd rather have a 'strongbox-config' type that can be saved and loaded.
// so, we 'load settings' from a file, creating addons dirs, preferences, etc, create a result, stick it on the heap.
// then we 'save settings' by ... finding the settings in the results and saving them?
// - no, we find all of the addon dirs, preferences (inc selected things, etc), catalogues, and then create a config.json file and save to file.
// use fixed ids to prevent loading duplicates
// for example, the short catalogue would have the ID 'strongbox/short-catalogue' or something. loading it from settings twice simply replaces the existing one.

func home_path(path string) string {
	user, err := user.Current()
	if err != nil {
		panic(fmt.Errorf("failed to find current user: %w", err))
	}
	if path == "" {
		return user.HomeDir
	}
	if path[0] != '/' {
		panic("programming error. path for user home must start with a forward slash")
	}
	return filepath.Join(user.HomeDir, path)
}

func default_config_dir() string {
	return home_path("/.config/strongbox")
}

func default_data_dir() string {
	return home_path("/.local/share/strongbox")
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

func join(a string, b string) string {
	c, _ := filepath.Abs(filepath.Join(a, b))
	return c
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

		// "/home/$you/.config/strongbox/user-catalogue.json"
		"user-catalogue-file": join(config_dir, "user-catalogue.json"),
	}
}

// ---

// reads: immutable cli args and immutable env args from app into state, then mutable config file from disk into state
// opens file, reads contents, validates it, creates any addon-dirs, catalogues, preferences,
func strongbox_settings_service_load(app *core.App, args core.FnArgs) core.FnResult {
	settings_file := args.ArgList[0].Val.(string)
	settings, err := load_settings_file(settings_file)
	if err != nil {
		return core.ErrorFnResult(err, "loading settings")
	}

	result_list := []core.Result{}

	// add the raw settings file contents to app state. we may need it later?
	config_ns := core.NS{Major: "strongbox", Minor: "settings", Type: "config"}
	result_list = append(result_list, core.NewResult(config_ns, settings))

	// add each of the catalogue locations
	catalogue_loc_ns := core.NS{Major: "strongbox", Minor: "catalogue", Type: "location"}
	for _, catalogue_loc := range settings.CatalogueLocationList {
		result_list = append(result_list, core.NewResult(catalogue_loc_ns, catalogue_loc))
	}

	// add each of the addon directories
	addon_dir_ns := core.NS{Major: "strongbox", Minor: "addon-dir", Type: "dir"}
	for _, addon_dir := range settings.AddonDirList {
		result_list = append(result_list, core.NewResult(addon_dir_ns, addon_dir))
	}

	// add each of the preferences
	preference_ns := core.NS{Major: "strongbox", Minor: "settings", Type: "preference"}
	result_list = append(result_list, core.NewResult(preference_ns, settings.Preferences))

	flatten := core.NS{}
	return core.FnResult{Result: core.NewResult(flatten, result_list)}
}

// pulls settings values from app state and writes results as json to a file
func strongbox_settings_service_save(app *core.App, args core.FnArgs) core.FnResult {
	settings_file := args.ArgList[0].Val.(string)

	fmt.Println(settings_file)

	return core.FnResult{}
}

func AsFnArgs(id string, someval interface{}) core.FnArgs {
	return core.FnArgs{ArgList: []core.Arg{{Key: id, Val: someval}}}
}

func load_settings(app *core.App) {

	// perhaps a safer way would be to find the service function and call it with the keyval?
	// it would go through parsing and validation that way.

	// perhaps an interface *like* CLI, but without prompts for picking args.
	// if an arg isn't provided or has a default, it fails.

	// service := app.FindService(NS{"strongbox", "settings", "service"})
	// service.CallFunction("load-settings", app, []string{app.KeyVal("strongbox", "paths", "cfg-file")})

	r := strongbox_settings_service_load(app, AsFnArgs("settings-file", app.KeyVal("strongbox", "paths", "cfg-file")))
	if r.Err != nil {
		slog.Error("error loading settings", "err", r.Err)
	} else {
		fmt.Println(core.QuickJSON(r.Result))
	}

	// from this data loaded from config file:
	// validate it, see `settings/load_settings_file`

	// create discrete types
	// - type:strongbox/addon-dir
	// - type:bw/preference

	// everything loaded needs to be recreated!
	// if I load all the preferences and dirs etc, I then need to be able to marshell them back to gether again and spit them back into an identical settings file

	// add the settings file to app state
	app.UpdateResultList(r.Result)
}

// ---

func settings_file_argdef() core.ArgDef {
	return core.ArgDef{
		ID:      "settings-file",
		Label:   "Settings file",
		Default: home_path("/.config/strongbox/config.json"), // todo: pull this from keyvals.strongbox.paths.cfg-file
		Parser:  core.ParseStringAsPath,                      // todo: create a settings file if one doesn't exist
		ValidatorList: []core.PredicateFn{
			core.IsFilenameValidator,
			core.FileDirIsWriteableValidator,
			core.FileIsWriteableValidator,
		},
	}
}

func provider() []core.Service {
	state_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "settings", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Load settings",
				Description: "Reads the settings file, creating one if it doesn't exist, and loads the contents into state.",
				Interface: core.FnInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				TheFn: strongbox_settings_service_load,
			},
			{
				Label:       "Save settings",
				Description: "Writes a settings file to disk.",
				Interface: core.FnInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				TheFn: strongbox_settings_service_save,
			},
			{
				Label:       "Default settings",
				Description: "Replace current settings with default settings.",
			},
			{
				Label: "Set preference",
			},
		},
	}

	catalogue_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "catalogue", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Catalogue info",
				Description: "Displays information about each available catalogue, including the emergency catalogue.",
			},
			{
				Label: "Update catalogues",
			},
			{
				Label: "Switch active catalogue",
			},
		},
	}

	dir_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "addon-dir", Type: "service"},
		FnList: []core.Fn{
			{
				Label: "New addon directory",
			},
			{
				Label: "Remove addon directory",
			},
			{
				Label:       "Browse addon directory",
				Description: "Opens the addon directory in a file browser.",
			},
		},
	}

	addon_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "addon", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Install addon",
				Description: "Download and unzip an addon from the catalogue.",
			},
			{
				Label:       "Import addon",
				Description: "Install an addon from outside of the catalogue.",
			},
			{
				Label:       "Un-install addon",
				Description: "Removes an addon from an addon directory, including all bundled addons.",
			},
			{
				Label:       "Re-install addon",
				Description: "Install the addon again, possible for the first time through Strongbox.",
			},
			{
				Label:       "Check addon",
				Description: "Check online for any updates but do not install them.",
			},
			{
				Label:       "Update addon",
				Description: "Download and install any updates for the selected addon",
			},
			{
				Label:       "Pin addon",
				Description: "Prevent updates to this addon.",
			},
			{
				Label:       "Un-pin addon",
				Description: "If an addon is pinned, this will un-pin it.",
			},
			{
				Label:       "Ignore addon",
				Description: "Do not touch this addon. Do not update it, remove it, overwrite it not pin it.",
			},
			{
				Label:       "Stop ignoring addon",
				Description: "If an addon is being ignored, this will stop ignoring it.",
			},

			// ungroup addon
			// set primary addon
			// find similar addons
			// switch source
		},
	}

	search_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "search", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Search",
				Description: "Search catalogue for an addon by name and description.",
			},
		},
	}

	// general services, like clearing cache, pruning zip files, etc

	return []core.Service{
		state_services,
		catalogue_services,
		dir_services,
		addon_services,
		search_services,
	}
}

func set_paths(app *core.App) {
	app.SetKeyVals("strongbox", "paths", generate_path_map())
}

// ensure all directories in `generate-path-map` exist and are writable, creating them if necessary.
// this logic depends on paths that are not generated until the application has been started."
func init_dirs(app *core.App) {
	data_dir := app.KeyVal("strongbox", "paths", "data-dir")

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
	for key, val := range app.KeyVals("strongbox", "paths") {
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

func Start(app *core.App) {

	set_paths(app)
	init_dirs(app)

	for _, service := range provider() {
		app.RegisterService(service)
	}

	// prune-http-cache
	// load-settings
	load_settings(app)

	// watch-stats!

}

func Stop() {

}
