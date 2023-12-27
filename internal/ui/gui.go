package ui

import (
	"bw/internal/core"

	"log/slog"

	"github.com/visualfc/atk/tk"
)

type Window struct {
	*tk.Window
}

func NewWindow() *Window {
	mw := &Window{tk.RootWindow()}
	//lbl := tk.NewLabel(mw, "Hello ATK")
	//btn := tk.NewButton(mw, "Quit")
	//btn.OnCommand(func() {
	//tk.Quit()
	//})
	//tk.NewVPackLayout(mw).AddWidgets(lbl, tk.NewLayoutSpacer(mw, 0, true), btn)

	theme_widj := tk.NewComboBox(mw, &tk.WidgetAttr{"state", "readonly"}, &tk.WidgetAttr{"takeFocus", 0})
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

	tk.Pack(tabber, &tk.LayoutAttr{"expand", 1}, &tk.LayoutAttr{"fill", "both"})

	tabber.AddTab(mf, "main")

	//disabled := tk.WidgetAttr{"state", "disabled"}

	tabber.AddTab(mf2, "main 2") //, &disabled)
	//defer tabber.Destroy()

	vpack := tk.NewVPackLayout(mw)
	vpack.AddWidget(theme_widj)
	vpack.AddWidget(tabber)

	mw.ResizeN(800, 600)
	return mw
}

func StartGUI(app *core.App) {
	tk.Init() // could this be problematic? is idempotent? without it the root window gets destroyed on quit

	// https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
	tk.MainInterp().EvalAsStringList(`
source ttkthemes/ttkthemes/themes/pkgIndex.tcl
source ttkthemes/ttkthemes/png/pkgIndex.tcl
`)

	slog.Debug("ttk theme list", "theme-list", tk.TtkTheme.ThemeIdList())
	default_theme := "clearlooks"
	err := tk.TtkTheme.SetThemeId(default_theme)
	if err != nil {
		slog.Warn("failed to set default theme", "default-theme", default_theme, "error", err)
	}
	tk.MainLoop(func() {
		mw := NewWindow()
		mw.SetTitle(app.KeyVal("bw", "app", "name"))
		mw.Center(nil)
		mw.ShowNormal()
	})
}
