package ui

import (
	"bw/internal/core"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

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
	tree.SetColumns(tk_col_list)

	var insert_treeview_items func(*int, []Row)
	insert_treeview_items = func(parent *int, row_list []Row) {
		var parent_idx string
		if parent == nil {
			parent_idx = "root"
		} else {
			parent_idx = strconv.Itoa(*parent)
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

			//tree.InsertChildren(parent_idx, 0, [][]string{
			tree.InsertChildren(parent_idx, 0, [][]string{
				vals,
			})
			//insert_treeview_items(parent, i, parent.Children)
			/*
				tree.InsertChildList(0, 0, [][]string{

				})
			*/
			//tree.InsertChildList(parent, 0, row.children

		}
	}
	insert_treeview_items(nil, row_list)
}

// ---

func tablelist_widj(parent tk.Widget) *tk.Tablelist {

	tl := tk.NewTablelist(parent)
	tl.SetLabelCommandSortByColumn()             // column sort
	tl.SetLabelCommand2AddToSortColumns()        // multi-column-sort
	tl.SetSelectMode(tk.TablelistSelectExtended) // click+drag to select
	tl.MovableColumns(true)                      // draggable columns

	/*
		tl.SetColumns([]tk.TablelistColumn{
			{Title: "foo"},
			{Title: "bar"},
		})

		tl.InsertChildren(0, 0, [][]string{
			{"boop", "baap"},
			{"baaa", "dddd"},
		})
		tl.InsertChildList(0, 0, [][]string{
			{"foo", "bar"},
			{"baz", "bop"},
		})
	*/
	h_sb := tk.NewScrollBar(tl, tk.Horizontal)
	v_sb := tk.NewScrollBar(tl, tk.Vertical) // todo: this is a bit off. not stretching vertically

	core.PanicOnErr(tl.BindXScrollBar(h_sb))
	core.PanicOnErr(tl.BindYScrollBar(v_sb))

	tk.Pack(tl, layout_attr("side", "left"), layout_attr("expand", 1), layout_attr("fill", "both"))
	tk.Pack(v_sb, layout_attr("side", "right"), layout_attr("fill", "y"))
	tk.Pack(h_sb, layout_attr("side", "bottom"), layout_attr("fill", "x"))

	return tl
}

func NewWindow(app *core.App) *Window {
	//mw := tk.RootWindow()
	mw := &Window{tk.RootWindow()}
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(app, mw))

	vpack := tk.NewVPackLayout(mw)

	// this all works
	//tree := tree_widj(mw)
	//vpack.AddWidget(tree)
	/*
		app.AddListener(func(old_state core.State, new_state core.State) {
			new_result_list := new_state.Root.Item.([]core.Result)
			tk.Async(func() {
				update_treeview(new_result_list, tree)
			})
		})
	*/

	tablelist := tablelist_widj(mw)

	app.AddListener(func(old_state core.State, new_state core.State) {
		new_result_list := new_state.Root.Item.([]core.Result)
		tk.Async(func() {
			update_tablelist(new_result_list, tablelist)
		})
	})

	vpack.AddWidget(tablelist)

	tk.Pack(vpack, layout_attr("expand", 1), layout_attr("fill", "both"))

	return mw
}

func StartGUI(app *core.App) {
	tk.Init() // could this be problematic? is idempotent? without it the root window gets destroyed on quit
	tk.SetErrorHandle(func(err error) {
		core.PanicOnErr(err)
	})

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
		mw.SetTitle(app.KeyVal("bw", "app", "name"))
		mw.Center(nil)
		mw.ShowNormal()

		// app is built, do an empty update to populate widgets
		app.KickState()
	})
}
