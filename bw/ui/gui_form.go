// form wrangling for the gui.
// renders a `core.Form` as tk widgets and reads the user's input back out of them.

package ui

import (
	"bw/core"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
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

	// returns the field's value when it is not a single string, such as several chosen
	// files. nil uses `Input.Get()`.
	value func() any
}

// returns the ID of the argument this field is for, "" for the submit button.
func (f GUIFormField) ID() string {
	return f.argdef.ID
}

// returns the field's current value for submitting.
func (f GUIFormField) Value() any {
	if f.value != nil {
		return f.value()
	}
	return f.Input.Get()
}

// a checkbox used as a form input, reading and writing "true" and "false".
type TKCheckbox struct {
	*tk.CheckButton
}

func (c TKCheckbox) Get() string {
	if c.IsChecked() {
		return "true"
	}
	return "false"
}

func (c TKCheckbox) Set(val string) {
	b, err := core.ParseTruthyFalseyAsBool(nil, val)
	c.SetChecked(err == nil && b.(bool))
}

var _ TKInput = (*TKCheckbox)(nil)

// a combobox used as a form input, holding the label of the chosen item.
type TKCombobox struct {
	*tk.ComboBox
}

func (c TKCombobox) Get() string {
	return c.CurrentText()
}

func (c TKCombobox) Set(val string) {
	c.SetCurrentText(val)
}

var _ TKInput = (*TKCombobox)(nil)

// a list box used as a form input for choosing several items.
// `Get` returns the chosen labels joined by newlines, `GUIFormField.Value` the list.
type TKListbox struct {
	*tk.ListBox
}

func (l TKListbox) Get() string {
	return strings.Join(l.SelectedItems(), "\n")
}

func (l TKListbox) Set(val string) {
	l.ClearSelection()
	wanted := strings.Split(val, "\n")
	for i, item := range l.Items() {
		if slices.Contains(wanted, item) {
			l.SetSelectionRange(i, i)
		}
	}
}

var _ TKInput = (*TKListbox)(nil)

// returns the `tk.FileType` filters for a picker offering only `ext_list`.
func file_types(ext_list []string) []tk.FileType {
	if len(ext_list) == 0 {
		return nil
	}
	ft_list := []tk.FileType{}
	for _, ext := range ext_list {
		ft_list = append(ft_list, tk.FileType{Info: strings.TrimPrefix(ext, ".") + " files", Ext: ext})
	}
	return ft_list
}

// ---

// a `core.Form` plus the tk widgets rendering it.
type GUIForm struct {
	*core.Form                // the form to wrap
	container  *tk.PackLayout // pointer to the thing wrapping the entire form
	Fields     []GUIFormField // components for each form field, including buttons
	selection  map[string]any // the values the form was opened with, for `FromSelection` args
}

// the widgets `RenderServiceArgDef` can render.
var KNOWN_WIDGETS = mapset.NewSet(
	core.InputWidgetTextField,
	core.InputWidgetSelection,
	core.InputWidgetMultiSelection,
	core.InputWidgetFileSelection,
	core.InputWidgetDirSelection,
	core.InputWidgetCheckbox,
)

