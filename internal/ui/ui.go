package ui

import (
	"bw/internal/core"
	"log/slog"
	"reflect"
	"slices"
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

	// consume an event sent to the UI from the app
	Get() UIEvent

	// add an event for the UI to process.
	// essentially: ui.Get() -> processing() -> ui.Put()
	// see `ui.Dispatch`
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
	for {
		ev := ui_inst.Get() // needs to block

		slog.Info("DISPATCH processing event", "event", ev)

		//switch ev.Key() {
		switch ev.Key {

		case "row-modified":
			ui_inst.UpdateRow(ev.Val.(string))

		case "row-added":
			ui_inst.AddRow(ev.Val.(string))

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

// watches state changes and generates `UIEvent`s for a given `UI` instance.
// new results are those that are not present in the old results.
// modified results are those that are present in the old results but DeepEqual fails.
// missing results are those that are present in the old results but not in the new.
func UIEventListener(ui UI) core.Listener2 {
	callback := func(old_results, new_results []core.Result) {

		slog.Info("ui.go, UIEventListener called", "num-results", len(new_results)) //, "old", old_results, "new", new_results)

		// we have a blob of results here,
		// some with parents
		// and some with parents with parents
		// and some without.

		// we need to sort results into insertion order.
		// all parents must be added before children can be added.

		parent_present_idx := map[string]bool{}
		slices.SortFunc(new_results, func(a, b core.Result) int {
			parent_present_idx[a.ID] = true
			if a.Parent == nil {
				if b.Parent == nil {
					return 0 // neither have parents, doesn't matter what order
				}
				// a has a parent and b doesnt
				return -1
			}

			_, a_parent_present := parent_present_idx[a.Parent.ID]
			if a_parent_present {
				// a has a parent and we've sorted it already
				return 1
			}

			// a has a parent and we've not sorted it yet
			return -1
		})

		/*
			   // debugging
				for idx, result := range new_results {
					if result.Parent == nil {
						fmt.Printf("[%v] id:%v parent:nil\n\n", idx, result.ID)
					} else {
						fmt.Printf("[%v] id:%v parent:%s\n\n", idx, result.ID, result.Parent.ID)
					}
				}
		*/

		if len(old_results) == 0 {
			// if the old results are empty we don't need to do a bunch of stuff
			slog.Info("add row event, no old results", "num-new", len(new_results))
			for _, result := range new_results {
				slog.Debug("processing result from app (1)", "event", result)
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

			slog.Debug("processing result from app (2)", "event", result)

			if !old_present && new_present {
				// not present in old index, present in new index
				slog.Info("add row event, not present in old index, present in new index")
				ui.Put(UIEvent{
					Key: "row-added",
					// seems a shame to go from data to id back to data again when it hits UI instance
					Val: result.ID,
				})
				continue
			}

			if !new_present && old_present {
				// not present in new index, present in old index
				slog.Info("del row event, not present in new index, present in old index")
				ui.Put(UIEvent{
					Key: "row-removed",
					Val: result.ID,
				})
				continue
			}

			if reflect.DeepEqual(old_val, new_val) {
				slog.Debug("old and new vals are the same, no update") //, "old", old_val, "new", new_val)
				continue
			}

			// old and new vals are somehow different.
			// note! if row contains a function it will always be different.
			slog.Debug("mod row event, old and new vals are somehow different.") //, "old", old_val, "new", new_val)
			ui.Put(UIEvent{
				Key: "row-modified",
				Val: result.ID,
			})
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
