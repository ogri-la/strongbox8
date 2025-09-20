// put all State coordination into App methods.
// keep this file and it's logic dumb.

package core

import (
	"fmt"
	"log/slog"
	"strings"
)

type State struct {
	Root Result `json:"-"`

	// a map of {id=>index, ...} into the Root.Item.([]Result) result list
	index map[string]int

	// a bucket of key+vals. complete free for all state modification. be careful.
	KeyVals map[string]any

	ListenerList []Listener
}

func NewState() State {
	return State{
		Root:         Result{NS: NS{}, Item: []Result{}},
		index:        map[string]int{}, // internal map of Result.ID => state.Root.i
		KeyVals:      map[string]any{},
		ListenerList: []Listener{},
	}
}

func (state *State) GetResults() []Result {
	return state.Root.Item.([]Result)
}

func (state *State) GetResult(id string) (Result, error) {
	empty_result := Result{}
	idx, present := state.index[id]
	if !present {
		return empty_result, fmt.Errorf("result with id not present: %v", id)
	}
	r := state.Root.Item.([]Result)[idx]
	return r, nil
}

func (state *State) SetRoot(rl []Result) {
	state.Root.Item = rl
}

func (state *State) GetIndex() map[string]int {
	return state.index
}

// ---

func (state *State) AddListener(new_listener Listener) {
	slog.Debug("adding listener", "id", new_listener.ID)
	state.ListenerList = append(state.ListenerList, new_listener)
}

// ---

// returns the value stored for the given `key` as a string.
// returns an empty string if the value doesn't exist.
// returns an empty string if the value stored isn't a string.
func (state *State) GetKeyVal(key string) string {
	val, present := state.KeyVals[key]
	if !present {
		return ""
	}
	str, isstr := val.(string)
	if !isstr {
		return ""
	}
	return str
}

// returns the value stored for the given `key`.
// return nil if the key doesn't exist.
func (state *State) GetKeyAnyVal(key string) any {
	val, present := state.KeyVals[key]
	if !present {
		return nil
	}
	return val
}

// returns a subset of `state.KeyVals` for all keys starting with given `prefix` whose values are strings.
func (state *State) SomeKeyVals(prefix string) map[string]string {
	subset := make(map[string]string)
	for key, val := range state.KeyVals {
		valstr, isstr := val.(string)
		if isstr && strings.HasPrefix(key, prefix) {
			subset[key] = valstr
		}
	}
	return subset
}

// returns a subset of `state.KeyVals` for all keys starting with given `prefix`.
// `state.KeyVals` contains mixed typed values so use with caution!
func (state *State) SomeKeyAnyVals(prefix string) map[string]any {
	subset := make(map[string]any)
	for key, val := range state.KeyVals {
		if strings.HasPrefix(key, prefix) {
			subset[key] = val
		}
	}
	return subset
}

func (state *State) SetKeyAnyVal(key string, val any) {
	state.KeyVals[key] = val
}
