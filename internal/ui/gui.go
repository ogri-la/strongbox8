package ui

import (
	"bw/internal/core"
	"fmt"
	"reflect"

	"log/slog"

	"github.com/visualfc/atk/tk"
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

func donothing() {}

func build_theme_menu() []menuitem {
	theme_list := []menuitem{}
	for _, theme := range tk.TtkTheme.ThemeIdList() {
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
			name: "Preferences",
		},
		{
			name: "Help",
			items: []menuitem{
				{name: "Debug", fn: func() { fmt.Println(tk.MainInterp().EvalAsStringList(`wtree::wtree`)) }},
				{name: "About", fn: func() {
					title := "bw"
					heading := app.KeyVal("bw", "app", "name")
					version := app.KeyVal("bw", "app", "version")
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
	id       string
	row      map[string]string
	children []Row
}

func build_treeview_data(res_list []core.Result, col_list *[]string, col_set *map[string]bool) []Row {
	row_list := []Row{}

	for _, res := range res_list {
		row := Row{id: res.ID, row: map[string]string{}}

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

		} else {
			//println("does not implement")
		}

		row_list = append(row_list, row)
	}

	return row_list

}

func tree_widj(app *core.App, parent tk.Widget) *tk.TreeView {
	col_list := []string{"id"}
	col_set := map[string]bool{"id": true} // urgh
	row_list := build_treeview_data(app.ResultList(), &col_list, &col_set)

	// ---

	tree := tk.NewTreeView(parent)

	// figure out the bounds of the result set.
	// for each result, test if $somemethod exists
	// - if so, call that to figure out fields and labels, otherwise:

	// for each result, determine the cells.
	// - if struct, each field becomes a cell.
	// - if list, each index becomes a cell
	// - if primative, value is a single cell
	// for each cell, determine a cell label
	// - if struct, it's the field name
	// - if list, its a counter
	// - if primative, it's 0

	// for each result, test if $somemethod exists to determine children.
	// or, if a Result's item is a list of Results, then the first Item is the parent and the rest are children?
	// - if so, call that then recursively do the above.

	// the result should be a superset of all possible fields to display

	tree.SetColumnCount(len(col_list))

	for i, col := range col_list {
		tree.SetHeaderLabel(i, col)
		tree.SetColumnWidth(i, 10) // this seems pack the columns in for now
	}

	var insert_treeview_items func(*tk.TreeItem, []Row)
	insert_treeview_items = func(parent *tk.TreeItem, row_list []Row) {
		for i, row := range row_list {
			vals := []string{}
			for _, col := range col_list {
				val, present := row.row[col]
				if !present {
					vals = append(vals, "")
				} else {
					vals = append(vals, val)
				}
			}
			item := tree.InsertItem(parent, i, row.id, vals[1:])
			insert_treeview_items(item, row.children)
		}
	}
	insert_treeview_items(nil, row_list)

	h_sb := tk.NewScrollBar(tree, tk.Horizontal)
	v_sb := tk.NewScrollBar(tree, tk.Vertical)

	core.PanicOnErr(tree.BindXScrollBar(h_sb))
	core.PanicOnErr(tree.BindYScrollBar(v_sb))

	tk.Pack(tree, &tk.LayoutAttr{"side", "left"}, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})
	tk.Pack(v_sb, &tk.LayoutAttr{"side", "right"}, &tk.LayoutAttr{"fill", "y"})  //, &tk.LayoutAttr{"expand", 1})
	tk.Pack(h_sb, &tk.LayoutAttr{"side", "bottom"}, &tk.LayoutAttr{"fill", "x"}) //, &tk.LayoutAttr{"expand", 1})

	return tree
}

func NewWindow(app *core.App) *Window {
	//mw := tk.RootWindow()
	mw := &Window{tk.RootWindow()}
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(app, mw))

	//tk.Pack(mw, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

	vpack := tk.NewVPackLayout(mw)
	vpack.AddWidget(tree_widj(app, mw))

	tk.Pack(vpack, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

	return mw
}

func StartGUI(app *core.App) {
	tk.Init() // could this be problematic? is idempotent? without it the root window gets destroyed on quit

	// https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
	fmt.Println(tk.MainInterp().EvalAsStringList(`
source widgettree/widgettree.tcl

set dir bwidget
source bwidget/pkgIndex.tcl

set dir ttkfile/fsdialog
source ttkfile/fsdialog/pkgIndex.tcl
package require fsdialog
source ttkthemes/ttkthemes/themes/pkgIndex.tcl
source ttkthemes/ttkthemes/png/pkgIndex.tcl

`))
	// todo: set as bw preference
	// todo: limit available themes
	// todo: dark theme
	// todo: style main menu
	default_theme := "clearlooks"
	err := tk.TtkTheme.SetThemeId(default_theme)
	if err != nil {
		slog.Warn("failed to set default theme", "default-theme", default_theme, "error", err)
		panic("programming error")
	}
	tk.MainLoop(func() {
		mw := NewWindow(app)
		mw.SetTitle(app.KeyVal("bw", "app", "name"))
		mw.Center(nil)
		mw.ShowNormal()
	})
}
