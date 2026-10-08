// form.go is responsible for coordinating the collection of inputs for a `Service`
// and finally yielding a valid `ServiceFnArgs` that satifies the `ServiceInterface`,
// or a `FormError` with detailed validation information.

package core

import "fmt"

type Form struct {
	Service Service
	App     *App // optional, given to parsers that need it
	input   ServiceFnArgs
	parsed  ServiceFnArgs // the inputs after parsing, set by `Validate`
}

func NewForm() Form {
	return Form{
		input: NewServiceFnArgs(),
	}
}

func MakeForm(s Service) Form {
	f := NewForm()
	f.Service = s
	return f
}

// ---

type FormError struct {
	Error          error            // form-level error
	FieldErrorList map[string]error // a mapping of ArgDef.ID to an error
}

func NewFormError() FormError {
	return FormError{
		FieldErrorList: map[string]error{},
	}
}

// ---

// returns the bound form data as a map of field-id=>any values.
func (f *Form) Data() map[string]any {
	kvmap := map[string]any{} // field-id => KeyVal.Val
	for _, arg := range f.input.ArgList {
		kvmap[arg.Key] = arg.Val
	}
	return kvmap
}

// replaces the form's inputs with `arg_list`.
// inputs not present in `arg_list` are discarded.
func (f *Form) Update(arg_list []KeyVal) {
	f.input = ServiceFnArgs{ArgList: arg_list}
}

// returns the value for `argdef` given its bound `raw` value: a choice is resolved from
// its label, a string is parsed with the argdef's parser, any other value (such as
// selected results) is used as it is.
func (f *Form) parse(argdef ArgDef, raw any) (any, error) {
	if argdef.Choice != nil && !argdef.FromSelection {
		// a choice is picked by its label, and must be one of the choices
		return argdef.Choice.WithArgs(f.Data()).Resolve(f.App, raw)
	}
	raw_str, is_str := raw.(string)
	if !is_str || argdef.Parser == nil {
		return raw, nil
	}
	return ParseArgDef(f.App, argdef, raw_str)
}

// parses each field of the service's interface and checks it against its validators,
// returning nil when every field passes.
// a field with no input takes its `ArgDef.Default`.
// the parsed values are kept for `Submit`.
func (f *Form) Validate() *FormError {
	fe := NewFormError()

	keyvalidx := f.Data()
	parsed := NewServiceFnArgs()

	for _, argdef := range f.Service.Interface.ArgDefList {
		argval, has_val := keyvalidx[argdef.ID]
		if !has_val {
			argval = argdef.Default
		}
		parsed_val, err := f.parse(argdef, argval)
		if err != nil {
			fe.FieldErrorList[argdef.ID] = err
			continue
		}
		err = ValidateArgDef(argdef, parsed_val)
		if err != nil {
			fe.FieldErrorList[argdef.ID] = err
			continue
		}
		parsed.ArgList = append(parsed.ArgList, KeyVal{Key: argdef.ID, Val: parsed_val})
	}

	if len(fe.FieldErrorList) > 0 {
		fe.Error = fmt.Errorf("form has errors")
		return &fe
	}

	f.parsed = parsed
	return nil
}

// ---

// resetting a form clears any updates and returns each field to their default values.
func (f *Form) Reset() {
	*f = MakeForm(f.Service)
}

// submitting a form yields a valid set of parsed service function arguments, in the
// order of the service's interface, or a FormError with form-level and field-level error
// information.
func (f *Form) Submit() (ServiceFnArgs, *FormError) {
	empty_result := ServiceFnArgs{}

	ferr := f.Validate()
	if ferr != nil {
		return empty_result, ferr
	}

	return f.parsed, nil
}
