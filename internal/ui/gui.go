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
	"reflect"
	"strconv"

	"github.com/visualfc/atk/tk"
)

const (
	key_details_pane_state = "bw.gui.details-pane"
	key_selected_results   = "bw.gui.selected-rows"
	key_expanded_rows      = "bw.gui.expanded_rows"
)

var NS_DUMMY_ROW = core.NewNS("bw", "ui", "dummyrow")

// ---

// a row in a View
type Row struct {
	row      map[string]string
	children []Row
}

// a column in a View
type Column struct {
	*tk.TablelistColumn
}

// a View describes how to render a list of core.Result structs.
// it is stateful and sits somewhere between the main app state and the internal Tablelist state.
// the GUI may have many Views over it's data, each one represented by a tab (notebook in tcl/tk).
type View struct {
	Columns []Column
	Rows    []Row
}

// ---

type GUIState struct {
	Views []View
}

type GUIWindow struct {
	*tk.Window
}

type GUIMenuItem struct {
	name string
	fn   func()
}

type GUIMenu struct {
	name  string
	items []GUIMenuItem
}

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
	current := app.KeyVal(key)
	opposite, err := OppositeVal(current)
	if err != nil {
		panic("programming error, key val not set or unsupported: " + err.Error())
	}
	app.SetKeyVal(key, opposite)
	return opposite
}

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

