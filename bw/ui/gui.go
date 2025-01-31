package ui

// we need to capture collapsed/expanded state
// selection state (already done)
// essentially: gui state
// when expanded, update list of those expanded
// when collapsed, same

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/visualfc/atk/tk"

	mapset "github.com/deckarep/golang-set/v2"
)

const (
	key_gui_state          = "bw.ui.gui"
	key_details_pane_state = "bw.gui.details-pane"
	key_selected_results   = "bw.gui.selected-rows"
	key_expanded_rows      = "bw.gui.expanded_rows"
)

var NS_KEYVAL = core.NewNS("bw", "ui", "keyval")
var NS_VIEW = core.NewNS("bw", "ui", "view")
var NS_DUMMY_ROW = core.NewNS("bw", "ui", "dummyrow")

// ---

// a row to be inserted into a Tablelist
type Row struct {
	Row      map[string]string `json:"row"`
	Children []Row             `json:"children"`
}

// ---

type GUIMenuItem struct {
	name string
	fn   func()
}

type GUIMenu struct {
	name  string
	items []GUIMenuItem
}

type Window struct {
	*tk.Window
	tabber *tk.Notebook
}

// ---

type GUITab struct {
	gui         *GUIUI
	tab_body    *tk.PackLayout
	tab_body_id string
	table_widj  *tk.TablelistEx
	title       string
	filter      func(core.Result) bool
	column_list []Column          // available columns and their properties for this tab
	row_idx     map[string]string // a mapping of gui.Row.ID => tablelist 'full key'

	IgnoreMissingParents bool // results with a parent that are missing get a parent_id of '-1' (top-level)
}

func (tab *GUITab) SetTitle(title string) {
	tab.gui.TkSync(func() {
		tab.title = title
		tab.gui.mw.tabber.SetTab(tab.tab_body, title)
	})
}

// basic columns are created as rows are added to the table.
// each tab may specify it's own set of columns, each with their own attributes.
// tablelist columns not declared are created.
// tablelist columns present but not declared are hidden.
// tablelist column order inconsistent with declared are re-ordered.
func (tab *GUITab) SetColumnAttrs(column_list []Column) {
	tab.gui.TkSync(func() {
		tab.column_list = column_list

		// first, find all columns to hide.
		// these are columns that are not present in the new idx.
		//current_col_idx := mapset.NewSetFromMapKeys(tab.gui.col_idx) // map[string]bool => set[string]
		current_col_idx := mapset.NewSet[string]()
		for _, col := range tab.column_list {
			current_col_idx.Add(col.Title)
		}

		new_col_idx := mapset.NewSet[string]()
		for _, col := range column_list {
			new_col_idx.Add(col.Title)
		}

		difference := current_col_idx.Difference(new_col_idx)

		// we now need to find the indicies of each of these columns to hide.
		// some of these columns may not exist yet!
		to_be_hidden := []int{}
		for pos, col := range tab.column_list {
			if difference.Contains(col.Title) {
				to_be_hidden = append(to_be_hidden, pos)
			}
		}

		tab.table_widj.ToggleColumnHide2(to_be_hidden)

		// next, find all columns to add.
		// ...

		// next, order the columns.
		// to be implemented: https://www.nemethi.de/tablelist/tablelistWidget.html#movecolumn
		new_col_pos_idx := map[string]int{}
		for i, col := range column_list {
			i := i
			new_col_pos_idx[col.Title] = i
		}
		for old_pos, old_col := range tab.column_list {
			new_pos, present := new_col_pos_idx[old_col.Title]
			if !present {
				// problem here? if cols are not present, `old_pos`
				slog.Warn("skipping col, not present in new", "col", old_col, "col-pos", old_pos)
				continue
			}
			slog.Info("moving col", "col", old_col, "col-pos", old_pos, "new-pos", new_pos)
			if new_pos != old_pos {
				tab.table_widj.MoveColumn(old_pos, new_pos)
			}
		}

		tab.table_widj.TreeColumn(0)
	})
}

var _ UITab = (*GUITab)(nil)

// ---

func dummy_row() []core.Result {
	return []core.Result{core.NewResult(NS_DUMMY_ROW, "", fmt.Sprintf("dummy-%v", core.UniqueID()))}
}

func OppositeVal(val string) (string, error) {
	switch val {
	case "true":
		return "false", nil
	case "false":
		return "true", nil
	case "open":
		return "close", nil
	case "close":
		return "open", nil
	case "opened":
		return "closed", nil
	case "closed":
		return "opened", nil
	case "show":
		return "hide", nil
	case "hide":
		return "show", nil
	}
	return "", errors.New("unsupported value: " + val)
}

