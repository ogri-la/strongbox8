package ui

import (
	"bw/internal/core"
	"fmt"

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
		menu{
			name: "File",
			items: []menuitem{
				{name: "Open", fn: donothing},
				{name: "Exit", fn: tk.Quit},
			},
		},
		menu{
			name:  "View",
			items: build_theme_menu(),
		},
		menu{
			name: "Preferences",
		},
		menu{
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

func tree_widj(app *core.App, parent tk.Widget) *tk.TreeView {

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

	tree.SetColumnCount(2)
	tree.SetHeaderLabel(0, "id")
	tree.SetHeaderLabel(1, "?")

	for i, res := range app.ResultList() {

		item := tree.InsertItem(nil, i, res.ID, []string{"foo"})
		tree.InsertItem(item, 0, res.ID+"(child)", []string{"bar"})
	}

	tk.Pack(tree, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

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
