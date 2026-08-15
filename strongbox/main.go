package main

import (
	"bw/bw"
	"bw/core"
	"bw/ui"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	strongbox "strongbox/src"

	"sync"

	"log/slog"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/lmittmann/tint"
	"github.com/visualfc/atk/tk"
)

func stderr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// reads the command line flags and sets the default logger's level.
// exits with a status of 1 when the verbosity level is not recognised.
func handle_flags() {
	logging_level_ptr := flag.String("verbosity", "info", "level is one of 'debug', 'info', 'warn', 'error', 'fatal'")
	flag.Parse()

	logging_level, present := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}[*logging_level_ptr]
	if !present {
		stderr("unknown verbosity level")
		os.Exit(1)
	}
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: logging_level})))
}

// returns the absolute path held in the given XDG `envvar`, suffixed with 'strongbox'
// unless its base name already starts with it.
// the prefix check, rather than an equality check, accommodates 'strongbox8' during
// development.
// returns an empty string when the variable is unset.
// panics if the value cannot be made absolute.
// set the environment variables and cwd before init when testing, for isolation.
func xdg_path(envvar string) string {
	xdg_path_str := os.Getenv(envvar)
	if xdg_path_str == "" {
		return xdg_path_str
	}
	xdg_path_str, err := filepath.Abs(xdg_path_str)
	if err != nil {
		slog.Error("error parsing envvar", "envvar", envvar, "error", err)
		panic("programming error")
	}
	if !strings.HasPrefix(filepath.Base(xdg_path_str), "strongbox") {
		xdg_path_str, _ = filepath.Abs(filepath.Join(xdg_path_str, "strongbox")) // "/home/.config" => "/home/.config/strongbox"
	}
	return xdg_path_str
}

func default_config_dir() string {
	return core.HomePath("/.config/strongbox8")
}

func default_data_dir() string {
	return core.HomePath("/.local/share/strongbox8")
}

// builds the whole application: starts boardwalk, builds the GUI and its tabs, registers
// the providers and applies the user's column preferences.
// returns the GUI without waiting on it, so tests can drive it.
// panics if the strongbox provider fails to start.
func main_gui() *ui.GUIUI {
	tk.SetDebugHandle(func(script string) {
		slog.Debug("tk", "script", script)
	})
	tk.SetErrorHandle(func(err error) {
		slog.Error("tk", "error", err)
		debug.PrintStack()
	})

	app := core.Start() // start boardwalk
	// defer app.Stop() // don't do this. `main_gui` is called during testing

	// paths.
	// the data dir must point at strongbox before the gui starts, so the tk scripts are
	// installed in the right place. this duplicates what the provider does on start,
	// because provider start happens after both the app and the gui have started.
	data_dir := xdg_path("XDG_DATA_HOME")
	config_dir := xdg_path("XDG_CONFIG_HOME")

	if config_dir == "" {
		config_dir = default_config_dir()
	}
	if data_dir == "" {
		data_dir = default_data_dir()
	}

	app.State.SetKeyAnyVal("app.data-dir", data_dir)
	app.State.SetKeyAnyVal("app.config-dir", config_dir)

	// ----

	var ui_wg sync.WaitGroup

	gui := ui.MakeGUI(app, &ui_wg)
	app.AddObserver(gui)

	gui.Start().Wait() // installs tcl/tk scripts, starts boardwalk gui

	// --- init Strongbox

	gui.AddTab(strongbox.TAB_LABEL_INSTALLED, func(r core.Result) bool {
		if r.ParentID == "" {
			return r.NS == strongbox.NS_ADDONS_DIR
		}
		return true
	})
	addons_dir_tab := gui.GetCurrentTab()

	addons_dir_tab_column_list := []ui.UIColumn{
		{Title: "ns"},
		{Title: "source"},
		{Title: "selected"},
		{Title: core.ITEM_FIELD_NAME, MaxWidth: 30},
		{Title: core.ITEM_FIELD_DESC, MaxWidth: 75},
		{Title: "tags"},
		{Title: core.ITEM_FIELD_DATE_CREATED},
		{Title: core.ITEM_FIELD_DATE_UPDATED},
		{Title: "dirsize"},
		{Title: "installed-version", MaxWidth: 15},
		{Title: "available-version", MaxWidth: 15},
		{Title: "version"}, // addon version if no updates, else available-version
		{Title: "game-version"},
	}
	addons_dir_tab.SetColumnAttrs(addons_dir_tab_column_list)

	// --- search catalogue tab

	gui.AddTab("search", func(r core.Result) bool {
		return r.NS == strongbox.NS_CATALOGUE_ADDON
	})
	gui_search_tab := gui.GetTab("search")
	gui_search_tab.IgnoreMissingParents = true
	gui_search_tab.SetColumnAttrs([]ui.UIColumn{
		{Title: "source", Hidden: true},
		{Title: core.ITEM_FIELD_NAME, MaxWidth: 30},
		{Title: core.ITEM_FIELD_DESC, MaxWidth: 100},
		{Title: "tags", MaxWidth: 50},
		{Title: core.ITEM_FIELD_DATE_UPDATED, Hidden: true},
		{Title: "downloads"},
	})
	gui_search_tab.SetSearchFilter(func(input string, row map[string]string) bool {
		if input == "" {
			return true
		}
		needle := strings.ToLower(input)
		name := strings.ToLower(row[string(core.ITEM_FIELD_NAME)])
		desc := strings.ToLower(row[string(core.ITEM_FIELD_DESC)])
		return name == needle || strings.Contains(desc, needle)
	})

	gui.ApplyTablelistStyling()
	gui.Show()

	// --- init providers

	app.RegisterProvider(bw.Provider(app))
	sp := strongbox.Provider(app)
	app.RegisterProvider(sp)
	app.StartProviders()

	if !app.ProviderStarted(sp) {
		panic("failed to start strongbox")
	}

	// --- apply user column preferences

	settings := strongbox.FindSettings(app)

	column_prefs_set := mapset.NewSet[string]()
	for _, col_pref := range settings.Preferences.SelectedColumns {
		column_prefs_set.Add(col_pref)
	}
	column_prefs_set.Add("ns") // debugging
	for i, col := range addons_dir_tab_column_list {
		addons_dir_tab_column_list[i].Hidden = !column_prefs_set.Contains(col.Title)
	}
	addons_dir_tab.SetColumnAttrs(addons_dir_tab_column_list)

	gui.RebuildMenu()

	return gui
}

func main() {
	handle_flags()
	gui := main_gui()
	gui.WG.Wait()
	gui.App().Stop()
}
