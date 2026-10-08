package main

import (
	"bw/bw"
	"bw/core"
	"bw/ui"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	strongbox "strongbox/src"

	"sync"

	"log/slog"

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

// the widest an installed tab column may grow, by column key.
var installed_column_max_width = map[string]int{
	core.ITEM_FIELD_NAME: 30,
	core.ITEM_FIELD_DESC: 75,
	"installed-version":  15,
	"available-version":  15,
	"combined-version":   25,
}

// returns the installed tab's columns, showing those named in `selected_columns`.
func installed_columns(selected_columns []string) []ui.UIColumn {
	shown := strongbox.SelectedColumnKeys(selected_columns)
	column_list := []ui.UIColumn{}
	for _, key := range strongbox.InstalledColumnKeys() {
		column_list = append(column_list, ui.UIColumn{Title: key, MaxWidth: installed_column_max_width[key], Hidden: !shown.Contains(key)})
	}
	return column_list
}

// what a test replaces to run strongbox without a network or a person.
// a zero value runs strongbox as normal.
type gui_opts struct {
	transport    http.RoundTripper                // answers every HTTP request. nil uses the caching transport
	confirm      func(title, message string) bool // answers confirmations. nil asks the user
	report_error func(title, message string)      // receives failed services. nil tells the user
}

// builds the whole application: starts boardwalk, builds the GUI and its tabs, registers
// the providers and applies the user's column preferences.
// returns the GUI without waiting on it, so tests can drive it.
// panics if the strongbox provider fails to start.
func main_gui(opts gui_opts) *ui.GUIUI {
	tk.SetDebugHandle(func(script string) {
		slog.Debug("tk", "script", script)
	})
	tk.SetErrorHandle(func(err error) {
		slog.Error("tk", "error", err)
		debug.PrintStack()
	})

	app := core.Start() // start boardwalk
	if opts.transport != nil {
		app.HTTPClient.Transport = opts.transport
	}
	// defer app.Stop() // don't do this. `main_gui` is called during testing

	// paths.
	// the data dir must point at strongbox before the gui starts, so the tk scripts are
	// installed in the right place. this duplicates what the provider does on start,
	// because provider start happens after both the app and the gui have started.
	paths := strongbox.GeneratePathMap(os.Getenv, core.HomePath(""))
	app.SetDataDir(paths["app.data-dir"])
	app.State().SetKeyAnyVal("app.config-dir", paths["app.config-dir"])

	// ----

	var ui_wg sync.WaitGroup

	gui := ui.MakeGUI(app, &ui_wg)
	if opts.confirm != nil {
		gui.Confirm = opts.confirm
	}
	if opts.report_error != nil {
		gui.ReportError = opts.report_error
	}
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

	addons_dir_tab.SetColumnAttrs(installed_columns(strongbox.COL_LIST_DEFAULT))

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
	gui_search_tab.SetSearchFilter(strongbox.CatalogueSearchFilter)

	// --- files tab

	// filesystem results browsed via the `fs-browse` service, plus the terminal
	// rows shown when loading a directory's children was abandoned
	gui.AddTab("files", func(r core.Result) bool {
		return r.NS == bw.BW_NS_FS_DIR || r.NS == bw.BW_NS_FS_FILE || r.NS == core.NS_LOAD_FAILURE
	})
	gui.GetTab("files").SetColumnAttrs([]ui.UIColumn{
		{Title: core.ITEM_FIELD_NAME},
		{Title: "ns", Hidden: true},
	})

	// the files tab starts with the user's home directory as a browsable root
	app.AddReplaceResults(bw.MakeDirResult(core.HomePath("")))

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

	addons_dir_tab.SetColumnAttrs(installed_columns(strongbox.FindSettings(app).Preferences.SelectedColumns))

	gui.RebuildMenu()

	return gui
}

func main() {
	handle_flags()
	if err := strongbox.RefuseRoot(os.Geteuid()); err != nil {
		stderr(err.Error())
		os.Exit(1)
	}
	gui := main_gui(gui_opts{})
	gui.WG.Wait()
	gui.App().Stop()
}
