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

func NewWindow(app *core.App) *Window {

	mw := &Window{tk.RootWindow()}

	mm := tk.NewMenu(mw)

	//fm := tk.NewMenu(mw)
	fm := mm.AddNewSubMenu("File")
	fm.SetTearoff(false)

	importcmd := tk.NewAction("Import")
	importcmd.OnCommand(func() {
		//tk.GetOpenFile(mw, "Open", []tk.FileType{}, "", "")
		fmt.Println(tk.MainInterp().EvalAsStringList(`tk_getOpenFile`))
	})

	fm.AddAction(importcmd)

	//fm := tk.NewAction("File")

	//mm.AddAction(fm)
	mw.SetMenu(mm)

	//btn.OnCommand(func() {
	//tk.Quit()
	//})

	theme_widj := tk.NewComboBox(mw, &tk.WidgetAttr{"state", "readonly"})
	theme_widj.SetValues(tk.TtkTheme.ThemeIdList())
	theme_widj.SetCurrentText(tk.TtkTheme.ThemeId())
	theme_widj.OnSelected(func() {
		tk.TtkTheme.SetThemeId(theme_widj.CurrentText())
	})

	tabber := tk.NewNotebook(mw, tk.NotebookAttrWidth(20), tk.NotebookAttrHeight(20), tk.NotebookAttrTakeFocus(true))

	mf := tk.NewFrame(mw)

	mf2 := tk.NewFrame(mw)

	// 'fill=both' seems to extend the widget horizontally
	// - Stretch the content both horizontally and vertically
	// 'expand=1' seems to center the widget horizontally and vertically
	// - "Specifies whether the content should be expanded to consume extra space in their container."
	//tk.Pack(tabber, &tk.LayoutAttr{"fill", "both"}, &tk.LayoutAttr{"expand", 1})

	tabber.AddTab(mf, "main")

	//disabled := tk.WidgetAttr{"state", "disabled"}

	tabber.AddTab(mf2, "main 2") //, &disabled)
	//defer tabber.Destroy()

	tree := tk.NewTreeView(mf)

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

	// ---

	tk.Pack(tabber, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

	vpack2 := tk.NewVPackLayout(mf)
	vpack2.AddWidget(tree)

	// ---

	vpack := tk.NewVPackLayout(mw)
	vpack.AddWidget(theme_widj)
	vpack.AddWidget(tabber)

	mw.ResizeN(800, 600)
	return mw
}

func StartGUI(app *core.App) {
	tk.Init() // could this be problematic? is idempotent? without it the root window gets destroyed on quit

	// https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
	fmt.Println(tk.MainInterp().EvalAsStringList(`
set dir ttkfile/fsdialog
source ttkfile/fsdialog/pkgIndex.tcl
package require fsdialog
source ttkthemes/ttkthemes/themes/pkgIndex.tcl
source ttkthemes/ttkthemes/png/pkgIndex.tcl

`))

	slog.Debug("ttk theme list", "theme-list", tk.TtkTheme.ThemeIdList())
	default_theme := "clearlooks"
	err := tk.TtkTheme.SetThemeId(default_theme)
	if err != nil {
		slog.Warn("failed to set default theme", "default-theme", default_theme, "error", err)
	}
	tk.MainLoop(func() {
		mw := NewWindow(app)
		mw.SetTitle(app.KeyVal("bw", "app", "name"))
		mw.Center(nil)
		mw.ShowNormal()
	})
}
