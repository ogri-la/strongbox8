package ui

import (
	"bw/core"
	"sync"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

/*
var test_ns = core.MakeNS("bw", "test", "ns")

// create app, update state
func TestAppAddResults(t *testing.T) {
	app := core.NewApp()

	var ui_wg sync.WaitGroup
	gui := MakeGUI(app, &ui_wg)
	app.AddObserver(gui)

	app.AppendResults(core.NewResult(test_ns, "", "dummy-id"))
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

// insertion order still needs to happen even if we're missing parents.
func Test_sort_insertion_order__missing_parents(t *testing.T) {
	expected := []core.Result{
		{ID: "foo", ParentID: "bup"}, // nothing with `.ID=bup` exists
		{ID: "bar", ParentID: "foo"},
		{ID: "baz", ParentID: "bar"},
	}
	given := []core.Result{
		{ID: "baz", ParentID: "bar"},
		{ID: "bar", ParentID: "foo"},
		{ID: "foo", ParentID: "bup"},
	}
	assert.Equal(t, expected, sort_insertion_order(given))
}

func TestConfirmAndRunService(t *testing.T) {
	app := core.NewApp()
	var wg sync.WaitGroup
	gui := MakeGUI(app, &wg)
	gui.async = func(fn func()) { fn() } // there is no Tk main loop to hand off to

	ran := make(chan bool, 1)
	service := core.Service{
		ID:    "destroy",
		Label: "Destroy",
		Fn: func(_ *core.App, _ core.ServiceFnArgs) core.ServiceResult {
			ran <- true
			return core.ServiceResult{}
		},
		Confirm: func(_ *core.App, _ core.ServiceFnArgs) string { return "really?" },
	}

	asked := ""
	gui.Confirm = func(_ string, message string) bool {
		asked = message
		return false
	}
	gui.ConfirmAndRunService(service, core.NewServiceFnArgs(), nil)
	assert.Equal(t, "really?", asked)
	select {
	case <-ran:
		t.Fatal("declined service ran")
	case <-time.After(50 * time.Millisecond):
	}

	gui.Confirm = func(_ string, _ string) bool { return true }
	gui.ConfirmAndRunService(service, core.NewServiceFnArgs(), nil)
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("confirmed service did not run")
	}
}

func TestConfirmAndRunService__no_confirmation(t *testing.T) {
	app := core.NewApp()
	var wg sync.WaitGroup
	gui := MakeGUI(app, &wg)
	gui.async = func(fn func()) { fn() } // there is no Tk main loop to hand off to
	gui.Confirm = func(_ string, _ string) bool {
		t.Fatal("asked for confirmation")
		return false
	}
	ran := make(chan bool, 1)
	service := core.Service{Fn: func(_ *core.App, _ core.ServiceFnArgs) core.ServiceResult {
		ran <- true
		return core.ServiceResult{}
	}}
	gui.ConfirmAndRunService(service, core.NewServiceFnArgs(), nil)
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("service did not run")
	}
}

func Test_status_text(t *testing.T) {
	assert.Equal(t, "idle", status_text(nil))
	given := []core.JobInfo{
		{Name: "checking for updates", Done: 4, Total: 10},
		{Name: "downloading", Done: 2},
		{Name: "refreshing"},
	}
	assert.Equal(t, "checking for updates (4/10) · downloading (2) · refreshing", status_text(given))
}

func Test_row_style(t *testing.T) {
	assert.Equal(t, map[string]string{"background": "", "foreground": ""}, row_style(mapset.NewSet[core.Tag](), "#fff"))
	assert.Equal(t, map[string]string{"background": "#fff", "foreground": ""}, row_style(mapset.NewSet(core.TAG_HAS_UPDATE), "#fff"))
	assert.Equal(t, GUI_ROW_MUTED_FOREGROUND, row_style(mapset.NewSet(core.TAG_MUTED), "#fff")["foreground"])
	assert.Equal(t, GUI_ROW_BUSY_FOREGROUND, row_style(mapset.NewSet(core.TAG_MUTED, core.TAG_BUSY), "#fff")["foreground"])
}

func Test_about_text(t *testing.T) {
	state := core.NewState()
	state.SetKeyAnyVal("bw.app.name", "bw")
	state.SetKeyAnyVal("bw.app.version", "0.1.0")
	title, message := about_text(&state)
	assert.Equal(t, "bw", title)
	assert.Equal(t, "version: 0.1.0", message)

	state.SetKeyAnyVal("app.name", "strongbox")
	state.SetKeyAnyVal("app.about", `version: 8.0.0\nhttps://github.com/ogri-la/strongbox`)
	state.SetKeyAnyVal("app.update-available", "8.1.0")
	title, message = about_text(&state)
	assert.Equal(t, "strongbox", title)
	assert.Equal(t, "version: 8.0.0\nhttps://github.com/ogri-la/strongbox\n\nversion 8.1.0 is available", message)

	state.SetKeyAnyVal("app.update-url", "https://github.com/ogri-la/strongbox/releases")
	_, message = about_text(&state)
	assert.Equal(t, "version: 8.0.0\nhttps://github.com/ogri-la/strongbox\n\nversion 8.1.0 is available\nhttps://github.com/ogri-la/strongbox/releases", message)
}