// reads every field's value into the form, plus any values from the selection, then
// submits it, returning parsed args or the validation errors.
func (gf *GUIForm) SubmitFields() (core.ServiceFnArgs, *core.FormError) {
	keyvals := []core.KeyVal{}
	// values from the selection are not form fields, carry them through as they are
	for _, argdef := range gf.Service.Interface.ArgDefList {
		if argdef.FromSelection {
			keyvals = append(keyvals, core.KeyVal{Key: argdef.ID, Val: gf.selection[argdef.ID]})
		}
	}
	for _, field := range gf.Fields {
		if field.argdef.ID == "" {
			continue // the submit button
		}
		keyvals = append(keyvals, core.KeyVal{Key: field.argdef.ID, Val: field.Value()})
	}
	gf.Update(keyvals)
	return gf.Submit()
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
// an argdef without a widget is rendered as a text field.
// an unsupported widget is logged at ERROR and rendered as a note, so the rest of the
// form can still be used.
// the field starts with the argdef's default, or `argval` when one is given.
func RenderServiceArgDef(app *core.App, parent tk.Widget, argdef core.ArgDef, argval any) GUIFormField {
	return render_service_arg_def(app, parent, argdef, argval, nil)
}

// renders `argdef` as `RenderServiceArgDef` does, with the form's values `form_args` for
// choices that depend on them.
func render_service_arg_def(app *core.App, parent tk.Widget, argdef core.ArgDef, argval any, form_args map[string]any) GUIFormField {
	field := GUIFormField{}
	field.argdef = argdef

	field_container := tk.NewVPackLayout(parent)
	field.container = *field_container

	default_val := argdef.Default
	if argdef.DefaultFn != nil {
		default_val = argdef.DefaultFn(app)
	}
	if argval_str, is_str := argval.(string); is_str && argval_str != "" {
		default_val = argval_str
	}

	lbl := tk.NewLabel(field_container, argdef.Label)
	tooltip := tk.NewLabel(field_container, argdef.Description) // todo: make actual tooltip. not supported by visualfc/atk atm
	field.label = *lbl
	field.tooltip = *tooltip

	widget := argdef.Widget
	if widget == "" {
		widget = core.InputWidgetTextField
	}

	picker := argdef.FilePicker
	if picker == nil {
		picker = &core.FilePickerOpts{}
	}
	initial_dir := ""
	if picker.InitialDirFn != nil {
		initial_dir = picker.InitialDirFn(app)
	}

	switch widget {
	case core.InputWidgetDirSelection:
		selected := tk.NewLabel(field_container, default_val)
		field.Input = TKLabel{selected}

		btn := tk.NewButton(field_container, "Choose ...")
		btn.OnCommand(func() {
			start := initial_dir
			if start == "" {
				start = default_val
			}
			mustexist := false // handled in validators
			res, _ := tk.ChooseDirectory(field_container, argdef.Label, start, mustexist)
			if res != "" {
				selected.SetText(res)
			}
		})
		field_container.AddWidgets(lbl, tooltip, selected, btn)

	case core.InputWidgetFileSelection:
		selected := tk.NewLabel(field_container, default_val)
		field.Input = TKLabel{selected}
		chosen := []string{}
		if default_val != "" {
			chosen = []string{default_val}
		}
		if picker.Multiple {
			field.value = func() any {
				if len(chosen) == 0 {
					return ""
				}
				return slices.Clone(chosen)
			}
		}

		btn := tk.NewButton(field_container, "Choose ...")
		btn.OnCommand(func() {
			if picker.Multiple {
				res, _ := tk.GetOpenMultipleFile(field_container, argdef.Label, file_types(picker.Extensions), initial_dir, "")
				if len(res) > 0 {
					chosen = res
					selected.SetText(strings.Join(res, "\n"))
				}
				return
			}
			res, _ := tk.GetOpenFile(field_container, argdef.Label, file_types(picker.Extensions), initial_dir, "")
			if res != "" {
				chosen = []string{res}
				selected.SetText(res)
			}
		})
		field_container.AddWidgets(lbl, tooltip, selected, btn)

	case core.InputWidgetSelection, core.InputWidgetMultiSelection:
		if argdef.Choice == nil {
			slog.Error("cannot render choice widget, argdef has no choices", "argdef", argdef.ID)
			field.Input = TKLabel{tk.NewLabel(field_container, "(no choices available)")}
			field_container.AddWidgets(lbl, tooltip)
			break
		}
		label_list := argdef.Choice.WithArgs(form_args).Labels(app)
		if widget == core.InputWidgetMultiSelection || argdef.Choice.Exclusivity == core.ArgChoiceNonExclusive {
			lb := tk.NewListBox(field_container)
			lb.SetSelectMode(tk.ListSelectMultiple)
			lb.SetItems(label_list)
			input := TKListbox{lb}
			input.Set(default_val)
			field.Input = input
			field.value = func() any { return lb.SelectedItems() }
			field_container.AddWidgets(lbl, tooltip, lb)
		} else {
			cb := tk.NewComboBox(field_container)
			cb.SetValues(label_list)
			cb.SetState(tk.StateReadOnly)
			input := TKCombobox{cb}
			input.Set(default_val)
			field.Input = input
			field_container.AddWidgets(lbl, tooltip, cb)
		}

	case core.InputWidgetCheckbox:
		cb := tk.NewCheckButton(field_container, argdef.Label)
		input := TKCheckbox{cb}
		input.Set(default_val)
		field.Input = input
		field_container.AddWidgets(tooltip, cb)

	case core.InputWidgetTextField:
		e := tk.NewEntry(field_container)
		entry := &TKEntry{e}
		entry.Set(default_val)
		field.Input = entry
		field_container.AddWidgets(lbl, tooltip, entry) // gotta be pointers 'nil interface'

	default:
		// `RenderServiceForm` has already logged this with the service's name
		slog.Debug("cannot render argdef, unsupported widget", "widget", widget, "argdef", argdef.ID)
		note := tk.NewLabel(field_container, fmt.Sprintf("(unsupported input: %s)", widget))
		field.Input = TKLabel{note}
		field_container.AddWidgets(lbl, tooltip, note)
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

	form.App = gui.App()
	gui_form.Form.App = gui.App()
	initial := form.Data()
	gui_form.selection = initial

	for _, argdef := range form.Service.Interface.ArgDefList {
		if argdef.FromSelection {
			continue // filled from the selection, carried through on submit
		}
		if argdef.Widget != "" && !KNOWN_WIDGETS.Contains(argdef.Widget) {
			slog.Error("service form has an unsupported widget", "service", form.Service.ID, "argdef", argdef.ID, "widget", argdef.Widget)
		}
		field := render_service_arg_def(gui.App(), gui_form.container, argdef, initial[argdef.ID], initial)
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
		args, ferr := gui_form.SubmitFields()
		if ferr != nil {
			slog.Warn("form is invalid", "error", ferr.Error, "field-errors", ferr.FieldErrorList)
			return
		}

		slog.Debug("form is valid", "service", form.Service.ID)
		gui.ConfirmAndRunService(form.Service, args, func(res core.ServiceResult) {
			if res.Err == nil {
				gui.current_tab().close_form()
			}
		})
	})

	return &gui_form
}
