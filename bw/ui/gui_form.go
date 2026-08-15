// form wrangling for the gui.
// renders a `core.Form` as tk widgets and reads the user's input back out of them.

package ui

import (
	"bw/core"
	"fmt"
	"log/slog"

	"github.com/visualfc/atk/tk"
)

// superseded. these types now live in `core` and are used from there.
/*
   type ArgExclusivity string

var (
	ArgChoiceExclusive    ArgExclusivity = "exclusive"
	ArgChoiceNonExclusive ArgExclusivity = "non-exclusive"
)

type ArgChoice struct {
	// labelmap ?
	ChoiceList  []any            // a hardcoded list of things that the user can pick from
	ChoiceFn    func(*App) []any // a function that can be called that yields things the user can pick from
	Exclusivity ArgExclusivity   // can a single or multiple choices be selected?
}

type InputWidget string

var (
	InputWidgetTextField      InputWidget = "text-field"
	InputWidgetTextBox        InputWidget = "text-box"
	InputWidgetSelection      InputWidget = "choice-list"
	InputWidgetMultiSelection InputWidget = "multi-choice-list"
	InputWidgetFileSelection  InputWidget = "file-picker"
	InputWidgetDirSelection   InputWidget = "dir-picker" // just like a file-picker but limited to directories
)

// a description of a single function argument,
// including a parser and a set of validator functions.
type ArgDef struct {
	ID            string        // "name", same requirements as a golang function argument
	Label         string        // "Name"
	Default       string        // value to use when input is blank. value will go through parser and validator.
	Widget        InputWidget   // type of widget to use for input
	Choice        *ArgChoice    // if non-nil, user's input is limited to these choices
	Parser        ParseFn       // parses user input, returning a 'normal' value or an error. string-to-int, string-to-int64, etc
	ValidatorList []PredicateFn // "required", "not-blank", "not-super-long", etc
}
*/

// a form widget whose value can be read and written.
// implemented by each widget the form needs to interact with.
type TKInput interface {
	Get() string
	Set(v string)
}

// ---

type TKEntry struct {
	*tk.Entry
}

func (tke TKEntry) Set(val string) {
	tke.SetText(val)
}

var _ TKInput = (*TKEntry)(nil)

func (tke TKEntry) Get() string {
	return tke.Text()
}

// ---

type TKButton struct {
	*tk.Button
}

func (tkb TKButton) Get() string {
	return tkb.Text() // "Submit", ...
}

func (tkb TKButton) Set(val string) {
	tkb.SetText(val)
}

var _ TKInput = (*TKButton)(nil)

// ---

// a label used as a form input.
// the dialog widgets (select file, select dir, confirm) need somewhere to hold their
// return value, and a label can be read directly in a headless environment where the
// dialog itself cannot be driven.
type TKLabel struct {
	*tk.Label
}

func (tkl TKLabel) Get() string {
	return tkl.Text()
}

func (tkl TKLabel) Set(s string) {
	tkl.SetText(s)
}

var _ TKInput = (*TKLabel)(nil)

// ---

// the widgets making up a single form field, and the `core.ArgDef` they were built from.
type GUIFormField struct {
	argdef    core.ArgDef
	label     tk.Label
	Input     TKInput
	tooltip   tk.Label
	container tk.PackLayout
}

// ---

// a `core.Form` plus the tk widgets rendering it.
type GUIForm struct {
	*core.Form                // the form to wrap
	container  *tk.PackLayout // pointer to the thing wrapping the entire form
	Fields     []GUIFormField // components for each form field, including buttons
}

func MakeGUIForm(f core.Form) GUIForm {
	return GUIForm{
		Form:   &f,
		Fields: []GUIFormField{},
	}
}

// updates each gui form field with the value held in the form data.
// every value is stringified, and it is the `ArgDef.Parser` that converts it back on
// submission.
// todo: there is no reverse of `ArgDef.Parser`, so a non-string value round-trips through
// `%v` formatting.
func (gf *GUIForm) Fill() {
	vals := gf.Data()
	for _, field := range gf.Fields {
		field.Input.Set(fmt.Sprintf("%v", vals[field.argdef.ID]))
	}
}

