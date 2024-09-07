package main

import (
	"bw/internal/bw"
	"bw/internal/core"
	"bw/internal/strongbox"
	"bw/internal/ui"
	"flag"
	"fmt"
	"os"

	"log/slog"

	"github.com/lmittmann/tint"
)

func stderr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func init() {
	logging_level_ptr := flag.String("verbosity", "info", "level is one of 'debug', 'info', 'warn', 'error', 'fatal'")
	flag.Parse()

	// ---

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
	/*
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: logging_level,
		})))
	*/
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: logging_level})))
}

func main() {
	app := core.Start()

	// init providers. can't do this from core because of circular dependencies.
	// providers must register their services with `core`.

	bw.Start(app)
	strongbox.Start(app)

	// start UI
	ui.StartCLI(app)

	// go!
	go ui.CLI(app) // this seems to work well! cli open in terminal, gui open in new window

	ui.StartGUI(app)
}
