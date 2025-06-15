package main

import (
	"bw/bw"
	"bw/core"
	"bw/ui"
	"flag"
	"fmt"
	"os"
	strongbox "strongbox/src"

	"sync"

	"log/slog"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/lmittmann/tint"
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

func main_cli() *ui.CLIUI {
	app := core.Start()

	var ui_wg sync.WaitGroup

	cli := ui.MakeCLI(app, &ui_wg)
	cli.Start().Wait()

	return cli
}

func main_gui() *ui.GUIUI {
	app := core.Start()

	var ui_wg sync.WaitGroup

	gui := ui.MakeGUI(app, &ui_wg)
	gui_event_listener := ui.UIEventListener(gui)
	app.State.AddListener(gui_event_listener)
	gui.Start().Wait()

	// --- init Strongbox

	gui.AddTab("addons-dir", func(r core.Result) bool {
		// any result is allowed in the 'addons dir' results tab,
		// but top-level results _must_ be AddonDir results.
		if r.ParentID == "" {
			return r.NS == strongbox.NS_ADDONS_DIR
		}
		return true
	}).Wait()
	addons_dir_tab := gui.GetTab("addons-dir").(*ui.GUITab)

	// --- columns

	addons_dir_tab_column_list := []ui.Column{
		// --- debugging

		{Title: "ns"},

		// ---

		{Title: "source"},
		//{Title: "browse"}, // disabled until implemented
		{Title: core.ITEM_FIELD_NAME, MaxWidth: 30},
		{Title: core.ITEM_FIELD_DESC, MaxWidth: 75},
		{Title: "tags"},
		{Title: core.ITEM_FIELD_DATE_CREATED},
		{Title: core.ITEM_FIELD_DATE_UPDATED},
		{Title: "dirsize"},
		{Title: "installed-version", MaxWidth: 15},
		{Title: "available-version", MaxWidth: 15},
		{Title: "version"},          // addon version
		{Title: "combined-version"}, // addon version if no updates, else available-version
		{Title: "game-version"},
		//{Title: "UberButton", HiddenTitle: true}, // disabled until implemented
	}
	addons_dir_tab.SetColumnAttrs(addons_dir_tab_column_list)

	// --- search catalogue tab

	if false {
		catalogue_addons := func(r core.Result) bool {
			return r.NS == strongbox.NS_CATALOGUE_ADDON
		}

		gui.AddTab("search", catalogue_addons).Wait()
		gui_search_tab := gui.GetTab("search").(*ui.GUITab)
		gui_search_tab.IgnoreMissingParents = true
		gui_search_tab.SetColumnAttrs([]ui.Column{
			{Title: "source", Hidden: true}, // TODO: erm, Hidden isn't doing anything
			{Title: core.ITEM_FIELD_NAME, MaxWidth: 30},
			{Title: core.ITEM_FIELD_DESC, MaxWidth: 100},
			{Title: "tags", Hidden: true, MaxWidth: 50},
			{Title: core.ITEM_FIELD_DATE_UPDATED, Hidden: true},
			{Title: "size", Hidden: true},
			{Title: "downloads"},
		})
		gui.SetActiveTab("search").Wait()
	}

	// --- init providers

	app.RegisterProvider(bw.Provider(app))
	app.RegisterProvider(strongbox.Provider(app))

	app.StartProviders()      // todo: use a waitgroup here for providers doing async
	defer app.StopProviders() // clean up

	// everything below this comment is a hack and needs a better home

	// --- update ui with user prefs

	// gui has been loaded
	// providers have been started
	// data is present (right? do we need a wait group anywhere?)
	settings := strongbox.FindSettings(app)

	// --- take user column preferences and update gui

	// select just those in the settings and hide any columns that are not
	column_prefs_set := mapset.NewSet[string]()
	for _, col_pref := range settings.Preferences.SelectedColumns {
		column_prefs_set.Add(col_pref)
	}
	column_prefs_set.Add("ns") // debugging
	for i, col := range addons_dir_tab_column_list {
		addons_dir_tab_column_list[i].Hidden = !column_prefs_set.Contains(col.Title)
	}
	addons_dir_tab.SetColumnAttrs(addons_dir_tab_column_list)

	// --- configure strongbox

	// now that gui and providers are init'ed,
	// add provider menu to gui
	gui.RebuildMenu()

	return gui
}

func main() {
	handle_flags()
	//cli := main_cli()
	//cli.WG.Wait()

	gui := main_gui() // it *is* possible to have both cli and gui running at the same time ...
	gui.WG.Wait()
}