// ---

// renders the given `argdef` as a labelled widget under `parent`.
// only the text-field and dir-picker widgets are implemented.
// panics if the argdef has no widget, or one that is not implemented.
// the `argval` is used for logging only, the field takes its value from the argdef's
// default.
func RenderServiceArgDef(app *core.App, parent tk.Widget, argdef core.ArgDef, argval any) GUIFormField {
	if argdef.Widget == "" {
		slog.Error("cannot render argdef, argdef.Widget is empty", "argdef", argdef, "argval", argval)
		panic("programming error")
	}

	field := GUIFormField{}
	field.argdef = argdef

	field_container := tk.NewVPackLayout(parent)
	field.container = *field_container

	default_val := argdef.Default
	if argdef.DefaultFn != nil {
		// perhaps in future we defer realising the dynamic values until the form is fully rendered, then update it with dynamic values?
		default_val = argdef.DefaultFn(app)
	}

	// todo: validate the default value?

	tooltip := tk.NewLabel(field_container, argdef.Description) // todo: make actual tooltip. not supported by visualfc/atk atm

	switch argdef.Widget {
	case core.InputWidgetDirSelection:
		lbl := tk.NewLabel(field_container, argdef.Label)

		selected := tk.NewLabel(field_container, argdef.Default)
		selected_input := TKLabel{selected}

		btn := tk.NewButton(field_container, argdef.Label)
		btn_input := &TKButton{btn}

		btn_input.OnCommand(func() {
			mustexist := false // handled in validators
			res, _ := tk.ChooseDirectory(field_container, argdef.ID, default_val, mustexist)
			selected.SetText(res)
		})

		field.label = *lbl
		field.Input = selected_input
		field.tooltip = *tooltip

		field_container.AddWidgets(lbl, tooltip, selected, btn)

	case core.InputWidgetTextField:
		lbl := tk.NewLabel(field_container, argdef.Label)

		e := tk.NewEntry(parent)
		entry := &TKEntry{e}

		field.label = *lbl
		field.Input = entry
		field.tooltip = *tooltip

		field_container.AddWidgets(lbl, tooltip, entry) // gotta be pointers 'nil interface'
	default:
		slog.Error("cannot render argdef, unsupported widget", "widget", argdef.Widget, "argdef", argdef, "argval", argval)
		panic("programming error")
	}

	return field
}

// renders the given `form` as a field per argument plus a submit button.
// submitting validates the input and, when it is valid, queues the service and closes the
// form once it succeeds.
// an invalid form is logged and stays open, without showing the user which field failed.
func RenderServiceForm(gui *GUIUI, parent tk.Widget, form core.Form) *GUIForm {
	gui_form := MakeGUIForm(form)
	gui_form.container = tk.NewVPackLayout(parent)

	for _, argdef := range form.Service.Interface.ArgDefList {
		field := RenderServiceArgDef(gui.App(), gui_form.container, argdef, "")
		gui_form.Fields = append(gui_form.Fields, field)
		gui_form.container.AddWidget(&field.container)
	}

	// every form has a 'submit' button
	submit_btn := tk.NewButton(parent, "Submit")
	gui_form.Fields = append(gui_form.Fields, GUIFormField{
		Input: &TKButton{submit_btn},
	})
	gui_form.container.AddWidget(submit_btn)

	submit_btn.OnCommand(func() {
		keyvals := []core.KeyVal{}
		for _, w := range gui_form.Fields {
			slog.Info("input", "id", w.argdef.ID, "val", w.Input.Get())
			keyvals = append(keyvals, core.KeyVal{Key: w.argdef.ID, Val: w.Input.Get()})
		}

		form.Update(keyvals)

		err := form.Validate()

		if err == nil {
			slog.Info("form is valid", "service", form.Service)
			gui.RunService(form.Service, core.ServiceFnArgs{ArgList: keyvals}, func(res core.ServiceResult) {
				if res.Err == nil {
					gui.current_tab().close_form()
				}
			})
		} else {
			slog.Warn("form is invalid", "error", err)
		}
	})

	return &gui_form
}