func build_menu(app *core.App, parent tk.Widget) *tk.Menu {
	menu_bar := tk.NewMenu(parent)
	menu_data := []GUIMenu{
		{
			name: "File",
			items: []GUIMenuItem{
				{name: "Open", fn: donothing},
				{name: "Exit", fn: tk.Quit},
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

func build_treeview_data(app *core.App, res_list []core.Result, col_list *[]string, col_set *map[string]bool) []Row {
	row_list := []Row{}

	for _, res := range res_list {
		if res.Item == nil { // dummy row, do not descend any further
			continue
		}

		row := Row{row: map[string]string{
			"id": res.ID,
			"ns": res.NS.String(),
		}}

		r := reflect.TypeOf((*core.TableItem)(nil)).Elem()
		if reflect.TypeOf(res.Item).Implements(r) {
			//println("implements")
			item_as_row := res.Item.(core.TableItem)

			// build up a list of known columns
			for _, col := range item_as_row.ItemKeys() {
				_, present := (*col_set)[col]
				if !present {
					(*col_list) = append((*col_list), col)
					(*col_set)[col] = true
				}
			}

			for key, val := range item_as_row.ItemMap() {
				row.row[key] = val
			}

			if item_as_row.ItemHasChildren() {
				if res.ChildrenRealised() {
					// children have already been visited already, insert them now
					//children := item_as_row.ItemChildren()
					children, _ := core.Children(app, &res)
					row.children = append(row.children, build_treeview_data(app, children, col_list, col_set)...)
				} else {
					// insert a dummy row indicating a row potentially has children.
					// these will be fetched and inserted when the expand button is clicked.
					row.children = append(row.children, build_treeview_data(app, dummy_row(), col_list, col_set)...)
				}
			}
		}

		row_list = append(row_list, row)
	}

	return row_list
}

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

func update_tablelist(app *core.App, result_list []core.Result, expanded_rows map[string]bool, tree *tk.Tablelist) {

	col_list := []string{"id", "ns"}
	col_set := map[string]bool{"id": true, "ns": true} // urgh
	row_list := build_treeview_data(app, result_list, &col_list, &col_set)

	// when the number of columns has changed, rebuild them.
	// todo: this is naive, there may be other changes that result in the same number of columns, but this is fine for now.
	old_col_len := tree.ColumnCount()
	if old_col_len != len(col_list) {

		slog.Debug("num cols changed, rebuilding", "old", old_col_len, "new", len(col_list))

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

	var insert_treeview_items func(int, []Row)
	insert_treeview_items = func(parent int, row_list []Row) {
		var parent_idx string
		if parent == -1 {
			// "root" is the invisible top-most element in a tree of items.
			// to insert items that appear to be top-level their parent must be 'root'.
			// to insert children of these top-level items, their parent must be 0.
			parent_idx = "root"
			parent = parent + 1 // -1 == 0, 0 == 1, 1 == 2 // this isn't tested.
		} else {
			parent_idx = strconv.Itoa(parent)
			parent += 1
		}
		for _, row := range row_list {
			vals := []string{}
			for _, col := range col_list {
				val, present := row.row[col]
				if !present {
					vals = append(vals, "")
				} else {
					vals = append(vals, val)
				}
			}
			cidx := 0 // todo: nfi. children-of-children?

			tli := tree.InsertChildListEx(parent_idx, cidx, [][]string{
				vals,
			})[0]

			result_id := vals[0]
			tli.SetName(result_id)

			if len(row.children) > 0 {
				insert_treeview_items(parent, row.children)

				_, is_expanded := expanded_rows[result_id]
				if !is_expanded {
					tli.Collapse()
				}

			}
		}
	}
	insert_treeview_items(-1, row_list)

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
	//tree.CollapseAll()

}

// ---

func tablelist_widj(app *core.App, parent tk.Widget) *tk.TablelistEx {

	app.SetKeyVal(key_expanded_rows, map[string]bool{}) // todo: this is global, not per-widj

	// the '-name' values of the selected rows
	app.SetKeyVal(key_selected_results, []string{}) // todo: this is global, not per-widj

	widj := tk.NewTablelistEx(parent)
	widj.LabelCommandSortByColumn()                       // column sort
	widj.LabelCommand2AddToSortColumns()                  // multi-column-sort
	widj.SetSelectMode(tk.TABLELIST_SELECT_MODE_EXTENDED) // click+drag to select
	widj.MovableColumns(true)                             // draggable columns
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

	// when the result list changes
	AddGuiListener(app, func(old_state core.State, new_state core.State) {
		old := old_state.Root.Item
		new := new_state.Root.Item
		if !reflect.DeepEqual(old, new) {
			expanded_rows := new_state.KeyAnyVal(key_expanded_rows).(map[string]bool)
			// just top-level rows
			new_root := new_state.Root.Item.([]core.Result)
			result_list := core.FilterResultList(new_root, func(r core.Result) bool {
				return r.Parent == nil
			})
			update_tablelist(app, result_list, expanded_rows, widj.Tablelist)
		}
	})

	// when a row is expanded
	widj.OnItemExpanded(func(tablelist_item *tk.TablelistItem) {
		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows := app.KeyAnyVal(key_expanded_rows).(map[string]bool)
		expanded_rows[key] = true
		app.SetKeyVal(key_expanded_rows, expanded_rows)

		// update app state, fetching the children of the result
		res := app.FindResultByID(key)
		if core.EmptyResult(res) {
			fmt.Println("no results found for key. cannot expand", key)
		} else {
			//fmt.Println("found res", res, "for key", key)
			core.Children(app, &res)
		}
	})

	// when a row is collapsed
	widj.OnItemCollapsed(func(tablelist_item *tk.TablelistItem) {
		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows := app.KeyAnyVal(key_expanded_rows).(map[string]bool)
		delete(expanded_rows, key)
		app.SetKeyVal(key_expanded_rows, expanded_rows)
	})

	// when rows are selected
	widj.OnSelectionChanged(func() {
		// fetch the associated 'name' attribute (result ID) of each selected row
		idx_list := widj.CurSelection2()
		selected_key_list := []string{}
		for _, idx := range idx_list {
			name := widj.RowCGet(idx, "-name")
			selected_key_list = append(selected_key_list, name)
		}

		// update app state, setting the list of selected ids
		app.SetKeyVal(key_selected_results, selected_key_list)
	})

	return widj
}

//

func details_widj(app *core.App, parent tk.Widget, pane *tk.Paned, tablelist *tk.Tablelist) *tk.GridLayout {
	app.SetKeyVal(key_details_pane_state, "opened")

	p := tk.NewGridLayout(parent)
	btn := tk.NewButton(parent, "toggle")
	p.AddWidget(btn)

	txt := tk.NewText(parent)
	txt.SetText("unset!")
	p.AddWidget(txt)

	selected_rows_changed := func(s core.State) any {
		return s.KeyAnyVal(key_selected_results)
	}
	AddGuiListener2(app, selected_rows_changed, func(new_key_val any) {
		if new_key_val == nil {
			txt.SetText("nil!")
			return
		}

		key_list := new_key_val.([]string)

		repr := ""
		for _, r := range app.FindResultByIDList(key_list) {
			repr += r.ID
		}

		txt.SetText(fmt.Sprintf("%v", repr))

	})

	btn.OnCommand(func() {
		ToggleKeyVal(app, key_details_pane_state)
	})

	details_pane_toggled := func(s core.State) any {
		return s.KeyVal(key_details_pane_state)
	}
	AddGuiListener2(app, details_pane_toggled, func(new any) {
		if new == "opened" {
			pane.SetPane(1, 25)
		} else {
			pane.SetPane(1, 0)
		}
	})

	return p
}

//

func NewWindow(app *core.App) *GUIWindow {
	//mw := tk.RootWindow()
	mw := &GUIWindow{tk.RootWindow()}
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(app, mw))

	/*
	    ___________ ______
	   |_|_|_|_|_|_|     x|
	   |           |      |
	   |  results  |detail|
	   |           |      |
	   |___________|______|

	*/
	paned := tk.NewPaned(mw, tk.Horizontal)

	// ---

	results_widj := tablelist_widj(app, mw)

	// ---

	d_widj := details_widj(app, mw, paned, results_widj.Tablelist)

	// ---

	paned.AddWidget(results_widj, 75)
	paned.AddWidget(d_widj, 25)

	tk.Pack(paned, layout_attr("expand", 1), layout_attr("fill", "both"))

	return mw
}

func StartGUI(app *core.App) {
	tk.Init() // could this be problematic? is idempotent? without it the root window gets destroyed on quit
	tk.SetErrorHandle(core.PanicOnErr)

	// tablelist: https://www.nemethi.de
	// ttkthemes: https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading

	slog.Info("tcl/tk", "tcl", tk.TclVersion(), "tk", tk.TkVersion())

	cwd, _ := os.Getwd()
	tk.SetAutoPath(filepath.Join(cwd, "tcl-tk"))
	_, err := tk.MainInterp().EvalAsStringList(`
# has no package
source tcl-tk/widgettree/widgettree.tcl

# $auto_path doesn't seem to work until searched

# tablelist/scaleutil is doing crazy fucking things
# like peering into running processes looking for and calling
# xfconf-query, gsettings, xrdb, xrandr etc.
# shortcircuit it's logic by giving it what it wants up front.
# we'll deal with it later.
set ::tk::scalingPct 100

package require Tablelist_tile 7.0`)

	core.PanicOnErr(err)

	// todo: set as bw preference
	// todo: limit available themes
	// todo: dark theme
	// todo: style main menu
	default_theme := "clearlooks"
	tk.TtkTheme.SetThemeId(default_theme)

	tk.MainLoop(func() {
		mw := NewWindow(app)
		mw.SetTitle(app.KeyVal("bw.app.name"))
		mw.Center(nil)
		mw.ShowNormal()

		// app is built, do an empty update to populate widgets
		app.KickState()
	})
}
