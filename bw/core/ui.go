// ui.go
// UI-related types and general purpose logic.

package core

import (
	"log/slog"
	"reflect"

	mapset "github.com/deckarep/golang-set/v2"
)

// ---

type ResultDiff struct {
	Added    []string
	Modified []string
	Deleted  []string
}

func DiffResults(old_snapshot, new_snapshot *Snapshot) ResultDiff {
	diff := ResultDiff{}

	old_results := old_snapshot.Results()
	new_results := new_snapshot.Results()

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

	slog.Debug("DiffResults", "adding", len(diff.Added), "updating", len(diff.Modified), "deleting", len(diff.Deleted))

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
