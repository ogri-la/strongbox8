package ui

import (
	"bw/internal/core"
	"log/slog"
	"reflect"
	"sync"
)

type UIEvent struct {
	NS  core.NS
	Key string
	Val any
}

/*
type UIEvent interface {
	NS() core.NS
	Key() string
	Val() any
}
*/

/*
func keyval(key string, val any) UIEvent {
	return UIEvent{Key: key, Val: val}
        }
*/

type UIEventChan chan (UIEvent)

type UIRow interface {

	// if row has children, expands children.
	// if row has lazy children, lazy children are realised to a depth of 1.
	Expand()

	// returns true if the row has children and the children have been toggled open.
	Expanded() bool

	// returns true if row has children.
	HasChildren() bool

	// returns true if row has children and children are lazily loaded.
	HasLazyChildren()

	// returns true if row has children and children are lazily loaded and children have been loaded.
	LazyChildrenRealised()
}

// a tab contains:
// - a table of rows
// - a way to filter those rows
// - a way to expand lazily fetched rows
// - a way to select a row
// - a way to see the details of selected rows
// - a way to apply call provider services on selected rows
// a table row is a core.Result
type UITab interface {
	// the name of this tab.
	//GetTitle() string
	//SetTitle(title string)

	// -- columns
	// all columns are always present,
	// but they can be hidden as necessary.
	//HideColumn()
	//ShowColumn()

	// -- rows
	AddRow()
	AddManyRows()
	UpdateRow()
	//UpdateManyRows()
	//RemoveRow()
	//RemoveManyRows()
	//RemoveAllRows()

	// -- row selection
	// select a single row. selecting a row deselects all other rows.
	//SelectRow()
	// select many rows, not necessarily continguous.
	//SelectManyRows()
	// if row is selected, the row is now unselected.
	//DeselectRow()

	// -- detail pane
	// if a single item is selected, it is the detail for that.
	// if many items are selected, it is the detail for them.
	//OpenDetail()
	//CloseDetail()
}

// what a UI should be able to do
type UI interface {
	Start() *sync.WaitGroup
	Stop()

	// the name of this UI.
	// if gui, the name of the window.
	SetTitle(string)

	// the UI is sent notifications of state changes from the app.
	// this fetches the oldest event on the queue.
	// todo: will it block when the queue is empty??
	Get() UIEvent

	// the UI can send notifications of state changes to the app.
	// notify whoever is listening that the UI has changed it's state
	Put(UIEvent)

	// tab handling
	GetTab(title string) UITab // finds something implementing a UITab using what it can from the UIEvent
	AddTab(title string, view core.ViewFilter) *sync.WaitGroup
	//RemoveTab(id string)

	// a UI is responsible for it's own internal set of results.
	// when the app adds/updates/deletes a result, the UI is told about it.
	AddRow(id string)
	UpdateRow(id string)
	DeleteRow(id string)
}

// ---

// generic bridge for incoming events from app to a UI instance and it's methods
func Dispatch(ui_inst UI) {

	var wg sync.WaitGroup
	slog.Info("DISPATCH started")
	for {

		ev := ui_inst.Get() // needs to block

		slog.Info("DISPATCH looping", "event", ev)

		//switch ev.Key() {
		switch ev.Key {

		case "row-modified":
			slog.Info("ui.go, DISPATCH, update row")
			ui_inst.UpdateRow(ev.Val.(string))

		case "row-added":
			wg.Add(1)
			go func() {
				defer wg.Done()
				ui_inst.AddRow(ev.Val.(string))
			}()
			wg.Wait()

		case "row-deleted":
			ui_inst.DeleteRow(ev.Val.(string))

		// convenience? creates a new tab (somehow) and adds it to the UI
		case "add-tab":
			// todo: check tab exists first?
			title, is_str := ev.Val.(string)
			if is_str {
				ui_inst.AddTab(title, func(_ core.Result) bool { return true })
			}
		case "set-title":
			val, is_str := ev.Val.(string)
			if is_str {
				ui_inst.SetTitle(val)
			} else {
				slog.Error("refusing to set title, value type is unsupported")
			}
		default:
			slog.Error("ignoring unhandled event type", "event-type", ev.Key)
		}
	}
}

// generates UI events for a given UI instance.
// create a new UI instance, attach this listener, and when the application state changes UIEvent structs are pushed to the UI using `ui.Put`.
func UIEventListener(ui UI) core.Listener2 {

	// new results are those that are not present in the old results
	// modified results are those that are present in the old results but DeepEqual fails
	// missing results are those that are present in the old results but not in the new

	// note! we're not seeing _all_ results from application state, just those that core.Listener.ReducerFn returned `true` for.

	callback := func(old_results, new_results []core.Result) {

		slog.Info("ui.go, UIEventListener called") //, "old", old_results, "new", new_results)

		if len(old_results) == 0 {
			// if the old results are empty we don't need to do a bunch of stuff
			slog.Info("add row event, no old results", "num-new", len(new_results))
			for _, result := range new_results {
				ui.Put(UIEvent{
					Key: "row-added",
					Val: result.ID,
				})
			}

			// and that's all.
			return
		}

		old_idx := map[string]any{}

		for _, result := range old_results {
			old_idx[result.ID] = result
		}

		new_idx := map[string]any{}
		for _, result := range new_results {
			new_idx[result.ID] = result
		}

		for _, result := range new_results {

			old_val, old_present := old_idx[result.ID]
			new_val, new_present := new_idx[result.ID]

			if !old_present && new_present {
				// not present in old index, present in new index
				slog.Info("add row event, not present in old index, present in new index")
				ui.Put(UIEvent{
					Key: "row-added",
					Val: result.ID,
				})
			}

			if !new_present && old_present {
				// not present in new index, present in old index
				slog.Info("del row event, not present in new index, present in old index")
				ui.Put(UIEvent{
					Key: "row-removed",
					Val: result.ID,
				})
			}

			if reflect.DeepEqual(old_val, new_val) {
				slog.Info("old and new vals are the same, no update") //, "old", old_val, "new", new_val)
				continue
			} else {
				// old and new vals are somehow different.
				// note! if row contains a function it will always be different.
				slog.Info("mod row event, old and new vals are somehow different.") //, "old", old_val, "new", new_val)
				ui.Put(UIEvent{
					Key: "row-modified",
					Val: result.ID,
				})
			}

		}
	}

	reducer := func(core.Result) bool {
		return true
	}
	return core.Listener2{
		ID:         "ui-event-listener",
		ReducerFn:  reducer,
		CallbackFn: callback,
	}
}
