package main

import (
	"bw/internal/bw"
	"bw/internal/core"
	"bw/internal/ui"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/lmittmann/tint"
)

func stderr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func main() {

	// -- cli handling

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

	// -- init

	app := core.Start()

	// -- init providers

	app.RegisterProvider(bw.Provider(app))
	//app.RegisterProvider(strongbox.Provider(app))

	app.StartProviders()
	defer app.StopProviders() // clean up

	// -- init UI
	// whatever UI(s) we have,
	// the providers are ready to go.

	var ui_wg sync.WaitGroup

	// totally works
	//cli := ui.CLI(app, &ui_wg)
	//cli.Start().Wait() // this seems to work well! cli open in terminal, gui open in new window

	gui := ui.GUI(app, &ui_wg)

	listener := ui.UIEventListener(gui)
	app.AddListener(listener)

	gui.Start().Wait()

	// do not filter results (yet) - NOT ACTUALLY DOING ANYTHING
	all_results := func(r core.Result) bool {
		return true
	}
	gui.AddTab("all", all_results).Wait()

	// now we want to control the user interfaces.
	// each UI instance has it's own state that isn't synchronised with the app.
	// this means we could, theoretically, have multiple GUIs open,
	// all operating on the same app state but with different 'views' of the same data.

	// gui events will happen asynchronously.
	// starting the GUI, adding a tab, toggling a widget, etc, all happen in their own time.
	// for each of these events I want something to signal that it's complete: a waitgroup!
	//    gui.AddTab(...).Wait()

	/*
		for i := 0; i < 10; i++ {
			gui.AddTab(fmt.Sprintf("Foo: %d", i)).Wait()
		}
	*/

	foo1 := core.Result{ID: "foo1", Item: map[string]string{"path": "./foo1"}}

	bar1 := core.Result{ID: "bar1", Item: map[string]string{"path": "./bar1"}}
	baz1 := core.Result{ID: "baz1", Item: map[string]string{"path": "./baz1"}}
	bup1 := core.Result{ID: "bup1", Item: map[string]string{"path": "./bup1"}}

	foo2 := core.Result{ID: "foo2", Item: map[string]string{"path": "./foo2"}}
	bar2 := core.Result{ID: "bar2", Item: map[string]string{"path": "./bar2"}}

	bup1.Parent = &baz1
	baz1.Parent = &bar1
	bar1.Parent = &foo1

	bar2.Parent = &foo2

	app.AddResults(foo1, bar1, baz1, bup1, foo2, bar2)

	//app.AddResults(foo1)

	/*
		app.UpdateResult("foo1", func(r core.Result) core.Result {
			slog.Info("updating result!", "r", r)
			r.Item.(map[string]string)["path"] = "1!1"
			return r
		})
	*/

	for i := 0; i < 100; i++ {
		i := i
		app.UpdateResult("foo1", func(r core.Result) core.Result {
			r.Item.(map[string]string)["path"] = strconv.Itoa(i + 1)
			return r
		})

		slog.Info("---------- SL:EEEEEPING _------------")
		time.Sleep(100 * time.Millisecond)

	}

	/*
		gui.AddTab("addons", func(r core.Result) bool {
			return r.NS == strongbox.NS_ADDON_DIR
		}).Wait()

		gui.AddTab("search", func(r core.Result) bool {
			return r.NS == strongbox.NS_CATALOGUE
		}).Wait()
	*/
	// totally works
	//gui.SetActiveTab("search").Wait()

	// ---

	ui_wg.Wait() // wait for UIs to complete before exiting
}
