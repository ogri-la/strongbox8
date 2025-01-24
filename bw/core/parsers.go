package core

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

func Identity(_ *App, val string) (interface{}, error) {
	return val, nil
}

func ParseStringAsInt(_ *App, v string) (int, error) {
	str, err := strconv.Atoi(v)
	if err != nil {
		//return 0, fmt.Errorf("cannot convert input to an integer: %w", err)
		return 0, errors.New("cannot parse input to an integer")
	}
	return str, nil
}

func ParseYesNoStringAsBool(_ *App, val string) (interface{}, error) {
	val = strings.TrimSpace(strings.ToLower(val))
	if val != "" && val[0] == 'y' {
		return true, nil
	}
	return false, nil
}

func ParseStringAsPath(_ *App, val string) (interface{}, error) {
	return filepath.Abs(val)
}

// returns a `Result` as an `interface{}` for the first Result whose ID equals `val`.
// returns `nil` if a Result not found.
func ParseStringAsResultID(app *App, val string) (interface{}, error) {
	return app.FindResultByID(val), nil
}

func ParseStringStripWhitespace(_ *App, val string) (interface{}, error) {
	newstr := strings.TrimSpace(val)
	if newstr == "" {
		return nil, errors.New("value is blank")
	}
	return newstr, nil
}