func ToggleKeyVal(app *core.App, key string) string {
	slog.Debug("toggling keyval", "key", key)
	current := app.KeyVal(key)
	opposite, err := OppositeVal(current)
	if err != nil {
		panic("programming error, key val not set or unsupported: " + err.Error())
	}
	app.SetKeyVal(key, opposite)
	return opposite
}

/*
// just like `app.AddListener`,
// but wraps given `callback` function in `tk.Async` so it executes on main thread.
func AddGuiListener(app *core.App, callback func(old_state, new_state core.State)) {
	wrapped_fn := func(os, ns core.State) {
		tk.Async(func() {
			callback(os, ns)
		})
	}
	app.AddListener(wrapped_fn)
}
*/

func AddGuiListener(app *core.App, listener core.Listener2) {
	original_callback := listener.CallbackFn
	listener.CallbackFn = func(old_results, new_results []core.Result) {
		tk.Async(func() {
			original_callback(old_results, new_results)
		})
	}
	app.AddListener(listener)
}

/*
// convenience. function `lookup` extracts a value from state,
// and if this value is different between new and old states,
// calls function `callback` with the new value
// on the main thread.
func AddGuiListener2(app *core.App, lookup func(new_state core.State) any, callback func(someval any)) {
	wrapped_fn := func(os, ns core.State) {
		old := lookup(os)
		new := lookup(ns)
		if !reflect.DeepEqual(old, new) {
			callback(new)
		}
	}
	AddGuiListener(app, wrapped_fn)
}
*/

func donothing() {}

func build_theme_menu() []GUIMenuItem {
	theme_list := []GUIMenuItem{}
	for _, theme := range tk.TtkTheme.ThemeIdList() {
		if theme == "scid" {
			// something wrong with this one
			continue
		}
		theme := theme
		theme_list = append(theme_list, GUIMenuItem{name: theme, fn: func() {
			tk.TtkTheme.SetThemeId(theme)
		}})
	}
	return theme_list

}

func build_menu(gui *GUIUI, parent tk.Widget) *tk.Menu {
	app := gui.app
	menu_bar := tk.NewMenu(parent)
	menu_data := []GUIMenu{
		{
			name: "File",
			items: []GUIMenuItem{
				{name: "Open", fn: donothing},
				{name: "Exit", fn: gui.Stop},
			},
		},
		{
			name:  "View",
			items: build_theme_menu(),
		},
		{
			name: "Details",
			items: []GUIMenuItem{
				{name: "Toggle", fn: func() {
					ToggleKeyVal(app, key_details_pane_state)
				}},
			},
		},
		{
			name: "Preferences",
		},
		{
			name: "Help",
			items: []GUIMenuItem{
				{name: "Debug", fn: func() { fmt.Println(tk.MainInterp().EvalAsStringList(`wtree::wtree`)) }},
				{name: "About", fn: func() {
					title := "bw"
					heading := app.KeyVal("bw.app.name")
					version := app.KeyVal("bw.app.version")
					message := fmt.Sprintf(`version: %s
https://github.com/ogri-la/strongbox
AGPL v3`, version)
					tk.MessageBox(parent, title, heading, message, "ok", tk.MessageBoxIconInfo, tk.MessageBoxTypeOk)
				}},
			},
		},
	}

	for _, toplevel_item := range menu_data {
		submenu := menu_bar.AddNewSubMenu(toplevel_item.name)
		submenu.SetTearoff(false)
		for _, submenu_item := range toplevel_item.items {
			submenu_item_action := tk.NewAction(submenu_item.name)
			submenu_item_action.OnCommand(submenu_item.fn)
			submenu.AddAction(submenu_item_action)
		}
	}

	return menu_bar
}

