// UI-related types and general purpose logic.

package core

import (
	"log/slog"
	"reflect"
)

// ---

type ResultDiff struct {
	Added    []string
	Modified []string
	Deleted  []string
}

// compares two snapshots by result ID and returns what was added, modified and deleted.
// every result is reported as added when the old snapshot is empty.
// IDs are reported in snapshot order, not set order: the UI inserts rows in this
// order and siblings must keep the order their parent produced them in.
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

	for _, result := range old_results {
		_, present := new_idx[result.ID]
		if !present {
			diff.Deleted = append(diff.Deleted, result.ID)
		}
	}

	for _, result := range new_results {
		old_result, present := old_idx[result.ID]
		if !present {
			diff.Added = append(diff.Added, result.ID)
		} else if !reflect.DeepEqual(old_result, result) {
			diff.Modified = append(diff.Modified, result.ID)
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

// merges the menus in `b` into `a` and returns the result.
// menus are matched by name: the items of a menu in `b` are appended to the items of
// the menu in `a` with the same name, otherwise the menu is appended whole.
// `a` may be modified.
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
