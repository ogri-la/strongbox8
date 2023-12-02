package main

import (
	"bw/internal/bw"
	"bw/internal/core"
	"bw/internal/strongbox"
	"bw/internal/ui"
	"os"

	"log/slog"
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func main() {
	app := core.Start()

	// init providers. can't do this from core because of circular dependencies.
	// providers must register their services with `core`.

	bw.Start(app)
	strongbox.Start(app)

	// start ui

	ui.CLI(app)
}