// https://www.nemethi.de/tablelist/tablelistWidget.html#insertchildlist
// `parentNodeIndex`: parent of the chunk of results to insert.
// - if this is a top level item the value is -1 ("root"), otherwise it's the index of the parent
// `childIndex` this is where in the list of the parent's children to insert the rows.
// - if the value is '0' the children will be inserted at the beginning
// - if the value is 'last' or equal to the number of children the parent already has, the chidlren be inserted at the end.
func _insert_treeview_items(tree *tk.Tablelist, parent string, cidx int, row_list []Row, col_list []Column, row_idx map[string]string) int {

	if len(row_list) == 0 {
		panic("row list is empty")
	}

	if parent == "" {
		panic("parent_id is empty")
	}

	var parent_idx string
	if parent == "-1" {
		// "root" is the invisible top-most element in the tree of items.
		// to insert items that appear to be top-level their parent must be 'root'.
		// to insert children of these top-level items, their parent must be 0.
		parent_idx = "root"

	} else {
		parent_idx = parent
	}

	parent_list := [][]string{}
	for _, row := range row_list {
		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col.Title]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}
		parent_list = append(parent_list, single_row)
	}

	// insert the parents

	slog.Info("inserting rows", "num", len(parent_list), "parent", parent, "parent-fk", parent_idx)

	full_key_list := tree.InsertChildList(parent_idx, cidx, parent_list)
	slog.Info("results of inserting children", "fkl", full_key_list)

	for idx := 0; idx < len(full_key_list); idx++ {
		row_id := row_list[idx].Row["id"]
		row_full_key := full_key_list[idx]
		row_idx[row_id] = row_full_key
		slog.Debug("adding full key to index", "key", row_id, "val", row_full_key, "val2", row_list[idx])
	}

	return len(row_list)
}

// creates a list of rows and columns from the given `result_list`.
// does not consider children, does not recurse.
func build_treeview_row(result_list []core.Result, col_list []Column) ([]Row, []Column) {

	// if a list of columns is passed in,
	// only those columns will be supported.
	// otherwise, all columns will be supported.
	fixed := len(col_list) > 0

	if !fixed {
		col_list = []Column{
			{Title: "id"},
			{Title: "ns"},
		}
	}

	col_idx := mapset.NewSet[string]()
	for _, col := range col_list {
		col_idx.Add(col.Title)
	}

	row_list := []Row{}
	for _, result := range result_list {
		if result.Item == nil { // dummy row/unrealised row, skip
			continue
		}

		row := Row{Row: map[string]string{
			"id": result.ID,
			"ns": result.NS.String(),
		}}

		if core.HasItemInfo(result.Item) {
			item := result.Item.(core.ItemInfo)

			if !fixed {
				// append any missing columns
				for _, col := range item.ItemKeys() {
					if !col_idx.Contains(col) {
						col_list = append(col_list, Column{Title: col})
						col_idx.Add(col)
					}
				}
			}

			// build up the row
			for col, val := range item.ItemMap() {
				if col_idx.Contains(col) {
					row.Row[col] = val
				}
			}
		}
		row_list = append(row_list, row)
	}

	return row_list, col_list
}

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

// add each column in `col_list`,
// unless column exists.
func set_tablelist_cols(col_list []Column, tree *tk.Tablelist) {
	known_cols := map[string]bool{}
	for _, title := range tree.ColumnNames(tree.ColumnCount()) {
		known_cols[title] = true
	}

	tk_col_list := []*tk.TablelistColumn{}
	for _, col := range col_list {
		_, present := known_cols[col.Title]
		if present {
			slog.Debug("column exists, skipping", "col", col.Title)
			continue
		}

		tk_col := tk.NewTablelistColumn()
		tk_col.Title = col.Title
		tk_col_list = append(tk_col_list, tk_col)
	}

	if len(tk_col_list) > 0 {
		tree.InsertColumns("end", tk_col_list)
	}
}

// ---

