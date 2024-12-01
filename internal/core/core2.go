package core

import (
	"log/slog"
	"reflect"
)

// caveats: no support for state.keyvals yet
// does it even work??

type Listener2 struct {
	ID                string
	ReducerFn         func(Result) bool
	CallbackFn        func(old_results []Result, new_results []Result)
	WrappedCallbackFn func([]Result)
}

// calls each `Listener.ReducerFn` in `listener_list` on each item in the state,
// before finally calling each `Listener.CallbackFn` on each listener's list of filtered results.
func update_state2(new_state State, listener_list []Listener2, listeners_locked bool) []Listener2 {

	var listener_list_results = make([][]Result, len(listener_list))
	//listener_list_results := [][]Result{}

	// for each result in new state, apply every listener.reducer to it.
	// we could do N passes of the result list or we could do 1 pass of the result list with N iterations over the same item.
	// N passes over the result list lends itself to parallelism, N passes over an item is simpler for sequential access.
	for _, result := range new_state.Root.Item.([]Result) {
		for listener_idx, listener_struct := range listener_list {
			//slog.Debug("calling ReducerFn", "listener", listener_struct.ID)
			reducer_results := listener_list_results[listener_idx]
			if listener_struct.ReducerFn(result) {
				reducer_results = append(reducer_results, result)
			}
			listener_list_results[listener_idx] = reducer_results
		}
	}

	// call each listener callback with it's new set of results

	empty_results := []Result{}

	updated_listener_list := []Listener2{}
	for idx, listener_results := range listener_list_results {
		listener_results := listener_results
		listener := listener_list[idx]

		slog.Debug("calling listener with new results", "listener", listener.ID, "num-results", len(listener_results))

		if listener.WrappedCallbackFn == nil {
			// first time! no old results to compare to, call the listener
			if listeners_locked {
				slog.Debug("listeners LOCKED, ignoring listener")
			} else {
				slog.Debug("no wrapped callback for listener, calling listener for first time", "listener", listener.ID, "num-results", len(listener_results))
				listener.CallbackFn(empty_results, listener_results)
			}
		} else {
			// listener has been called before.
			// todo: only call the original function if the results have changed
			if listeners_locked {
				slog.Debug("listeners LOCKED, ignoring wrapped callback")
			} else {
				slog.Debug("wrapped callback exists, calling that", "listener", listener.ID, "num-results", len(listener_results))
				listener.WrappedCallbackFn(listener_results)
			}
		}

		// set/update the wrapped callback function using the current listener results
		listener.WrappedCallbackFn = func(old_results []Result) func(new_results []Result) {
			// note! the canonical form of a pointer is a pointer and *not* it's dereferenced value!
			// if a value isn't being detected as having changed, you might be using a pointer ...
			//old_results := listener_results
			return func(new_results []Result) {
				if reflect.DeepEqual(old_results, new_results) { // if there are any functions this will always be true
					slog.Debug("wrapped listener, not calling, old results and new results are identical", "id", listener.ID)
					//slog.Info("old and new", "old", old_results, "new", new_results)
				} else {
					slog.Debug("wrapped listener, calling, new results different to old results", "id", listener.ID)
					listener.CallbackFn(old_results, new_results)
				}
			}
		}(listener_results)

		updated_listener_list = append(updated_listener_list, listener)
	}

	return updated_listener_list
}
