package ui

// we need to capture collapsed/expanded state
// selection state (already done)
// essentially: gui state
// when expanded, update list of those expanded
// when collapsed, same

import (
	"bw/internal/core"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/visualfc/atk/tk"
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

// a column in a View

func NewViewFilter() core.ViewFilter {
	return func(core.Result) bool {
		return true
	}
}

// a View describes how to render a list of core.Result structs.
// it is stateful and sits somewhere between the main app state and the internal Tablelist state.
// the GUI may have many Views over it's data, each one represented by a tab (notebook in tcl/tk).
type View struct {
	Name        string
	ViewFilter  core.ViewFilter `json:"-"`
	DetailsOpen bool
	//SelectedRows []string
	//Columns    []Column
	//Rows       []Row
}

func NewView() *View {
	return &View{
		Name:       "untitled",
		ViewFilter: NewViewFilter(),
	}
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
	gui   *GUIUI
	ref   *tk.PackLayout
	id    string
	title string
}

/*
		func (tab *GUITab) SetTitle(title string) {
			// all well and good, but ...
			tab.title = title

			slog.Warn("gui tab SetTitle broken")

			// ... we kind of expect the gui to be updated when we do this ...
			// however the tab bar breaks and disappears if we switch away.
	                // update: probably because this needs to happen on the tk thread!
				tabber := tab.gui.mw.tabber
				tabber.SetTab(*tab.ref, title)

				// should work, harmless
				//tk.Pack(tab.gui.mw.tabber)
				//tk.Pack(tabber, layout_attr("fill", "both"), layout_attr("expand", "1"))

				slog.Info("tabber text", "txt", tab.gui.mw.tabber.Text(0))
		}
*/
func (tab GUITab) AddRow()      {}
func (tab GUITab) AddManyRows() {}
func (tab GUITab) UpdateRow()   {}

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

// recursive
// to do a quick bulk insert into the tablelist widjet we need to take the application results and
// chunk them into groups:
// - https://www.nemethi.de/tablelist/tablelistWidget.html#insertchildlist
// imagine a flat list of items, each item has an index, and a parent index
// the tablelist widget will ensure an item, even if it's in some crazy random position, will be nested under the correct parent.
// to keep things simple we'll do a depth-first
// `parentNodeIndex`: parent of the chunk of results to insert.
// - if this is a top level item the value is -1 ("root"), otherwise it's the index of the parent
// `childIndex` this is where in the list of the parent's children to insert the rows.
// - if the value is '0' the children will be inserted at the beginning
// - if the value is 'last' or equal to the number of children the parent already has, the chidlren be inserted at the end.
// TODO: to avoid recursion and the complexity of counting descendents, more can be done in the data prep to ensure this step
// happens in linear (O(n)) time
func _insert_treeview_items(tree *tk.Tablelist, parent int, cidx int, row_list []Row, col_list []string, row_idx map[string]string) int {

	if len(row_list) == 0 {
		panic("row list is empty")
	}

	var num_descendants int

	var parent_idx string
	if parent == -1 {
		// "root" is the invisible top-most element in the tree of items.
		// to insert items that appear to be top-level their parent must be 'root'.
		// to insert children of these top-level items, their parent must be 0.
		parent_idx = "root"

	} else {
		parent_idx = strconv.Itoa(parent) // 0 => "0"

	}

	// -1 => 0, 0 => 1
	parent += 1

	// we can only do bulk inserts of parents

	parent_list := [][]string{}
	for _, row := range row_list {
		// todo - shift this to _buildtreeview_data
		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}

		parent_list = append(parent_list, single_row)
	}

	//cidx := 0 // where in the list of children to insert this group.

	// insert the parents

	slog.Info("inserting children for parent", "parent_idx", parent_idx, "parent", parent, "pl", parent_list)
	full_key_list := tree.InsertChildList(parent_idx, cidx, parent_list)
	slog.Info("results of inserting children", "fkl", full_key_list)
	for idx := 0; idx < len(full_key_list); idx++ {
		row_id := row_list[idx].Row["id"]
		row_full_key := full_key_list[idx]
		row_idx[row_id] = row_full_key
		slog.Debug("adding full key to index", "key", row_id, "val", row_full_key, "val2", row_list[idx])
	}

	slog.Debug("done, inserting children of children")

	for idx, row := range row_list {
		_ = idx
		// recursive call to _insert_treeview_items will insert N children, messing with our internal pointer
		// we need to ensure that number is captured
		if len(row.Children) > 0 {
			slog.Info("child has children", "parent", parent, "num", len(row.Children))
			num_descendants += _insert_treeview_items(tree, parent+idx+num_descendants, cidx, row.Children, col_list, row_idx)

		} else {
			slog.Info("child has no children", "parent", parent)
		}
	}

	return num_descendants + len(row_list)

	//_insert_treeview_items(tree, parent, child_vals, col_list)

	/*
			/// ... we insert the row, get a widj back, and set the name of the widj.
			// I guess we want to find it later?
			tli := tree.InsertChildListEx(parent_idx, cidx, [][]string{
				vals,
			})[0]
			result_id := vals[0]
			tli.SetName(result_id)

			if len(row.children) > 0 {
				insert_treeview_items(parent, row.children)

				_, is_expanded := expanded_rows[result_id]
				if !is_expanded && !tli.IsCollapsed() {
					//slog.Debug("calling collapse.", "tli", tli.IsCollapsed(), "children", len(row.children))
					tli.Collapse()
				}
			}
		}
	*/

}

