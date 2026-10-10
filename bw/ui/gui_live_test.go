package ui

import (
	"bw/core"
	"bytes"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/visualfc/atk/tk"
)

// returns the form field with the given argdef `id`.
func form_field(t *testing.T, gf *GUIForm, id string) GUIFormField {
	t.Helper()
	for _, field := range gf.Fields {
		if field.argdef.ID == id {
			return field
		}
	}
	t.Fatalf("no form field: %s", id)
	return GUIFormField{}
}

// a single test for the whole live GUI: only one Tk interpreter can run per process.
func Test_gui_live(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display, run under xvfb-run")
	}

	app := core.Start()
	app.SetDataDir(t.TempDir())
	defer app.Stop()

	var wg sync.WaitGroup
	gui := MakeGUI(app, &wg)
	app.AddObserver(gui)
	gui.Start().Wait()
	defer gui.Stop()

	gui.AddTab("everything", func(core.Result) bool { return true })
	tab := gui.GetTab("everything")

	t.Run("form widgets render, fill and submit", func(t *testing.T) {
		selected := &core.Result{ID: "selected-1"}
		service := core.Service{
			ID:    "widgets",
			Label: "Widgets",
			Fn:    noop_fn,
			Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
				{ID: "selected", FromSelection: true},
				{ID: "name", Label: "Name", Widget: core.InputWidgetTextField, Default: "bob"},
				{ID: "track", Label: "Track", Widget: core.InputWidgetSelection, Default: "Classic",
					Choice: &core.ArgChoice{ChoiceList: []any{"retail", "classic"}, Exclusivity: core.ArgChoiceExclusive,
						LabelFn: func(v any) string { return map[string]string{"retail": "Retail", "classic": "Classic"}[v.(string)] }}},
				{ID: "tags", Label: "Tags", Widget: core.InputWidgetMultiSelection,
					Choice: &core.ArgChoice{ChoiceList: []any{"a", "b", "c"}, Exclusivity: core.ArgChoiceNonExclusive}},
				{ID: "strict", Label: "Strict", Widget: core.InputWidgetCheckbox, Default: "true", Parser: core.ParseTruthyFalseyAsBool},
				{ID: "zips", Label: "Zips", Widget: core.InputWidgetFileSelection,
					FilePicker: &core.FilePickerOpts{Multiple: true, Extensions: []string{".zip"}}},
				{ID: "dir", Label: "Dir", Widget: core.InputWidgetDirSelection},
				{ID: "odd", Label: "Odd", Widget: "telepathy"},
			}},
		}

		tab.OpenForm(service, []core.KeyVal{{Key: "selected", Val: selected}})
		gf := tab.GUIForm
		assert.NotNil(t, gf)

		var args core.ServiceFnArgs
		var ferr *core.FormError
		gui.TkSync(func() {
			// defaults are shown
			assert.Equal(t, "bob", form_field(t, gf, "name").Input.Get())
			assert.Equal(t, "Classic", form_field(t, gf, "track").Input.Get())
			assert.Equal(t, "true", form_field(t, gf, "strict").Input.Get())

			form_field(t, gf, "track").Input.Set("Retail")
			form_field(t, gf, "tags").Input.Set("a\nc")
			form_field(t, gf, "strict").Input.Set("false")
			form_field(t, gf, "dir").Input.Set("/tmp")
			args, ferr = gf.SubmitFields()
		})

		assert.Nil(t, ferr)
		actual := map[string]any{}
		for _, kv := range args.ArgList {
			actual[kv.Key] = kv.Val
		}
		assert.Equal(t, selected, actual["selected"])
		assert.Equal(t, "bob", actual["name"])
		assert.Equal(t, "retail", actual["track"])
		assert.Equal(t, []any{"a", "c"}, actual["tags"])
		assert.Equal(t, false, actual["strict"])
		assert.Equal(t, "", actual["zips"]) // nothing chosen
		assert.Equal(t, "/tmp", actual["dir"])

		tab.CloseForm()
	})

	t.Run("status bar shows running jobs and WaitForIdle waits for them", func(t *testing.T) {
		assert.Equal(t, "idle", gui.StatusText())

		job := app.StartJob("checking for updates", 3)
		job.Tick(1)
		app.Flush()
		gui.TkSync(func() {})
		assert.Eventually(t, func() bool { return gui.StatusText() == "checking for updates (1/3)" }, time.Second, 5*time.Millisecond)

		go func() {
			time.Sleep(20 * time.Millisecond)
			job.Finish()
		}()
		gui.WaitForIdle()
		assert.Empty(t, app.Jobs())
		assert.Eventually(t, func() bool { return gui.StatusText() == "idle" }, time.Second, 5*time.Millisecond)
	})

	t.Run("busy rows are styled and return to normal", func(t *testing.T) {
		r := core.MakeResult(core.MakeNS("test", "test", "item"), "busy-item", "busy-item")
		app.AddReplaceResults(r).Wait()
		gui.WaitForIdle()

		background_and_foreground := func() (string, string) {
			var bg, fg string
			gui.TkSync(func() {
				fkey := tab.ItemFkeyIndex["busy-item"]
				out, _ := tk.MainInterp().EvalAsString(tab.table_widj.Tablelist.Id() + " rowcget " + fkey + " -foreground")
				fg = out
				out, _ = tk.MainInterp().EvalAsString(tab.table_widj.Tablelist.Id() + " rowcget " + fkey + " -background")
				bg = out
			})
			return bg, fg
		}

		app.UpdateResult("busy-item", func(r core.Result) core.Result {
			r.Tags.Add(core.TAG_BUSY)
			return r
		})
		gui.WaitForIdle()
		_, fg := background_and_foreground()
		assert.Equal(t, GUI_ROW_BUSY_FOREGROUND, fg)

		app.UpdateResult("busy-item", func(r core.Result) core.Result {
			r.Tags.Remove(core.TAG_BUSY)
			r.Tags.Add(core.TAG_HAS_UPDATE)
			return r
		})
		gui.WaitForIdle()
		bg, fg := background_and_foreground()
		assert.Equal(t, "", fg)
		assert.Equal(t, GUI_ROW_MARKED_COLOUR, bg)

		app.UpdateResult("busy-item", func(r core.Result) core.Result {
			r.Tags.Remove(core.TAG_HAS_UPDATE)
			return r
		})
		gui.WaitForIdle()
		bg, _ = background_and_foreground()
		assert.Equal(t, "", bg)
	})

	t.Run("a choice outside the choices fails validation", func(t *testing.T) {
		service := core.Service{
			ID: "choice", Fn: noop_fn,
			Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
				{ID: "track", Widget: core.InputWidgetSelection,
					Choice: &core.ArgChoice{ChoiceList: []any{"retail"}, Exclusivity: core.ArgChoiceExclusive}},
			}},
		}
		called := atomic.Bool{}
		service.Fn = func(*core.App, core.ServiceFnArgs) core.ServiceResult {
			called.Store(true)
			return core.ServiceResult{}
		}
		tab.OpenForm(service, nil)
		var ferr *core.FormError
		gui.TkSync(func() {
			form_field(t, tab.GUIForm, "track").Input.Set("classic")
			_, ferr = tab.GUIForm.SubmitFields()
		})
		assert.NotNil(t, ferr)

		// pressing submit with the invalid value does not call the service
		submit_btn := tab.GUIForm.Fields[len(tab.GUIForm.Fields)-1].Input.(*TKButton)
		gui.TkSync(func() { submit_btn.Invoke() })
		gui.WaitForIdle()
		assert.False(t, called.Load())
		assert.NotNil(t, tab.GUIForm, "the form stays open")
		tab.CloseForm()
	})

	t.Run("an unknown widget is logged at ERROR naming the service and argument", func(t *testing.T) {
		var log_output bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&log_output, &slog.HandlerOptions{Level: slog.LevelError})))
		defer slog.SetDefault(prev)

		service := core.Service{
			ID: "odd-service", Fn: noop_fn,
			Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
				{ID: "name", Widget: core.InputWidgetTextField},
				{ID: "odd-arg", Widget: "telepathy"},
			}},
		}
		tab.OpenForm(service, nil)
		assert.Len(t, tab.GUIForm.Fields, 3, "every argument still gets a field, plus submit")
		tab.CloseForm()

		actual := log_output.String()
		assert.Contains(t, actual, "level=ERROR")
		assert.Contains(t, actual, "odd-service")
		assert.Contains(t, actual, "odd-arg")
	})

	t.Run("removing a result removes its row and its children's rows from the index", func(t *testing.T) {
		ns := core.MakeNS("test", "test", "item")
		parent := core.MakeResult(ns, "parent-item", "parent-item")
		child := core.MakeResult(ns, "child-item", "child-item")
		child.ParentID = parent.ID
		app.AddReplaceResults(parent, child)
		gui.WaitForIdle()

		indexed := func(id string) bool {
			var present bool
			gui.TkSync(func() {
				fkey, in_item_index := tab.ItemFkeyIndex[id]
				_, in_fkey_index := tab.FkeyItemIndex[fkey]
				present = in_item_index || in_fkey_index
			})
			return present
		}
		assert.True(t, indexed(parent.ID))
		assert.True(t, indexed(child.ID))

		var tk_errors []error
		gui.TkSync(func() {
			interp := tk.MainInterp()
			prev := interp.FnErrorHandle
			interp.FnErrorHandle = func(err error) { tk_errors = append(tk_errors, err) }
			t.Cleanup(func() { gui.TkSync(func() { interp.FnErrorHandle = prev }) })
		})

		app.RemoveResult(parent.ID)
		gui.WaitForIdle()
		assert.False(t, indexed(parent.ID))
		assert.False(t, indexed(child.ID))
		assert.Empty(t, tk_errors)

		// a result added again with the same ID gets a new row
		app.AddReplaceResults(parent)
		gui.WaitForIdle()
		assert.True(t, indexed(parent.ID))
		var row_count int
		gui.TkSync(func() { row_count = tab.RowChildCount(parent.ID) })
		assert.Equal(t, 0, row_count)
	})
}

func noop_fn(_ *core.App, _ core.ServiceFnArgs) core.ServiceResult { return core.ServiceResult{} }
