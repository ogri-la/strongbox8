package main

import (
	"bw/internal/bw"
	"bw/internal/core"
	"bw/internal/strongbox"
	"bw/internal/ui"
	"flag"
	"fmt"
	"os"
	"sync"

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
	app.RegisterProvider(strongbox.Provider(app))

	app.StartProviders()
	defer app.StopProviders() // clean up

	// -- init UI
	// whatever UI(s) we have,
	// the providers are ready to go.

	var ui_wg sync.WaitGroup

	cli := ui.CLI(app, &ui_wg)
	cli.Start().Wait() // this seems to work well! cli open in terminal, gui open in new window

	gui := ui.GUI(app, &ui_wg)
	gui.Start().Wait()

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

	gui.AddTab("some title").Wait()
	gui.AddTab("someotherid").Wait()
	gui.SetActiveTab("someotherid").Wait()

	ui_wg.Wait() // wait for UIs to complete
}