func _build_treeview_data(app *core.App, res_list []core.Result, col_list *[]string, col_set *map[string]bool, child_idx map[string][]core.Result) []Row {
	row_list := []Row{}

	for _, res := range res_list {
		if res.Item == nil { // dummy row, do not descend any further
			continue
		}

		row := Row{Row: map[string]string{
			"id": res.ID,
			"ns": res.NS.String(),
		}}

		// if the item has the ability to load children,
		// load them now.
		if core.HasItemInfo(res.Item) {
			item_as_row := res.Item.(core.ItemInfo)

			// build up a list of known columns
			for _, col := range item_as_row.ItemKeys() {
				_, present := (*col_set)[col]
				if !present {
					(*col_list) = append((*col_list), col)
					(*col_set)[col] = true
				}
			}

			for key, val := range item_as_row.ItemMap() {
				row.Row[key] = val
			}

			policy := item_as_row.ItemHasChildren()
			if policy != core.ITEM_CHILDREN_LOAD_FALSE {

				// policy is either 'load children' or 'lazy-load'

				// this is *not* the place to be modifying state (and causing a feedback loop).
				// an item with children has either been realised at this point or hasn't.
				// if it hasn't, it gets a dummy row that *will* trigger a state update.

				if res.ChildrenRealised {
					// children have already been visited.
					// stop thinking, ignore the policy and load them now.
					children, _ := core.Children(app, res)
					row.Children = append(row.Children, _build_treeview_data(app, children, col_list, col_set, child_idx)...)
				} else {
					if policy == core.ITEM_CHILDREN_LOAD_LAZY {
						// insert a dummy row indicating a row potentially has children.
						row.Children = append(row.Children, _build_treeview_data(app, dummy_row(), col_list, col_set, child_idx)...)
					} else {
						// insert the chilren
						children, _ := core.Children(app, res)
						row.Children = append(row.Children, _build_treeview_data(app, children, col_list, col_set, child_idx)...)
					}
				}
			}
		} else {
			// not a specialised Item, we might be able to handle some basic types
			switch t := res.Item.(type) {
			case map[string]string:
				for key, val := range t {
					row.Row[key] = val
					_, has_col := (*col_set)[key]
					if !has_col {
						(*col_list) = append((*col_list), key)
						(*col_set)[key] = true
					}
				}
			}
		}

		/*
			core_children, present := child_idx[row.Row["id"]]
			if present {
				row.Children = append(row.Children, _build_treeview_data(app, core_children, col_list, col_set, child_idx)...)
			}
		*/

		row_list = append(row_list, row)
	}

	return row_list
}

// convert `result_list` into a datastructure we can feed to the tablelist widjet.
// returns a list of converted results,
func build_treeview_data(app *core.App, result_list []core.Result) (row_list []Row, col_list []string, col_set map[string]bool) {
	col_list = []string{"id", "ns"}
	col_set = map[string]bool{"id": true, "ns": true} // urgh

	child_idx := map[string][]core.Result{} // foo1: [bar1,...]
	for _, r := range result_list {
		if r.Parent != nil {
			child_idx[r.Parent.ID] = append(child_idx[r.Parent.ID], r)
		}
	}

	//return _build_treeview_data(app, sub_result_list, &col_list, &col_set, child_idx), col_list, col_set
	return _build_treeview_data(app, result_list, &col_list, &col_set, child_idx), col_list, col_set
}

