package ui

import (
	"bw/core"
	"log/slog"
	"reflect"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
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

type UIEventChan chan ([]UIEvent)

// ---

type Column struct {
	Title       string // what to show as this column's name
	HiddenTitle bool   // is column's name displayed?
	Hidden      bool   // is column hidden?
	// halign
	// resizable
	// width
	// wrap
}

// ---

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

	SetColumnAttrs([]Column)
}

// what a UI should be able to do
type UI interface {
	Start() *sync.WaitGroup
	Stop()

	// the name of this UI.
	// if gui, the name of the window.
	SetTitle(string)

	// consume an event sent to the UI from the app
	Get() []UIEvent

	// add an event for the UI to process.
	// essentially: ui.Get() -> processing() -> ui.Put()
	// see `ui.Dispatch`
	Put(...UIEvent)

	// tab handling
	GetTab(title string) UITab // finds something implementing a UITab
	AddTab(title string, view core.ViewFilter) *sync.WaitGroup
	//RemoveTab(id string)

	// a UI is responsible for it's own internal set of results.
	// when the app adds/updates/deletes a result, the UI is told about it.
	AddRow(id ...string)
	UpdateRow(id string)
	DeleteRow(id string)
}

// ---

// watches state changes and generates `UIEvent`s for a given `UI` instance.
// new results are those that are not present in the old results.
// modified results are those that are present in the old results but DeepEqual fails.
// missing results are those that are present in the old results but not in the new.
func UIEventListener(ui UI) core.Listener {
	callback := func(old_results, new_results []core.Result) {

		slog.Debug("ui.go, UIEventListener called", "num-results", len(new_results)) //, "old", old_results, "new", new_results)

		// we have a blob of results here,
		// some with parents
		// and some with parents with parents
		// and some without.

		// we need to sort results into insertion order.
		// all parents must be added before children can be added.

		parent_present_idx := map[string]bool{}

		acc := []core.Result{}
		bounced := map[string]int{}

		original_length := len(new_results)

		if len(new_results) != 0 {
			// while the accumulator has fewer items than the given `new_results` ...
			for i := 0; len(acc) < original_length; i++ {
				res := new_results[i]

				if res.ParentID == "" {
					acc = append(acc, res)
					parent_present_idx[res.ID] = true
					continue
				}

				// has a parent, parent is present

				_, parent_present := parent_present_idx[res.ParentID]
				if parent_present {
					acc = append(acc, res)
					parent_present_idx[res.ID] = true
					continue
				}

				if bounced[res.ID] > len(new_results) {
					// todo: should we ever allow this condition?
					// what if we're updating a single existing value?
					slog.Error("new_results contains an orphaned result", "r", res.ID)
					panic("")
				}

				// has a parent, parent is not present, bounce result to end of list
				new_results = append(new_results, res)
				// ensure we keep a record of how many times it bounced.
				// worst case scenario it bounces from the very beginning to the very end of the results
				bounced[res.ID] += 1
			}
		}

		new_results = acc

		// debugging

		to_be_added := []UIEvent{}
		to_be_updated := []UIEvent{}
		to_be_deleted := []UIEvent{}

		if len(old_results) == 0 {

			// if the old results are empty we don't need to do a bunch of stuff

			slog.Debug("add row event, no old results", "num-new", len(new_results))
			for _, result := range new_results {
				slog.Debug("processing result from app (1)", "event", result)
				to_be_added = append(to_be_added, UIEvent{
					Key: "row-added",
					Val: result.ID,
				})
			}
		} else {

			// if not, we need to figure out which need to be added, modified and deleted

			old_idx := map[string]core.Result{}

			for _, result := range old_results {
				old_idx[result.ID] = result
			}

			new_idx := map[string]core.Result{}
			for _, result := range new_results {
				new_idx[result.ID] = result
			}

			old_set := mapset.NewSetFromMapKeys(old_idx)
			new_set := mapset.NewSetFromMapKeys(new_idx)

			ids_to_be_deleted := old_set.Difference(new_set) // items in old not in new
			ids_to_be_added := new_set.Difference(old_set)   // items in new not in old

			for id := range ids_to_be_deleted.Iter() {
				to_be_deleted = append(to_be_deleted, UIEvent{
					Key: "row-deleted",
					Val: id,
				})
			}

			for id := range ids_to_be_added.Iter() {
				to_be_added = append(to_be_added, UIEvent{
					Key: "row-added",
					Val: id,
				})
			}

			to_check := old_set.Intersect(new_set) // items in both old and new that need to be compared
			for id := range to_check.Iter() {
				old_result := old_idx[id]
				new_result := new_idx[id]

				if reflect.DeepEqual(old_result, new_result) {
					slog.Debug("old and new vals are the same, no update")
					continue
				}

				// old and new vals are somehow different.
				// note! if row contains a function it will always be different.
				to_be_updated = append(to_be_updated, UIEvent{
					Key: "row-modified",
					Val: new_result.ID,
				})
			}
		}

		slog.Info("UIEventListener updates", "adding", len(to_be_added), "updating", len(to_be_updated), "deleting", len(to_be_deleted))

		if len(to_be_added) > 0 {
			ui.Put(to_be_added...)
		}
		if len(to_be_updated) > 0 {
			ui.Put(to_be_updated...)
		}
		if len(to_be_deleted) > 0 {
			ui.Put(to_be_deleted...)
		}
	}

	reducer := func(core.Result) bool {
		return true
	}
	return core.Listener{
		ID:         "ui-event-listener",
		ReducerFn:  reducer,
		CallbackFn: callback,
	}
}

// ---

// generic bridge for incoming events from app to a UI instance and it's methods
func Dispatch(ui_inst UI) {
	time.Sleep(250 * time.Millisecond) // er...why again?
	for {
		ev_grp := ui_inst.Get() // needs to block
		if len(ev_grp) == 0 {
			panic("programming error: empty event group")
			continue
		}
		ev := ev_grp[0]

		id_list := []string{}
		for _, uievent := range ev_grp {
			id_list = append(id_list, uievent.Val.(string))
		}

		slog.Debug("DISPATCH processing event", "event", ev.Key, "val", ev.Val)

		//switch ev.Key() {
		switch ev.Key {

		case "row-modified":
			// for time being, we can only update rows individually.
			// it's a gui constraint, consider changing if another UI can handle
			// blocks of updates
			for _, id := range id_list {
				ui_inst.UpdateRow(id)
			}

		case "row-added":
			ui_inst.AddRow(id_list...)

		case "row-deleted":
			// no ui implements delete at the moment
			for _, id := range id_list {
				ui_inst.DeleteRow(id)
			}

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
