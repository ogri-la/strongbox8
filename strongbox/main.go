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

func main() {
	handle_flags()

	// --- init app

	app := core.Start()

	// --- init UI

	var ui_wg sync.WaitGroup

	do_cli := false
	if do_cli {
		cli := ui.NewCLI(app, &ui_wg)
		cli.Start().Wait() // this seems to work well! cli open in terminal, gui open in new window
	}

	gui := ui.NewGUI(app, &ui_wg)
	gui_event_listener := ui.UIEventListener(gui)
	app.AddListener(gui_event_listener)
	gui.Start().Wait()

	// --- init Strongbox

	addon_dirs_tab_results := func(r core.Result) bool {
		if r.ParentID == "" {
			return r.NS == strongbox.NS_ADDON_DIR
		}
		return true
	}
	gui.AddTab("addons-dir", addon_dirs_tab_results).Wait()
	tab := gui.GetTab("addons-dir")

	// columns available to be displayed.
	// columns not selected in user preferences are hidden.
	addon_dirs_column_list := []ui.Column{
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
	tab.SetColumnAttrs(addon_dirs_column_list)

	// --- search catalogue tab

	do_catalogue := false
	if do_catalogue {
		catalogue_addons := func(r core.Result) bool {
			return r.NS == strongbox.NS_CATALOGUE_ADDON
		}

		gui.AddTab("search", catalogue_addons).Wait()
		tab = gui.GetTab("search")
		guitab := tab.(*ui.GUITab)
		guitab.IgnoreMissingParents = true
		tab.SetColumnAttrs([]ui.Column{
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

	//
	// --- update ui with user prefs
	//

	// gui has been loaded
	// providers have been started
	// data is present (right? do we need a wait group anywhere?)
	prefs_result := app.FilterResultListByNSToResult(strongbox.NS_PREFS)
	if core.EmptyResult(prefs_result) {
		// strongbox preferences should have been found or created,
		// and loaded,
		// before now.
		panic("logic error, no strongbox preferences found")
	}
	prefs := prefs_result.Item.(strongbox.Preferences)

	//
	// --- take user column preferences and update gui
	//

	// by default all columns are present and
	// the user selects a set that are visible.
	column_prefs_set := mapset.NewSet[string]()
	for _, col_pref := range prefs.SelectedColumns {
		column_prefs_set.Add(col_pref)
	}
	column_prefs_set.Add("ns") // debugging

	slog.Debug("col prefs", "prefs", prefs, "prefs-set", column_prefs_set)

	updated_addon_dirs_column_list := []ui.Column{
		// debugging. bug here. cols must also be present above
		//{Title: "id"},
		//{Title: "ns"},
	}
	for _, col := range addon_dirs_column_list {
		if !column_prefs_set.Contains(col.Title) {
			// column is missing from user's preferences.
			// hide it.
			col.Hidden = true
			slog.Warn("hiding column", "column", col.Title)
		}
		updated_addon_dirs_column_list = append(updated_addon_dirs_column_list, col)
	}

	tab.SetColumnAttrs(updated_addon_dirs_column_list)

	//
	// --- take user's selected addon dir and update gui
	//

	selected_addons_dir_ptr := prefs.SelectedAddonDir
	if selected_addons_dir_ptr == nil {
		panic("logic error, selected addon dir should _not_ be null after loading settings")
	}
	selected_addons_dir := *selected_addons_dir_ptr

	// find index of row matching selected_addons_dir

	item := app.FindResultByID(selected_addons_dir)
	if core.EmptyResult(item) {
		panic("item is empty")
	}

	guitab := tab.(*ui.GUITab)
	fullkey, present := guitab.RowIndex[item.ID]
	if !present {
		panic("item not present in row index")
	}
	guitab.ExpandRow(fullkey)

	// ...

	guitab.HighlightRow(fullkey, "#FAEBD7")

	// ---

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

		for i := 0; i < 100; i++ {
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

	// ---

	slog.Debug("init complete, waiting for UI to end...")
	ui_wg.Wait() // wait for UIs to complete before exiting
}