func tablelist_widj(gui *GUIUI, parent tk.Widget) *tk.TablelistEx {

	//app.SetKeyVal(key_expanded_rows, map[string]bool{}) // todo: this is global, not per-widj
	//app.AddResults(core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows))

	// the '-name' values of the selected rows
	//app.SetKeyVal(key_selected_results, []string{}) // todo: this is global, not per-widj

	app := gui.app

	widj := tk.NewTablelistEx(parent)
	widj.LabelCommandSortByColumn()                       // column sort
	widj.LabelCommand2AddToSortColumns()                  // multi-column-sort
	widj.SetSelectMode(tk.TABLELIST_SELECT_MODE_EXTENDED) // click+drag to select
	widj.MovableColumns(true)                             // draggable columns

	// ---

	/*
		var expanded_rows map[string]bool
		expanded_rows_result := app.GetResult(key_expanded_rows)
		if expanded_rows_result != nil {
			expanded_rows = expanded_rows_result.Item.(map[string]bool)
		}
	*/

	// just top-level rows,
	// as children are included as necessary.
	/*
		   new_results := app.GetResultList()

			result_list := core.FilterResultList(new_results, func(r core.Result) bool {
				// "&& view(r)" - this turned out to accidentally be what I'm after.
				// we want to display just top-level items and just those that match the view filter,
				// but we also want those results to yield children of whatever type
				return r.Parent == nil && view(r)
			})
	*/

	//fmt.Println(core.QuickJSON(gui.row_list))

	//update_tablelist_widj(gui.row_list, gui.col_list, expanded_rows, widj.Tablelist)

	// ---

	/*
		tl.SetColumns([]tk.TablelistColumn{
			{Title: "foo"},
			{Title: "bar"},
		})

		// inserts two top-level items.
		// there are now two items in item list.
		tl.InsertChildList("root", 0, [][]string{
			{"boop", "baap"},
			{"baaa", "dddd"},
		})
		// inserts two more items under parent '0' (first item in list)
		// there are now four items in item list (0-3), items 1 and 2 are children.
		tl.InsertChildList(0, 0, [][]string{
			{"foo", "bar"},
			{"baz", "bop"},
		})

		// inserts an item under parent '1' (second item in list, which is a child of row 0)
		tl.InsertChildList(1, 0, [][]string{
			{"aaa", "aaa("},
		})
	*/

	// "when the core result list changes."
	/*
		AddGuiListener(app, core.Listener2{
			ID:        "changed rows listener",
			ReducerFn: view.ViewFilter,
			CallbackFn: func(new_results []core.Result) {

				// changes to state must include the keyvals.
				// should the listener instead be attached to State rather than State.Results or State.KeyVals?
				// - how does the reducer fn work then?
				// - should we ditch keyvals?

				expanded_rows := app.GetResult(key_expanded_rows).Item.(map[string]bool)

				// just top-level rows,
				// as children are included as necessary.
				result_list := core.FilterResultList(new_results, func(r core.Result) bool {
					return r.Parent == nil
				})

				app.AtomicUpdates(func() {
					update_tablelist(app, result_list, expanded_rows, widj.Tablelist)
				})

			},
		})
	*/
	// when a row is expanded
	widj.OnItemExpanded(func(tablelist_item *tk.TablelistItem) {

		slog.Warn("item expanded")

		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows := app.GetResult(key_expanded_rows)
		if expanded_rows == nil {
			slog.Error("the 'expanded rows' item has not been set, cannot record expansion")
			return
		}
		expanded_rows.Item.(map[string]bool)[key] = true
		app.SetResults(*expanded_rows)

		// update app state, fetching the children of the result
		res := app.FindResultByID(key)
		if core.EmptyResult(res) {
			slog.Debug("could not expand item, no results found", "item-key", key)
			return
		}
		//slog.Info("calling children ...")
		//core.Children(app, res) // todo (I suppose): this is blocking for some reason
		//slog.Info("done calling children")
	})

	// when a row is collapsed
	// disabled. this is called whenever children are realised.
	// using keyvals it didn't act so nutty (right?), but now it's really expensive
	// as an individual collapse event is sent for each
	// should we have a SetResultNoUpdate? SetResultBlind?

	widj.OnItemCollapsed(func(tablelist_item *tk.TablelistItem) {

		slog.Debug("item collapsed")

		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows_result := app.GetResult(key_expanded_rows)
		if expanded_rows_result == nil {
			return
		}
		expanded_rows := expanded_rows_result.Item.(map[string]bool)
		delete(expanded_rows, key)
		expanded_rows_result.Item = expanded_rows
		//app.SetResults(*expanded_rows_result)
	})

	// when rows are selected
	widj.OnSelectionChanged(func() {
		/*
			// fetch the associated 'name' attribute (result ID) of each selected row
			idx_list := widj.CurSelection2()
			selected_key_list := []string{}
			for _, idx := range idx_list {
				name := widj.RowCGet(idx, "-name")
				selected_key_list = append(selected_key_list, name)
			}

			// update the list of selected rows
			selected := core.NewResult(NS_KEYVAL, selected_key_list, key_selected_results)

			// show/hide the details widget
			rl := app.FilterResultList(func(r core.Result) bool {
				v, is_view := r.Item.(View)
				if is_view {
					fmt.Println("found view with name", v.Name)
				}
				return is_view && v.Name == view.Name
			})
			if len(rl) > 1 {
				panic(fmt.Sprintf("expected a single view, got %v", len(rl)))
			}
			r := rl[0]
			v := r.Item.(View)
			v.DetailsOpen = len(selected_key_list) > 0
			r.Item = v

			app.SetResults(selected, r)
		*/
		/*
			gui_state := app.KeyAnyVal(key_gui_state).(GUIState)
			for i, v := range gui_state.Views {
				if v.Name == view.Name {
					v.SelectedRows = selected_key_list
					v.DetailsOpen = len(selected_key_list) > 0
					gui_state.Views[i] = v
				} else {
					gui_state.Views[i] = v
				}
			}


			// update app state, setting the list of selected ids
			//app.SetKeyVal(key_selected_results, selected_key_list)
			app.SetKeyVal(key_gui_state, gui_state)
		*/

	})

	return widj
}

