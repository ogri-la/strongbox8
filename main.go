package main

import (
	"bw/internal/core"
	"bw/src/bw"
	"bw/src/strongbox"
	"bw/src/ui"
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

	// -- init UI
	// a basic guarantee is that whatever UI we have,
	// the providers are ready to go.

	var wg sync.WaitGroup
	go ui.CLI(app, &wg).Start() // this seems to work well! cli open in terminal, gui open in new window
	ui.GUI(app, &wg).Start()

	wg.Wait()

	app.StopProviders()
}
