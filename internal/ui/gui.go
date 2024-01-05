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

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

func tree_widj(parent tk.Widget) *tk.TreeView {
	col_list := []string{"id"}
	tree := tk.NewTreeView(parent)
	tree.SetColumnCount(len(col_list))

	for i, col := range col_list {
		tree.SetHeaderLabel(i, col)
		tree.SetColumnWidth(i, 10) // this seems to pack the columns in for now
	}

	h_sb := tk.NewScrollBar(tree, tk.Horizontal)
	v_sb := tk.NewScrollBar(tree, tk.Vertical)

	core.PanicOnErr(tree.BindXScrollBar(h_sb))
	core.PanicOnErr(tree.BindYScrollBar(v_sb))

	tk.Pack(tree, layout_attr("side", "left"), layout_attr("expand", 1), layout_attr("fill", "both"))
	tk.Pack(v_sb, layout_attr("side", "right"), layout_attr("fill", "y"))
	tk.Pack(h_sb, layout_attr("side", "bottom"), layout_attr("fill", "x"))

	return tree
}

func update_treeview(result_list []core.Result, tree *tk.TreeView) {
	col_list := []string{"id"}
	col_set := map[string]bool{"id": true} // urgh
	row_list := build_treeview_data(result_list, &col_list, &col_set)

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
}

func NewWindow(app *core.App) *Window {
	//mw := tk.RootWindow()
	mw := &Window{tk.RootWindow()}
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(app, mw))

	//tk.Pack(mw, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

	vpack := tk.NewVPackLayout(mw)
	tree := tree_widj(mw)

	vpack.AddWidget(tree)

	tk.Pack(vpack, layout_attr("expand", 1), layout_attr("fill", "both"))

	app.AddListener(func(old_state core.State, new_state core.State) {
		new_result_list := new_state.Root.Item.([]core.Result)
		update_treeview(new_result_list, tree)
	})

	return mw
}

// executes functions on the main thread
func monitor_state(app *core.App) {
	for {
		msg := <-app.Messages
		tk.Async(msg)
	}
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
		go monitor_state(app)
	})
}
