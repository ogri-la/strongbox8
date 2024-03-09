package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// state is transformed by a function,
// a listener is called,
// the transformed state and listener wrapper are returned.
func Test_update_state2(t *testing.T) {
	original_state := NewState()

	called := false
	listener_list := []Listener2{
		{
			ID: "Test",
			ReducerFn: func(r Result) bool {
				return true
			},
			CallbackFn: func(rl []Result) {
				called = true
			},
		},
	}

	transform_state_fn := func(s State) State {
		rl := s.Root.Item.([]Result)
		rl = append(rl, NewResult(test_ns, nil, "some-id"))
		s.Root.Item = rl
		return s
	}

	new_state, new_listener_list := update_state2(*original_state, transform_state_fn, listener_list)

	assert.True(t, called)
	assert.NotEqual(t, original_state, new_state)
	assert.NotEqual(t, listener_list, new_listener_list)
}

// state is transformed by a function,
// a listener is called,
// the transformed state and listener wrapper are returned,
// the state is transformed again,
// a listener is not called because the results are identical.
func Test_update_state2__idempotent_callback(t *testing.T) {
	original_state := NewState()

	called := false
	listener_list := []Listener2{
		{
			ID: "Test",
			ReducerFn: func(r Result) bool {
				return true
			},
			CallbackFn: func(rl []Result) {
				// toggles the value.
				// it will be switched to true on the first call,
				// then toggled to false if called again.
				// in this test it should should remain true and not be toggled.
				called = !called
			},
		},
	}

	// transform state by adding item
	transform_state_fn := func(s State) State {
		rl := s.Root.Item.([]Result)
		rl = append(rl, NewResult(test_ns, nil, "some-id"))
		s.Root.Item = rl
		return s
	}

	new_state, new_listener_list := update_state2(*original_state, transform_state_fn, listener_list)
	assert.True(t, called)

	// no new items, just same the state as before
	transform_state_fn2 := func(s State) State {
		return s
	}

	new_new_state, _ := update_state2(*new_state, transform_state_fn2, new_listener_list)

	assert.True(t, called)
	assert.Equal(t, new_state, new_new_state)

	// the new listener list will always be a new wrapper around the previous results
	//assert.Equal(t, new_listener_list, new_new_listener_list)
}

func Test_update_state2__callback_creates_listener(t *testing.T) {

	app := NewApp()
	app.AddListener(Listener2{
		ID: "Test",
		ReducerFn: func(r Result) bool {
			return true
		},
		CallbackFn: func(rl []Result) {
			app.AddListener(Listener2{
				ID: "InnerTest",
				ReducerFn: func(r Result) bool {
					return false
				},
				CallbackFn: func(_ []Result) {
					fmt.Println("inner called")
				},
			})
			fmt.Println(len(app.ListenerList))
		},
	})

	assert.Equal(t, 1, len(app.ListenerList))
	app.AddResults(NewResult(test_ns, nil, "some-id"))

	assert.Equal(t, 2, len(app.ListenerList))

	//app.AddResults(NewResult(test_ns, nil, "some-new-id"))

}
