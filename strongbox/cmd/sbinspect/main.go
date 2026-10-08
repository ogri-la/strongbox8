// sbinspect prints, as JSON, how strongbox 8 sees an addons dir or a settings file.
// it never writes: it is for comparing strongbox 8 with strongbox 7 and for diagnosing a
// user's setup.
//
//	sbinspect addons <dir> [--game-track retail] [--strict=true]
//	sbinspect settings <file>
package main

import (
	"bw/core"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	strongbox "strongbox/src"
)

const usage = `usage:
  sbinspect addons <dir> [--game-track retail] [--strict=true]
  sbinspect settings <file>`

// writes `val` to `out` as indented JSON.
func write_json(out io.Writer, val any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(val)
}

// returns the canonical game track for `given`, accepting strongbox's aliases.
func parse_game_track(given string) (strongbox.GameTrackID, error) {
	if alias, present := strongbox.GAMETRACK_ALIAS_MAP[given]; present {
		given = alias
	}
	if !strongbox.SUPPORTED_GAME_TRACKS.Contains(given) {
		return "", fmt.Errorf("unknown game track: %s", given)
	}
	return given, nil
}

func run_addons(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("addons", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	game_track := fs.String("game-track", strongbox.GAMETRACK_RETAIL, "the addons dir's game track")
	strict := fs.Bool("strict", true, "only use releases for the game track")
	// flags may come after the directory
	positional := []string{}
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		args = fs.Args()
		if len(args) > 0 {
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	if len(positional) != 1 {
		return errors.New("addons: expected one addons directory")
	}
	gt, err := parse_game_track(*game_track)
	if err != nil {
		return err
	}
	if !core.DirExists(positional[0]) {
		return fmt.Errorf("addons: not a directory: %s", positional[0])
	}
	ad := strongbox.MakeAddonsDir(positional[0])
	ad.GameTrackID = gt
	ad.Strict = *strict
	report, err := strongbox.InspectAddons(ad)
	if err != nil {
		return err
	}
	return write_json(out, report)
}

func run_settings(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("settings: expected one settings file")
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	report, err := strongbox.InspectSettings(b, core.DirExists)
	if err != nil {
		return err
	}
	return write_json(out, report)
}

// runs the command in `args`, without the program name, writing the report to `out`.
func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "addons":
		return run_addons(args[1:], out)
	case "settings":
		return run_settings(args[1:], out)
	}
	return fmt.Errorf("unknown command: %s\n%s", args[0], usage)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
