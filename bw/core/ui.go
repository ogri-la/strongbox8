// ui.go
// abstract definition of a user interface + some general purpose logic.
// `UI` has two implementations: ui.CLIUI and ui.GUIUI

package core

import (
	"log/slog"
	"reflect"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
)

// ---

// todo: rename UIColumn for consistency
type UIColumn struct {
	Title       string // what to show as this column's name
	HiddenTitle bool   // is column's name displayed?
	Hidden      bool   // is column hidden?
	// halign
	// resizable
	MaxWidth int
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
// a table row is a Result
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

	SetColumnAttrs([]UIColumn)
}

// what a UI should be able to do
type UI interface {
	// the UI holds a reference to the app instance it belongs to (and not the other way around)
	App() *App

	Start() *sync.WaitGroup
	Stop()

	SetTitle(string)

	// tab handling
	GetTab(title string) UITab
	AddTab(title string, view ViewFilter)

	AddRow(id ...string)
	UpdateRow(id string)
	DeleteRow(id string)
}

// ---

type ResultDiff struct {
	Added    []string
	Modified []string
	Deleted  []string
}

func DiffResults(old_results, new_results []Result) ResultDiff {
	diff := ResultDiff{}

	if len(old_results) == 0 {
		for _, result := range new_results {
			diff.Added = append(diff.Added, result.ID)
		}
		return diff
	}

	old_idx := map[string]Result{}
	for _, result := range old_results {
		old_idx[result.ID] = result
	}

	new_idx := map[string]Result{}
	for _, result := range new_results {
		new_idx[result.ID] = result
	}

	old_set := mapset.NewSetFromMapKeys(old_idx)
	new_set := mapset.NewSetFromMapKeys(new_idx)

	for id := range old_set.Difference(new_set).Iter() {
		diff.Deleted = append(diff.Deleted, id)
	}

	for id := range new_set.Difference(old_set).Iter() {
		diff.Added = append(diff.Added, id)
	}

	for id := range old_set.Intersect(new_set).Iter() {
		if !reflect.DeepEqual(old_idx[id], new_idx[id]) {
			diff.Modified = append(diff.Modified, id)
		}
	}

	slog.Info("DiffResults", "adding", len(diff.Added), "updating", len(diff.Modified), "deleting", len(diff.Deleted))

	return diff
}

// --- generic menu wrangling

var MENU_SEP = MenuItem{Name: "sep"}

// a clickable menu entry of a `Menu`
type MenuItem struct {
	Name string
	//Accelerator ...
	Fn func(*App)
	//Parent MenuItem
	ServiceID string // id of the service to call. takes precedence over Fn
}

// a top-level menu item, like 'File' or 'View'.
type Menu struct {
	Name string
	//Accelerator ...
	MenuItemList []MenuItem
}

// append-merges the contents of `b` into `a`
func MergeMenus(a []Menu, b []Menu) []Menu {
	a_idx := map[string]*Menu{}
	for i := range a {
		a_idx[a[i].Name] = &a[i]
	}

	for _, mb := range b {
		ma, present := a_idx[mb.Name]
		if present {
			// menu b exists in menu a,
			// append the items from menu b to the end of the items in menu a
			ma.MenuItemList = append(ma.MenuItemList, mb.MenuItemList...)
		} else {
			// menu b does not exist in menu a
			// append the menu as-is and update the index
			a = append(a, mb)
			a_idx[mb.Name] = &mb
		}
	}
	return a
}