// similar to `build_treeview_data`, but for a single row.
// does not consider children, does not recurse
func build_treeview_row(res_list []core.Result, col_list []string, col_set map[string]bool) ([]Row, []string, map[string]bool) {

	col_set["id"] = true
	col_set["ns"] = true
	if len(col_list) == 0 {
		col_list = []string{"id", "ns"}
	}

	row_list := []Row{}
	for _, res := range res_list {
		if res.Item == nil { // dummy row, do not descend any further
			continue
		}

		row := Row{Row: map[string]string{
			"id": res.ID,
			"ns": res.NS.String(),
		}}

		if core.HasItemInfo(res.Item) {
			item := res.Item.(core.ItemInfo)

			// build up a list of known columns
			for _, col := range item.ItemKeys() {
				_, present := col_set[col]
				if !present {
					col_list = append(col_list, col)
				}
				col_set[col] = true
			}

			for key, val := range item.ItemMap() {
				row.Row[key] = val
			}

		} else {
			// not a specialised Item, we might be able to handle some basic types
			switch t := res.Item.(type) {
			case map[string]string:
				for key, val := range t {
					row.Row[key] = val
					_, has_col := col_set[key]
					if !has_col {
						col_list = append(col_list, key)
					}
					col_set[key] = true
				}
			default:
				slog.Warn("basic fields (id, ns) for unsupported type", "type", t, "data", res)
			}
		}

		row_list = append(row_list, row)
	}

	return row_list, col_list, col_set
}

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

func set_tablelist_cols(col_list []string, tree *tk.Tablelist) {
	known_cols := map[string]bool{}
	for _, title := range tree.ColumnNames(tree.ColumnCount()) {
		known_cols[title] = true
	}

	slog.Info("knowns cols", "cols", known_cols)

	tk_col_list := []*tk.TablelistColumn{}
	for _, col_title := range col_list {
		_, present := known_cols[col_title]
		if present {
			slog.Debug("columns exists, skipping", "col", col_title)
			continue
		}

		col := tk.NewTablelistColumn()
		col.Title = col_title
		tk_col_list = append(tk_col_list, col)
	}

	if len(tk_col_list) > 0 {
		tree.InsertColumns("end", tk_col_list)
	}
}

// replaces the contents of the given tablelist widget with `row_list`
func replace_tablelist_widj(row_list []Row, col_list []string, expanded_rows map[string]bool, tree *tk.Tablelist) {

	panic("unused")

	slog.Info("updating tablelist with results", "results", row_list)

	//row_list, col_list, _ := build_treeview_data(app, result_list)

	// when the number of columns has changed, rebuild them.
	// todo: this check is naive as other changes may have resulted in the same column count, but this is fine for now.
	old_col_len := tree.ColumnCount()
	if old_col_len != len(col_list) {

		slog.Warn("num cols changed, rebuilding", "old", old_col_len, "new", len(col_list))

		tk_col_list := []*tk.TablelistColumn{}
		for _, col_title := range col_list {
			col := tk.NewTablelistColumn()
			col.Title = col_title
			tk_col_list = append(tk_col_list, col)
		}
		tree.DeleteAllColumns()
		tree.InsertColumnsEx(0, tk_col_list)
	}

	tree.DeleteAllItems()

	/*
		row_list = []Row{
			{
				Row: map[string]string{"id": "foo1"},
				Children: []Row{
					{
						Row: map[string]string{"id": "bar1"},
						Children: []Row{
							{
								Row: map[string]string{"id": "baz1"},
							},
							{
								Row: map[string]string{"id": "baz2"},
								Children: []Row{
									{
										Row: map[string]string{"id": "bup1"},
									},
								},
							},
						},
					},
					{
						Row: map[string]string{"id": "bar2"},
					},
					{
						Row: map[string]string{"id": "bar3"},
					},
				},
			},
			{
				Row: map[string]string{"id": "foo2"},
				Children: []Row{
					{
						Row: map[string]string{"id": "bar4"},
					},
				},
			},
		}
	*/

	top_level := 0
	row_idx := map[string]string{}
	_insert_treeview_items(tree, -1, top_level, row_list, col_list, row_idx)

	/*
		tree.OnItemCollapsed(func(e *tk.Event) {
			println("8888888888888888888")
			fmt.Println(e.UserData)
		})

		tree.OnItemPopulate(func(e *tk.Event) {
			println("ppppppppppppppppp")
			fmt.Println(e.UserData)
		})
	*/

	// todo: any updates to the results collapses children again, assuming the result is still present.
	// this doesn't seem like a reasonable thing to do.
	tree.CollapseAll()

}

