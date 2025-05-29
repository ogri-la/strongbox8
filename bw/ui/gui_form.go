// form wrangling for the gui

package ui

import (
	"bw/core"
	"fmt"
	"log/slog"

	"github.com/visualfc/atk/tk"
)

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

// every form widget that we need to interact with adheres to this interfaces.
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

type GUIFormField struct {
	argdef    core.ArgDef
	label     tk.Label
	input     TKInput
	tooltip   tk.Label
	container tk.PackLayout
}

// ---

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

// updates each gui form field with values from the form data.
func (gf *GUIForm) Fill() {
	vals := gf.Data()
	for _, ff := range gf.Fields {
		// todo: this stringification has serious implications.
		// we're *encoding* (possibly) non-string data and when the user submits that data back at us, we need to wrangling it to a native value again.
		// see GUIForm.GUIFormField.argdef.Parser => takes string, emits normal value. do we have a reverse? takes normal and emits string?

		ff.input.Set(fmt.Sprintf("%v", vals[ff.argdef.ID]))
	}
}

// ---

func RenderServiceArgDef(parent tk.Widget, argdef core.ArgDef, argval any) GUIFormField {
	field := GUIFormField{}
	field.argdef = argdef

	field_container := tk.NewVPackLayout(parent)
	field.container = *field_container

	switch argdef.Widget {
	case core.InputWidgetTextField:
		lbl := tk.NewLabel(field_container, argdef.Label)

		e := tk.NewEntry(parent)
		entry := &TKEntry{e}
		tooltip := tk.NewLabel(field_container, argdef.Description) // todo: make actual tooltip. not supported by visualfc/atk atm

		field.label = *lbl
		field.input = entry
		field.tooltip = *tooltip

		field_container.AddWidgets(lbl, tooltip, entry) // gotta be pointers 'nil interface'
	default:
		panic("unsupported widget: " + argdef.Widget)
	}

	return field
}

func RenderServiceForm(app *core.App, parent tk.Widget, form core.Form) *GUIForm {
	gui_form := MakeGUIForm(form)
	gui_form.container = tk.NewVPackLayout(parent)

	for _, argdef := range form.Service.Interface.ArgDefList {
		field := RenderServiceArgDef(gui_form.container, argdef, "")
		gui_form.Fields = append(gui_form.Fields, field)
		gui_form.container.AddWidget(&field.container)
	}

	// every form has a 'submit' button
	submit_btn := tk.NewButton(parent, "Submit")
	gui_form.Fields = append(gui_form.Fields, GUIFormField{
		input: &TKButton{submit_btn},
	})
	gui_form.container.AddWidget(submit_btn)

	// clicking the 'submit' button populates the underlying form
	submit_btn.OnCommand(func() {
		// on submit:
		// build up a []core.KeyVal of argdef.ID=>widget.value
		keyvals := []core.KeyVal{}
		for _, w := range gui_form.Fields {
			slog.Info("input", "id", w.argdef.ID, "val", w.input.Get())
			keyvals = append(keyvals, core.KeyVal{Key: w.argdef.ID, Val: w.input.Get()})
		}

		// update the form with the input values
		form.Update(keyvals)

		// validate form
		err := form.Validate()

		if err == nil {
			// if no errors,
			// call service with args

			slog.Info("form is valid", "service", form.Service)
			core.CallServiceFnWithArgs(app, form.Service, core.ServiceFnArgs{ArgList: keyvals})
		} else {
			slog.Warn("form is invalid", "error", err)
		}
	})

	return &gui_form
}