//

// todo: these accumulating parameters suggests a coupling problem
func details_widj(app *core.App, parent tk.Widget, pane *tk.TKPaned, view core.ViewFilter, tablelist *tk.Tablelist) *tk.PackLayout {
	//app.SetKeyVal(key_details_pane_state, "opened")

	p := tk.NewPackLayout(parent, tk.SideTop)

	btn := tk.NewButton(parent, "close")
	p.AddWidget(btn)

	txt := tk.NewText(parent)
	txt.SetText("")
	p.AddWidget(txt)

	/*
		selected_rows_changed := func(s core.State) any {
			return s.KeyAnyVal(key_selected_results)
		}
		AddGuiListener2(app, selected_rows_changed, func(new_key_val any) {
			if new_key_val == nil {
				txt.SetText("")
				return
			}

			key_list := new_key_val.([]string)

			repr := ""
			for _, r := range app.FindResultByIDList(key_list) {
				repr += r.ID
			}

			txt.SetText(fmt.Sprintf("%v", repr))
		})
	*/

	// when 'close' button is clicked, toggle the 'details open' state of this view to the opposite of what it was.
	btn.OnCommand(func() {
		/*
			// todo: use app.GetResult somehow
			rl := app.FilterResultList(func(r core.Result) bool {
				v, is_view := r.Item.(View)
				return is_view && v.Name == view.Name
			})
			if len(rl) > 1 {
				panic(fmt.Sprintf("expected a single view, got %v", len(rl)))
			}

			r := rl[0]
			v := r.Item.(View)
			v.DetailsOpen = !v.DetailsOpen
			r.Item = v

			app.SetResults(r)
		*/
	})

	/*
		// on-state-change, find our view and return it's current 'details open' state
		details_pane_toggled := func(s core.State) any {
			gui_state := s.KeyAnyVal(key_gui_state).(*GUIState)
			for _, v := range gui_state.Views {
				if v.Name == view.Name {
					fmt.Println("found this view", v.Name, "open", v.DetailsOpen)
					return v.DetailsOpen
				}
			}
			return nil
		}

		AddGuiListener2(app, details_pane_toggled, func(val any) {
			hide, is_bool := val.(bool)
			if is_bool {
				pane.HidePane(1, hide)
			}
		})
	*/

	/*
		AddGuiListener(app, core.Listener2{
			ID: "view details listener",
			ReducerFn: func(s core.Result) bool {
				v, is_view := s.Item.(View)
				return is_view && v.Name == view.Name
			},
			CallbackFn: func(new_results []core.Result) {
				if len(new_results) != 1 {
					// view has been deleted?
					// future: delete listeners, or
					// create listeners in such a way that they are not tied to widgets.
					// will lead to fewer, more complex listeners.
					return
				}
				new_view := new_results[0].Item.(View)
				if new_view.DetailsOpen {
					fmt.Println("hiding")
				} else {
					fmt.Println("not hiding")
				}
				//pane.HidePane(1, !new_view.DetailsOpen)
			},
		})
	*/
	return p
}

func AddTab(gui *GUIUI, title string, viewfn core.ViewFilter) { //app *core.App, mw *Window, title string) {

	/*
	    ___________ ______
	   |tab|_|_|_|_|     x|
	   |           |      |
	   |  results  |detail|
	   |   view    |      |
	   |___________|______|

	*/

	paned := tk.NewTKPaned(gui.mw, tk.Horizontal)

	table_widj := tablelist_widj(gui, gui.mw)

	table_id := table_widj.Id()
	gui.widget_ref[table_id] = table_widj

	//d_widj := details_widj(gui.app, gui.mw, paned, viewfn, table_widj.Tablelist)

	paned.AddWidget(table_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"stretch", "always"})
	//paned.AddWidget(d_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"width", "50p"})

	//paned.HidePane(1, !view.DetailsOpen)

	// ---

	tab_body := tk.NewVPackLayout(gui.mw)
	tab_body.AddWidgetEx(paned, tk.FillBoth, true, 0)

	gui.mw.tabber.AddTab(tab_body, title)

	//tk.Pack(paned, layout_attr("expand", 1), layout_attr("fill", "both"))

	gt := &GUITab{
		gui:         gui,
		tab_body:    tab_body,
		tab_body_id: tab_body.Id(),
		table_widj:  table_widj,
		title:       title,
		filter:      viewfn,
		row_idx:     make(map[string]string),
	}
	gui.TabList = append(gui.TabList, gt)
	gui.tab_idx[title] = tab_body.Id()
}

