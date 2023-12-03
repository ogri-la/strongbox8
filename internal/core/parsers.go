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

// recursive.
// I imagine it's going to be very easy to create infinite recursion with pointers ...
func find_result_by_id(result Result, id string) Result {
	if EmptyResult(result) {
		return result
	}

	if result.ID == id {
		//common.Stderr(fmt.Sprintf("found match: %s\n", common.QuickJSON(result)))
		return result
	}

	switch t := result.Item.(type) {
	case Result:
		// we have a Result.Result, recurse
		return find_result_by_id(t, id)

	case []Result:
		// we have a Result.[]Result, recurse on each
		for _, r := range t {
			rr := find_result_by_id(r, id)
			if EmptyResult(rr) {
				continue
			}
			// match! return what was found
			return rr
		}

	default:
		//stderr(fmt.Sprintf("can't inspect Result.Payload of type: %T\n", t))
	}

	return Result{}
}

// returns a `Result` as an `interface{}` for the first Result whose ID equals `val`.
// returns `nil` if a Result not found.
func ParseStringAsResultID(app *App, val string) (interface{}, error) {
	searchable := NewResult(NS{}, app.State.ResultList)
	return find_result_by_id(searchable, val), nil
}

func ParseStringStripWhitespace(_ *App, val string) (interface{}, error) {
	newstr := strings.TrimSpace(val)
	if newstr == "" {
		return nil, errors.New("value is blank")
	}
	return newstr, nil
}
