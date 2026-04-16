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

// filesystem paths whose location may vary based on the current working directory, environment variables, etc.
// this map of paths is generated during `start`, checked during `init-dirs` and then fixed in application state as ... TODO
// during testing, ensure the correct environment variables and cwd are set prior to init for proper isolation.
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
	// why 'prefix'? to accommodate 'strongbox' vs 'strongbox8' during development
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

	// ---

	// we need the app.data-dir to point to strongbox before the gui starts so it installs the scripts to the right location.
	// typically this would happen during provider start, which happens _after_ app and gui start ...
	// pre-app hook? pre-gui hook? leave this duplication as a necessary hack?

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

	gui.AddTab("installed", func(r core.Result) bool {
		if r.ParentID == "" {
			return r.NS == strongbox.NS_ADDONS_DIR
		}
		return true
	})
	addons_dir_tab := gui.GetCurrentTab()

	addons_dir_tab_column_list := []core.UIColumn{
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
		{Title: "version"},
		{Title: "game-version"},
	}
	addons_dir_tab.SetColumnAttrs(addons_dir_tab_column_list)

	// --- search catalogue tab

	gui.AddTab("search", func(r core.Result) bool {
		return r.NS == strongbox.NS_CATALOGUE_ADDON
	})
	gui_search_tab := gui.GetTab("search")
	gui_search_tab.IgnoreMissingParents = true
	gui_search_tab.SetColumnAttrs([]core.UIColumn{
		{Title: "source", Hidden: true},
		{Title: core.ITEM_FIELD_NAME, MaxWidth: 30},
		{Title: core.ITEM_FIELD_DESC, MaxWidth: 100},
		{Title: "tags", MaxWidth: 50},
		{Title: core.ITEM_FIELD_DATE_UPDATED, Hidden: true},
		{Title: "downloads"},
	})

	// --- show window before providers start so data populates progressively

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
