package core

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

func Identity(_ *App, val string) (any, error) {
	return val, nil
}

// returns `v` as an int.
// returns an `int` rather than an `any`, so it is not a `ParseFn`.
func ParseStringAsInt(_ *App, v string) (int, error) {
	str, err := strconv.Atoi(v)
	if err != nil {
		//return 0, fmt.Errorf("cannot convert input to an integer: %w", err)
		return 0, errors.New("cannot parse input to an integer")
	}
	return str, nil
}

// returns `true` when `val` starts with a 'y' or a 't', ignoring case and surrounding
// whitespace, and `false` for anything else.
// never returns an error.
func ParseTruthyFalseyAsBool(_ *App, val string) (any, error) {
	val = strings.TrimSpace(strings.ToLower(val))
	if val != "" && (val[0] == 'y' || val[0] == 't') {
		return true, nil
	}
	return false, nil
}

func ParseStringAsPath(_ *App, val string) (any, error) {
	return filepath.Abs(val)
}

// returns the `Result` whose ID equals `val`, as an `any`.
// returns an empty `Result` when not found, never an error.
// pair with `HasResultValidator` to reject the empty case.
func ParseStringAsResultID(app *App, val string) (any, error) {
	return app.FindResultByID(val), nil
}

func ParseStringStripWhitespace(_ *App, val string) (any, error) {
	newstr := strings.TrimSpace(val)
	if newstr == "" {
		return nil, errors.New("value is blank")
	}
	return newstr, nil
}
