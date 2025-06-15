package ui

import (
	"bw/core"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
var test_ns = core.NewNS("bw", "test", "ns")

// create app, update state
func TestAppAddResults(t *testing.T) {
	app := core.NewApp()

	var ui_wg sync.WaitGroup
	gui := NewGUI(app, &ui_wg)

	listener := UIEventListener(gui)
	app.AddListener(listener)

	app.AddResults(core.NewResult(test_ns, "", "dummy-id"))
	app.ProcessUpdate()

	assert.Equal(t, core.Result{}, app.StateRoot())
}
*/

func Test_sort_insertion_order__empty(t *testing.T) {
	expected := []core.Result{}
	given := []core.Result{}
	assert.Equal(t, expected, sort_insertion_order(given))
}

func Test_sort_insertion_order__already_sorted(t *testing.T) {
	expected := []core.Result{
		{ID: "foo", ParentID: ""},
		{ID: "bar", ParentID: "foo"},
		{ID: "baz", ParentID: "bar"},
	}
	given := []core.Result{
		{ID: "foo", ParentID: ""},
		{ID: "bar", ParentID: "foo"},
		{ID: "baz", ParentID: "bar"},
	}
	assert.Equal(t, expected, sort_insertion_order(given))

}

func Test_sort_insertion_order(t *testing.T) {
	expected := []core.Result{
		{ID: "foo", ParentID: ""},
		{ID: "bar", ParentID: "foo"},
		{ID: "baz", ParentID: "bar"},
	}
	given := []core.Result{
		{ID: "bar", ParentID: "foo"},
		{ID: "baz", ParentID: "bar"},
		{ID: "foo", ParentID: ""},
	}
	assert.Equal(t, expected, sort_insertion_order(given))
}

func Test_sort_insertion_order__deeply_nested(t *testing.T) {
	expected := []core.Result{
		{ID: "foo", ParentID: ""},
		{ID: "bar", ParentID: ""},
		{ID: "baz", ParentID: ""},

		// --- children
		{ID: "foo-1", ParentID: "foo"}, // foo.foo-1
		{ID: "foo-2", ParentID: "foo"}, // foo.foo-2
		{ID: "foo-3", ParentID: "foo"}, // foo.foo-3

		// (order is preserved)
		{ID: "bar-3", ParentID: "bar"}, // bar.bar-3
		{ID: "bar-2", ParentID: "bar"}, // bar.bar-2
		{ID: "bar-1", ParentID: "bar"}, // bar.bar-1

		// --- grand children
		{ID: "foo-1-3", ParentID: "foo-1"}, // foo.foo-1.foo-1-3
		{ID: "foo-1-1", ParentID: "foo-1"}, // foo.foo-1.foo-1-1
		{ID: "foo-1-2", ParentID: "foo-1"}, // foo.foo-1.foo-1-2
	}
	given := []core.Result{
		{ID: "foo-1-3", ParentID: "foo-1"}, // foo.foo-1.foo-1-3
		{ID: "foo-1", ParentID: "foo"},     // foo.foo-1
		{ID: "foo-2", ParentID: "foo"},     // foo.foo-2
		{ID: "foo", ParentID: ""},
		{ID: "foo-3", ParentID: "foo"}, // foo.foo-3
		{ID: "bar-3", ParentID: "bar"}, // bar.bar-3
		{ID: "bar-2", ParentID: "bar"}, // bar.bar-2
		{ID: "bar-1", ParentID: "bar"}, // bar.bar-1
		{ID: "bar", ParentID: ""},
		{ID: "foo-1-1", ParentID: "foo-1"}, // foo.foo-1.foo-1-1
		{ID: "foo-1-2", ParentID: "foo-1"}, // foo.foo-1.foo-1-2
		{ID: "baz", ParentID: ""},
	}
	actual := sort_insertion_order(given)

	/*
		for _, a := range actual {
			fmt.Printf("ID: %v ParentID: %v\n", a.ID, a.ParentID)
		}
	*/

	assert.Equal(t, expected, actual)
}
