package main

import (
	"bw/bw"
	"bw/core"
	"bw/ui"
	"flag"
	"fmt"
	"os"
	"strconv"
	strongbox "strongbox/src"

	"sync"
	"time"

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

	cli := ui.NewCLI(app, &ui_wg)
	cli.Start().Wait()

	return cli
}

func main_gui() *ui.GUIUI {
	app := core.Start()

	var ui_wg sync.WaitGroup

	gui := ui.NewGUI(app, &ui_wg)
	gui_event_listener := ui.UIEventListener(gui)
	app.AddListener(gui_event_listener)
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
		{Title: "name"},
		{Title: "description"},
		{Title: "tags"},
		{Title: core.ITEM_FIELD_DATE_CREATED},
		{Title: core.ITEM_FIELD_DATE_UPDATED},
		{Title: "dirsize"},
		{Title: "installed-version"},
		{Title: "available-version"},
		{Title: "version"},          // addon version
		{Title: "combined-version"}, // addon version if no updates else available-version
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
			{Title: "source", Hidden: true},
			{Title: "name"},
			{Title: "description"},
			{Title: "tags", Hidden: true},
			{Title: "updated", Hidden: true},
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

	// --- just dummy code

	foo := func() {

		foo1 := core.Result{ID: "foo1", Item: map[string]string{"path": "./foo1"}}

		bar1 := core.Result{ID: "bar1", Item: map[string]string{"path": "./bar1"}}
		baz1 := core.Result{ID: "baz1", Item: map[string]string{"path": "./baz1"}}
		bup1 := core.Result{ID: "bup1", Item: map[string]string{"path": "./bup1"}}

		foo2 := core.Result{ID: "foo2", Item: map[string]string{"path": "./foo2"}}
		bar2 := core.Result{ID: "bar2", Item: map[string]string{"path": "./bar2"}}

		bup1.ParentID = baz1.ID
		baz1.ParentID = bar1.ID
		bar1.ParentID = foo1.ID

		bar2.ParentID = foo2.ID

		//app.AddResults(foo1, bar1, baz1, bup1, foo2, bar2)

		//app.AddResults(foo1).Wait()
		//app.AddResults(foo2).Wait()

		// doesn't work, should work.
		app.AddResults(foo1, bar1, baz1, bup1, foo2, bar2).Wait()

		for i := range 100 {
			i := i
			app.UpdateResult("foo1", func(r core.Result) core.Result {
				r.Item.(map[string]string)["path"] = strconv.Itoa(i + 1)
				return r
			})

			slog.Info("---------- SL:EEEEEPING _------------")
			time.Sleep(10 * time.Millisecond)
		}
	}

	if false {
		foo()
	}

	return gui
}

func main() {
	handle_flags()
	//cli := main_cli()
	//cli.WG.Wait()

	gui := main_gui() // it *is* possible to have both cli and gui running at the same time ...
	gui.WG.Wait()
}
