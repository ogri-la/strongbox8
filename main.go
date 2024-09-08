package main

import (
	"bw/internal/core"
	"bw/src/ui"
	"flag"
	"fmt"
	"os"

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
	slog.Info("app started", "app", app)

	cli := ui.CLI(app)
	cli.Start()

	// -- init UI
	// always start the UI before providers.
	// the UI can provide feedback about the state of providers.

	/*
		// go!
		go ui.CLI(app) // this seems to work well! cli open in terminal, gui open in new window
		ui.StartGUI(app)

		// init providers. can't do this from core because of circular dependencies.
		// providers must register their services with `core`.

		bw.Start(app)
		strongbox.Start(app)

		// start UI
		ui.StartCLI(app)
	*/
}