// ---

func tablelist_widj(gui *GUIUI, parent tk.Widget, view core.ViewFilter) *tk.TablelistEx {

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

		slog.Debug("selection changed")

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

func AddTab(gui *GUIUI, title string, view core.ViewFilter) { //app *core.App, mw *Window, title string) {

	/*
	    ___________ ______
	   |tab|_|_|_|_|     x|
	   |           |      |
	   |  results  |detail|
	   |   view    |      |
	   |___________|______|

	*/

	paned := tk.NewTKPaned(gui.mw, tk.Horizontal)

	results_widj := tablelist_widj(gui, gui.mw, view)
	gui.widget_ref[results_widj.Id()] = results_widj

	d_widj := details_widj(gui.app, gui.mw, paned, view, results_widj.Tablelist)

	paned.AddWidget(results_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"stretch", "always"})
	paned.AddWidget(d_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"width", "50p"})

	//paned.HidePane(1, !view.DetailsOpen)

	// ---

	tab_body := tk.NewVPackLayout(gui.mw)
	tab_body.AddWidgetEx(paned, tk.FillBoth, true, 0)

	gui.mw.tabber.AddTab(tab_body, title)

	gui.tab_idx[title] = tab_body.Id()

	//tk.Pack(paned, layout_attr("expand", 1), layout_attr("fill", "both"))

}

var lock sync.Mutex

// when a row is ADDED, it is because the row doesn't exist to be modified in-place.
// as such, any new columns must be added and
// the row must find all of it's parents and children and
// the row must be inserted in the right place.
func (gui *GUIUI) AddRow(id string) {

	gui.TkSync(func() {
		slog.Info("GUI, adding row", "id", id)

		app_row := gui.app.GetResult(id)

		if app_row == nil {
			slog.Error("row with id does not exist", "id", id)
			panic("row with that id does not exist")
			return
		}

		var tree tk.TablelistEx
		for _, widj := range gui.widget_ref {
			if gui.widget_ref == nil {
				panic("tablelist not initialised yet")
			}
			tree = *widj.(*tk.TablelistEx)
			break
		}

		// if this row has a parent, then the parent's index must be found
		// and this row is added as a new child of that row.
		// if the parent can't be found, the child cannot be added.
		parent_idx := -1
		if app_row.Parent == nil {
			slog.Debug("row has no parent, it will be added to the top level")
		} else {
			slog.Info("row has parent, looking", "parent-id", app_row.Parent.ID)

			// we need to find the parent's index, ignoring the index it may presently be at.
			// every row inserted has the full key of the `gui.Row.Row["id"]` value.

			if len(gui.row_idx) == 0 {
				// nothing has been inserted yet!
				slog.Error("looking for parent of row to be inserted inside an empty table", "row", app_row, "parent", app_row.Parent)
				panic("")
			}

			fkey, present := gui.row_idx[app_row.Parent.ID]
			if !present {
				slog.Error("parent not found in map, cannot continue", "item", app_row, "parent", app_row.Parent.ID, "odx", gui.row_idx)
				panic("")
			}

			// get index of row with id - see indices:
			// - https://www.nemethi.de/tablelist/tablelistWidget.html#row_indices
			// - https://www.nemethi.de/tablelist/tablelistWidget.html#index
			parent_idx, err := tree.Index(fkey)

			if err != nil {
				slog.Error("failed to find full key in tablelist", "full-key", fkey)
				panic("")
			}

			slog.Info("parent index is", "idx", parent_idx)
		}

		// ---

		col_idx := map[string]bool{}
		col_list := []string{}

		for _, title := range tree.ColumnNames(tree.ColumnCount()) {
			col_idx[title] = true
			col_list = append(col_list, title)
		}

		row_list, col_list, col_idx := build_treeview_row([]core.Result{*app_row}, col_list, col_idx)

		// todo: doesn't this take into account existing columns??
		set_tablelist_cols(col_list, tree.Tablelist)

		slog.Debug("row list", "rl", row_list, "cols", col_list, "idx", col_idx)

		// ----

		//gui.row_list = append(gui.row_list, a...)
		//expanded_row := map[string]bool{}

		child_idx := 0 // where in list of children to add this child (if is child)
		_insert_treeview_items(tree.Tablelist, parent_idx, child_idx, row_list, col_list, gui.row_idx)

		//tablelist_widj.CollapseAll()

		//gui.row_list = append(gui.row_list, row_list...)

		// we may have N tablewidgets
		// each widget is using a processed list of core.Result called a gui.Row
		// these gui.Rows are nested in a way core.Results are not.
		// each widget needs to be updated
		// for each widget
		// ... assume gui.Row does not exist. this is AddRow after all
		// ... convert result to gui.Row
		// ... find gui.Row in gui.row_list
		// ... update value in place
	})

}

