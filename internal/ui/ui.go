package ui

type UIEvent struct {
	Key string
	Val any
}

func keyval(key string, val any) UIEvent {
	return UIEvent{Key: key, Val: val}
}

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

// a tab represents a single table of rows.
// a result is a row in a table.
type UITab interface {
	// the name of this tab.
	SetTitle(title string)

	// -- columns
	// all columns are always present,
	// but they can be hidden as necessary.
	HideColumn()
	ShowColumn()

	// -- rows
	AddRow()
	AddManyRows()
	UpdateRow()
	//UpdateManyRows()
	RemoveRow()
	//RemoveManyRows()
	//RemoveAllRows()

	// -- row selection
	// select a single row. selecting a row deselects all other rows.
	SelectRow()
	// select many rows, not necessarily continguous.
	//SelectManyRows()
	// if row is selected, the row is now unselected.
	DeselectRow()

	// -- detail pane
	// if a single item is selected, it is the detail for that.
	// if many items are selected, it is the detail for them.
	OpenDetail()
	CloseDetail()
}

type UI interface {
	Start()
	Stop()

	// the name of this UI.
	// if gui, the name of the window.
	SetTitle(title string)

	// the UI is sent notifications to change it's state.
	// this fetches the oldest event on the queue.
	Get() UIEvent

	// the UI can send notifications of state changes.
	// notify whoever is listening that the UI has changed it's state
	Put(event UIEvent)

	// tab handling
	AddTab()
	RemoveTab()
}