func AddRowToTree(gui *GUIUI, tab *GUITab, id_list ...string) {

	gui.TkSync(func() {

		tree := tab.table_widj

		// id_list is in insertion order.
		// however! the list needs to be grouped by parent_id
		// and inserted in batches
		// as the next batch may depend on the ID of a result inserted in the previous batch.

		// top-level results (no parent ID)
		// that fail the tab's view filter fn,
		// are excluded from being displayed,
		// including their children (obviously)
		excluded := map[string]bool{}

		result_list := []core.Result{}
		for _, id := range id_list {
			result := gui.app.GetResult(id)
			if result == nil {
				slog.Error("GUI, result with id not found in app", "id", id)
				panic("")
			}

			if result.ID != id {
				slog.Error("GUI, result with id != given id", "id", id, "result.ID", result.ID)
				panic("")
			}

			if !tab.filter(*result) {
				excluded[result.ID] = true
				continue
			}

			result_list = append(result_list, *result)
		}

		no_parent := "-1"
		bunch_list := core.Bunch(result_list, func(r core.Result) any {
			return r.ParentID
		})

		// figure out which parent to insert each bunch of results under
		for _, bunch := range bunch_list {
			first_row := bunch[0]
			var parent_id string
			var present bool
			if first_row.ParentID == "" {
				// easy, no parent to begin with,
				// use top-level.
				parent_id = no_parent
			} else {
				// has a parent, but
				// parent may have been excluded during filtering of results above,
				// or we may have a code error.

				// if parent has been filtered out,
				is_excluded := excluded[first_row.ParentID] // warning: bool default value is being used here for rows not found
				if is_excluded {
					// parent has been excluded.
					// this means all children (this bunch) are also excluded,
					// unless IgnoreMissingParents is true.
					if tab.IgnoreMissingParents {
						slog.Debug("parent has been excluded and this bunch of results will become top-level")
					} else {
						slog.Debug("parent has been excluded so this bunch of results will also be excluded")
						continue
					}
				}

				parent_id, present = tab.row_idx[first_row.ParentID]
				if !present {
					// parent not found!
					// this is to be expected if we're excluding results, but
					// unless IgnoreMissingParents is explicitly set to true,
					// this is a programming error.
					if tab.IgnoreMissingParents {
						// all good, just set parent to the top level.
						// note: using IgnoreMissingParents may mask programming problems.
						parent_id = no_parent
					} else {
						// no good. parent not found and IgnoreMissingParents is false.
						// programming or logic problem. die.
						msg := "parent not found in index. it hasn't been inserted yet or has been excluded without IgnoreMissingParents set to 'true'"
						slog.Warn(msg, "id", first_row.ID, "parent", first_row.ParentID, "idx", tab.row_idx, "num-exclusions", len(excluded), "ignore-missing-parents", tab.IgnoreMissingParents)
						panic("")
					}
				}
			}

			// ---

			row_list, col_list := build_treeview_row(bunch, tab.column_list)

			// todo: col_idx and col_list should be updated in-place.
			tab.column_list = col_list

			set_tablelist_cols(col_list, tree.Tablelist)

			child_idx := 0 // where in list of children to add this child (if is child)
			_insert_treeview_items(tree.Tablelist, parent_id, child_idx, row_list, col_list, tab.row_idx)
		}

		tree.CollapseAll()
	})

}

// when a row is ADDED, it is because the row doesn't exist to be modified in-place.
// as such, any new columns must be added and
// the row must find all of it's parents and children and
// the row must be inserted in the right place.
func (gui *GUIUI) AddRow(id_list ...string) {
	for _, tab := range gui.TabList {
		AddRowToTree(gui, tab, id_list...)
	}
}

// add a function to be executed on the UI thread and processed _synchronously_
func (gui *GUIUI) TkSync(fn func()) {
	gui.tk_chan <- fn
}