// add a function to be executed on the UI thread and processed _synchronously_
func (gui *GUIUI) TkSync(fn func()) {
	gui.tk_chan <- fn
}

func (gui *GUIUI) UpdateRow(id string) {
	gui.TkSync(func() {
		slog.Info("gui.UpdateRow UPDATING ROW", "id", id)
		if len(gui.row_idx) == 0 {
			slog.Error("gui failed to update row, gui has no rows to update yet", "id", id)
			panic("")
			return
		}

		fkey, present := gui.row_idx[id]
		if !present {
			slog.Error("gui failed to update row, row full key not found in row index", "id", id)
			panic("")
			return
		}

		app_row := gui.app.GetResult(id)
		if app_row == nil {
			slog.Error("gui failed to update row, result with id does not exist", "id", id)
			panic("")
			return
		}

		var tree tk.TablelistEx
		for _, widj := range gui.widget_ref {
			tree = *widj.(*tk.TablelistEx)
			break
		}

		// when a row is updated, just the row is updated, the children are not modified.
		// can new columns be introduced? not right now.
		// further, rows returned must match the current column ordering

		col_idx := map[string]bool{}
		col_list := []string{}

		for _, title := range tree.ColumnNames(tree.ColumnCount()) {
			col_idx[title] = true
			col_list = append(col_list, title)
		}
		set_tablelist_cols(col_list, tree.Tablelist)

		row_list, col_list, _ := build_treeview_row([]core.Result{*app_row}, col_list, col_idx)
		if len(row_list) != 1 {
			panic("row list should be precisely 1")
		}
		row := row_list[0]

		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}

		tree.Tablelist.RowConfigureText(fkey, single_row)

	})
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
	row_list []Row             // a conversion of []core.Result => []gui.Row
	row_idx  map[string]string // a mapping of gui.Row.ID => tablelist 'full key'
	col_list []string          // a list of column names
	//col_set  map[string]bool // an index of column names

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

func (gui *GUIUI) Get() UIEvent {
	slog.Debug("gui.GET called, fetching UI event from app", "implemented", true)
	return <-gui.inc
}

func (gui *GUIUI) Put(event UIEvent) {
	slog.Info("gui.PUT called, adding UI event from app", "event", event, "implemented", true)
	gui.inc <- event
}

func (gui *GUIUI) GetTab(title string) UITab {
	return &GUITab{}
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
		tk.SetAutoPath(filepath.Join(cwd, "tcl-tk"))
		_, err := tk.MainInterp().EvalAsStringList(`
# has no package
#source tcl-tk/widgettree/widgettree.tcl # disabled 2024-09-15: 'invalid command name "console"'

# $auto_path doesn't seem to work until searched

# tablelist/scaleutil is doing crazy fucking things
# like peering into running processes looking for and calling
# xfconf-query, gsettings, xrdb, xrandr etc.
# shortcircuit it's logic by giving it what it wants up front.
# we'll deal with it later.
set ::tk::scalingPct 100

package require Tablelist_tile 7.0`)
		core.PanicOnErr(err)

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
	app.AddResults(core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows))

	row_list := []Row{}

	return &GUIUI{
		row_list:   row_list,
		row_idx:    make(map[string]string),
		tab_idx:    make(map[string]string),
		widget_ref: make(map[string]any),
		inc:        make(chan UIEvent),
		out:        make(chan UIEvent),
		tk_chan:    make(chan func()),
		wg:         wg,
		app:        app,
	}
}
