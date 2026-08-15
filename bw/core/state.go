// put all State coordination into App methods.
// keep this file and it's logic dumb.

package core

import (
	"fmt"
	"strings"
)

// the application's data.
// modify it only through `App.UpdateState` and `App.UpdateResult`, which keep the
// index in step with the result list.
type State struct {
	Root Result `json:"-"`

	// a map of {id=>index, ...} into the Root.Item.([]Result) result list
	index map[string]int

	// a bucket of key+vals. complete free for all state modification. be careful.
	KeyVals map[string]any
}

func NewState() State {
	return State{
		Root:    Result{NS: NS{}, Item: []Result{}},
		index:   map[string]int{}, // internal map of Result.ID => state.Root.i
		KeyVals: map[string]any{},
	}
}

func (state *State) GetResults() []Result {
	return state.Root.Item.([]Result)
}

// returns the result with the given `id`,
// or an empty `Result` and an error when not found.
func (state *State) GetResult(id string) (Result, error) {
	empty_result := Result{}
	idx, present := state.index[id]
	if !present {
		return empty_result, fmt.Errorf("result with id not present: %v", id)
	}
	r := state.Root.Item.([]Result)[idx]
	return r, nil
}

// replaces the whole result list.
// the index is not rebuilt here; `App.process_update` does that after the update runs.
func (state *State) SetRoot(rl []Result) {
	state.Root.Item = rl
}

func (state *State) getIndex() map[string]int {
	return state.index
}

func (state *State) ResultIndex(id string) (int, bool) {
	idx, present := state.index[id]
	return idx, present
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
	subset := map[string]string{}
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
	subset := map[string]any{}
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

// ---

// a read-only view of a result list at a point in time, given to `StateObserver`s
// so they can compare state before and after an update.
type Snapshot struct {
	results []Result
	index   map[string]int
}

func MakeSnapshot(results []Result) *Snapshot {
	idx := map[string]int{}
	for i, r := range results {
		idx[r.ID] = i
	}
	return &Snapshot{results: results, index: idx}
}

// returns the result with the given `id`, or nil when not found.
func (s *Snapshot) GetResult(id string) *Result {
	idx, present := s.index[id]
	if !present {
		return nil
	}
	return &s.results[idx]
}

// returns the snapshotted results.
// the slice is not copied, do not modify it.
func (s *Snapshot) Results() []Result {
	return s.results
}