func UpdateRowInTree(gui *GUIUI, tab *GUITab, id string) {
	gui.TkSync(func() {
		slog.Info("gui.UpdateRow UPDATING ROW", "id", id)
		if len(tab.row_idx) == 0 {
			slog.Error("gui failed to update row, gui has no rows to update yet", "id", id)
			panic("")
		}

		fkey, present := tab.row_idx[id]
		if !present {
			slog.Error("gui failed to update row, row full key not found in row index", "id", id)
			panic("")
		}

		app_row := gui.app.GetResult(id)
		if app_row == nil {
			slog.Error("gui failed to update row, result with id does not exist", "id", id)
			panic("")
		}

		tree := tab.table_widj

		// when a row is updated, just the row is updated, the children are not modified.
		// can new columns be introduced? not right now.
		// further, rows returned must match the current column ordering
		set_tablelist_cols(tab.column_list, tree.Tablelist)

		row_list, col_list := build_treeview_row([]core.Result{*app_row}, tab.column_list)

		// todo: update many rows at once?
		if len(row_list) != 1 {
			slog.Error("gui failed to update row, result of building row should be precisely 1", "row-list", row_list)
			panic("")
		}
		row := row_list[0]

		// because updating rows can't introduce new columns,
		// add padding if a value for that column doesn't exist.
		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col.Title]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}

		tree.Tablelist.RowConfigureText(fkey, single_row)
	})
}

func (gui *GUIUI) UpdateRow(id string) {
	for _, tab := range gui.TabList {
		AddRowToTree(gui, tab, id)
	}
}

func (gui *GUIUI) DeleteRow(id string) {
	app_row := gui.app.GetResult(id)
	slog.Info("gui DeleteRow", "row", app_row, "implemented", false)
}

//

func NewWindow(gui *GUIUI) *Window {
	//app := gui.app

	mw := &Window{}
	mw.Window = tk.RootWindow()
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(gui, mw))
	mw.tabber = tk.NewNotebook(mw)

	vbox := tk.NewVPackLayout(mw)
	vbox.AddWidgetEx(mw.tabber, tk.FillBoth, true, 0)

	/*
		app.AddListener(core.Listener2{
			ID: "view listener",
			ReducerFn: func(r core.Result) bool {
				_, is_view := r.Item.(View)
				return is_view
			},
			CallbackFn: func(rl []core.Result) {

				// this listener is concerned about:
				// adding view tabs
				// destroy view tabs
				// moving view tabs
				// ...
				// it doesn't care about the internal state of the View itself,
				// that should be handled in some other listener.

				old_views := map[string]bool{}
				num_tabs := mw.tabber.TabCount()
				for i := 0; i < num_tabs; i++ {
					old_views[mw.tabber.Text(i)] = true
				}

				// future: the below doesn't preserve tab order.
				// future: it is possible to move the position of tabs without recreating them.
				// future: the below doesn't destroy tabs

				for _, r := range rl {
					view := r.Item.(View)
					_, is_present := old_views[view.Name]
					if !is_present {
						AddViewTab(app, mw, view)
					}
				}
			},
		})
	*/

	return mw
}

type GUIUI struct {
	TabList    []*GUITab
	tab_idx    map[string]string
	widget_ref map[string]any

	inc     UIEventChan // events coming from the core app
	out     UIEventChan // events going to the core app
	tk_chan chan func() // functions to be executed on the tk channel

	wg  *sync.WaitGroup
	app *core.App
	mw  *Window // intended to be the gui 'root', from where we can reach all gui elements
}

var _ UI = (*GUIUI)(nil)

func (gui *GUIUI) SetTitle(title string) {
}

func (gui *GUIUI) Get() []UIEvent {
	val := <-gui.inc
	slog.Debug("gui.GET called, fetching UI event from app", "val", val)
	return val
}

func (gui *GUIUI) Put(event ...UIEvent) {
	slog.Debug("gui.PUT called, adding UI event from app", "event", event, "implemented", true)
	gui.inc <- event
}

func (gui *GUIUI) GetTab(title string) UITab {
	for _, tab := range gui.TabList {
		if title == tab.title {
			return tab
		}
	}
	return nil
}

func (gui *GUIUI) AddTab(title string, filter core.ViewFilter) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	tk.Async(func() {
		AddTab(gui, title, filter)
		wg.Done()
	})
	return &wg
}

func (gui *GUIUI) SetActiveTab(title string) *sync.WaitGroup {
	slog.Info("setting active tab", "title", title)
	var wg sync.WaitGroup
	tab_id, exists := gui.tab_idx[title]
	if !exists {
		slog.Error("tab not found in index, cannot set active tab", "title", title)
		return &wg
	}
	widj, exists := tk.LookupWidget(tab_id)
	if !exists {
		slog.Error("widget with id not found. cannot set active tab", "title", title, "id", tab_id)
	}
	wg.Add(1)
	tk.Async(func() {
		gui.mw.tabber.SetCurrentTab(widj)
		wg.Done()
	})
	return &wg
}

func (gui *GUIUI) Stop() {
	gui.wg.Done()
	tk.Quit()
}

