package ui

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
	details_pane_state = "bw.gui.details-pane"
	selected_results   = "bw.gui.selected-rows"
)

type Window struct {
	*tk.Window
}

type menuitem struct {
	name string
	fn   func()
}

type menu struct {
	name  string
	items []menuitem
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

func build_theme_menu() []menuitem {
	theme_list := []menuitem{}
	for _, theme := range tk.TtkTheme.ThemeIdList() {
		if theme == "scid" {
			// something wrong with this one
			continue
		}
		theme := theme
		theme_list = append(theme_list, menuitem{name: theme, fn: func() {
			tk.TtkTheme.SetThemeId(theme)
		}})
	}
	return theme_list

}

func build_menu(app *core.App, parent tk.Widget) *tk.Menu {
	menu_bar := tk.NewMenu(parent)
	menu_data := []menu{
		{
			name: "File",
			items: []menuitem{
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
			items: []menuitem{
				{name: "Toggle", fn: func() {
					ToggleKeyVal(app, details_pane_state)
				}},
			},
		},
		{
			name: "Preferences",
		},
		{
			name: "Help",
			items: []menuitem{
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

type Row struct {
	row      map[string]string
	children []Row
}

func build_treeview_data(res_list []core.Result, col_list *[]string, col_set *map[string]bool) []Row {
	row_list := []Row{}

	for _, res := range res_list {
		row := Row{row: map[string]string{
			"id": res.ID,
			"ns": res.NS.String(),
		}}

		r := reflect.TypeOf((*core.TableRow)(nil)).Elem()
		if reflect.TypeOf(res.Item).Implements(r) {
			//println("implements")
			item_as_row := res.Item.(core.TableRow)

			// build up a list of known columns
			for _, col := range item_as_row.RowKeys() {
				_, present := (*col_set)[col]
				if !present {
					(*col_list) = append((*col_list), col)
					(*col_set)[col] = true
				}
			}

			for key, val := range item_as_row.RowMap() {
				row.row[key] = val
			}

			children := item_as_row.RowChildren()
			if len(children) > 0 {
				row.children = append(row.children, build_treeview_data(children, col_list, col_set)...)
			}
		}

		row_list = append(row_list, row)
	}

	return row_list
}

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

func update_tablelist(result_list []core.Result, tree *tk.Tablelist) {
	col_list := []string{"id", "ns"}
	col_set := map[string]bool{"id": true, "ns": true} // urgh
	row_list := build_treeview_data(result_list, &col_list, &col_set)

	tk_col_list := []tk.TablelistColumn{}
	auto_width := 0
	for _, col := range col_list {
		tk_col_list = append(tk_col_list, tk.TablelistColumn{Width: auto_width, Title: col, Align: "left"})
	}

	tree.DeleteAllItems()
	tree.DeleteAllColumns()
	tree.InsertColumnList(0, tk_col_list)

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
			cidx := 0 // todo: nfi
			tree.InsertChildList(parent_idx, cidx, [][]string{
				vals,
			})
			if len(row.children) > 0 {
				insert_treeview_items(parent, row.children)
			}
		}
	}
	insert_treeview_items(-1, row_list)

	// todo: any updates to the results collapses children again, assuming the result is still present.
	// this doesn't seem like a reasonable thing to do.
	tree.CollapseAll()
}

// ---

func tablelist_widj(app *core.App, parent tk.Widget) *tk.TablelistEx {

	widj := tk.NewTablelistEx(parent)
	widj.SetLabelCommandSortByColumn()             // column sort
	widj.SetLabelCommand2AddToSortColumns()        // multi-column-sort
	widj.SetSelectMode(tk.TablelistSelectExtended) // click+drag to select
	widj.MovableColumns(true)                      // draggable columns
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

	AddGuiListener(app, func(old_state core.State, new_state core.State) {
		old := old_state.Root.Item
		new := new_state.Root.Item
		if !reflect.DeepEqual(old, new) {
			update_tablelist(new_state.Root.Item.([]core.Result), widj.Tablelist)
		}
	})

	widj.OnSelectionChanged(func() {
		app.SetKeyVal(selected_results, widj.CurSelection())
	})

	return widj
}

//

func details_widj(app *core.App, parent tk.Widget, pane *tk.Paned) *tk.GridLayout {
	p := tk.NewGridLayout(parent)
	b := tk.NewButton(parent, "toggle")
	p.AddWidget(b)

	txt := tk.NewText(parent)
	txt.SetText("unset!")

	p.AddWidget(txt)

	AddGuiListener2(app, func(s core.State) any { return s.KeyAnyVal(selected_results) }, func(new any) {
		if new == nil {
			txt.SetText("nil!")
		} else {
			txt.SetText(fmt.Sprintf("%v", new))
		}
	})

	b.OnCommand(func() {
		ToggleKeyVal(app, details_pane_state)
	})
	return p
}

//

func NewWindow(app *core.App) *Window {
	//mw := tk.RootWindow()
	mw := &Window{tk.RootWindow()}
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

	d_widj := details_widj(app, mw, paned)

	// ---

	paned.AddWidget(results_widj, 75)
	paned.AddWidget(d_widj, 25)
	app.SetKeyVal(details_pane_state, "opened")

	AddGuiListener2(app, func(s core.State) any { return s.KeyVal(details_pane_state) }, func(new any) {
		if new == "opened" {
			paned.SetPane(1, 25)
		} else {
			paned.SetPane(1, 0)
		}
	})

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
