package ui

import (
	"bw/internal/core"
	"log/slog"
	"sync"
)

/*
type UIEvent struct {
	NS  core.NS
	Key string
	Val any
}
*/

type UIEvent interface {
	NS() core.NS
	Key() string
	Val() any
}

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

type UI interface {
	Start() *sync.WaitGroup
	Stop()

	// the name of this UI.
	// if gui, the name of the window.
	SetTitle(string)

	// the UI is sent notifications to change it's state.
	// this fetches the oldest event on the queue.
	Get() UIEvent

	// the UI can send notifications of state changes.
	// notify whoever is listening that the UI has changed it's state
	Put(UIEvent)

	// tab handling
	GetTab(title string) UITab // finds something implementing a UITab using what it can from the UIEvent
	AddTab(title string) *sync.WaitGroup
	//RemoveTab(id string)
}

// ---

// generic bridge for incoming events from app to UI and it's methods
func Dispatch(ui_inst UI) {
	for {
		ev := ui_inst.Get() // needs to block
		switch ev.Key() {

		// convenience? creates a new tab (somehow) and adds it to the UI
		case "add-tab":
			// todo: check tab exists first?
			title, is_str := ev.Val().(string)
			if is_str {
				ui_inst.AddTab(title)
			}
		case "set-title":
			val, is_str := ev.Val().(string)
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