func (gui *GUIUI) Start() *sync.WaitGroup {
	var init sync.WaitGroup
	init.Add(1)

	app := gui.app

	// listen for events from the app and tie them to UI methods
	go Dispatch(gui)

	/*
		default_view := NewView()
		default_view.Name = "all"
		default_view.ViewFilter = func(r core.Result) bool {
			// everything
			//return true
			return r.NS != NS_KEYVAL && r.NS != NS_VIEW
		}
	*/
	//gui_state.AddView(*default_view)
	//app.SetKeyVal(key_gui_state, gui_state)

	/*
		default_view_item := core.NewResult(NS_VIEW, *default_view, core.PrefixedUniqueId("view-"))
		expanded_rows_item := core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows)
		app.SetResults(default_view_item, expanded_rows_item)
	*/
	/*
		vendor_view := NewView()
		vendor_view.Name = "vendor"
		vendor_view.ViewFilter = func(r core.Result) bool {
			// everything that isn't bw.*.*
			return r.NS.Major != "bw"
		}
		gui_state.AddView(vendor_view)
	*/

	// --- tcl/tk init

	go func() {

		tk.Init()
		tk.SetErrorHandle(core.PanicOnErr)

		// tablelist: https://www.nemethi.de
		// ttkthemes: https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
		slog.Info("tcl/tk", "tcl", tk.TclVersion(), "tk", tk.TkVersion())

		// --- configure path
		// todo: fix environment so this isn't necessary

		cwd, _ := os.Getwd()

		// prepend a directory to the TCL `auto_path`,
		// where custom tcl/tk code can be loaded.
		tk.SetAutoPath(filepath.Join(cwd, "../tcl-tk"))

		_, err := tk.MainInterp().EvalAsString(`
# has no package
#source tcl-tk/widgettree/widgettree.tcl # disabled 2024-09-15: 'invalid command name "console"'

# $auto_path doesn't seem to work until searched

# tablelist/scaleutil is doing crazy fucking things
# like peering into running processes looking for and calling
# xfconf-query, gsettings, xrdb, xrandr etc.
# shortcircuit it's logic by giving it what it wants up front.
# we'll deal with it later.
set ::tk::scalingPct 100`)
		core.PanicOnErr(err)

		_, err = tk.MainInterp().EvalAsString(`
package require Tablelist_tile 7.0`)
		core.PanicOnErr(err) // "panic: error: NULL main window" happens here

		// --- configure theme
		// todo: set as bw preference
		// todo: limit available themes
		// todo: dark theme
		// todo: main menu seems to resist styling

		default_theme := "clearlooks"
		tk.TtkTheme.SetThemeId(default_theme)

		// ---

		tk.MainLoop(func() {

			mw := NewWindow(gui)
			gui.mw = mw

			mw.SetTitle(app.KeyVal("bw.app.name"))
			mw.Center(nil)
			mw.ShowNormal()
			mw.OnClose(func() bool {
				gui.Stop()
				return true
			})

			//slog.Warn("building TREEVIEW")
			//row_list, col_list, _ := build_treeview_data(app, app.GetResultList())

			//slog.Warn("DONE building TREEVIEW")

			//gui.row_list = row_list
			//gui.col_list = col_list

			init.Done() // the GUI isn't 'done', but we're done with init and ready to go.

			// listen for events from the app and tie them to UI methods
			//go Dispatch(gui)

			go func() {
				var wg sync.WaitGroup
				for {
					tk_fn := <-gui.tk_chan
					slog.Warn("--- TkSync block OPEN")
					wg.Add(1) // !important to be outside async
					tk.Async(func() {
						defer wg.Done()
						tk_fn()
					})
					wg.Wait()
					slog.Warn("--- TkSync block CLOSED")
				}
			}()

			// execute functions that need to be synchronous on the main loop

			// populate widgets
			//app.KickState()          // an empty update
			//app.RealiseAllChildren() // an update that realises all non-lazy children
		})

	}()

	return &init
}

func GUI(app *core.App, wg *sync.WaitGroup) *GUIUI {
	wg.Add(1)

	// this is where results will store whether they have been 'expanded' or not.
	// todo: move into core. lazily evaluated rows are a core feature, not limited to gui.
	// todo: race condition here.will sometimes trigger a gui update despite gui not being initialised
	//app.AddResults(core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows))

	return &GUIUI{
		tab_idx: make(map[string]string),

		widget_ref: make(map[string]any),
		inc:        make(chan []UIEvent),
		out:        make(chan []UIEvent),
		tk_chan:    make(chan func()),
		wg:         wg,
		app:        app,
	}
}
