// form.go is responsible for coordinating the collection of inputs for a `Service`
// and finally yielding a valid `ServiceFnArgs` that satifies the `ServiceInterface`,
// or a `FormError` with detailed validation information.

package core

import "fmt"

type Form struct {
	Service Service
	input   ServiceFnArgs
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

// update the form with new inputs
func (f *Form) Update(arg_list []KeyVal) {
	f.input = ServiceFnArgs{ArgList: arg_list}
}

func (f *Form) Validate() *FormError {
	fe := NewFormError()

	keyvalidx := map[string]any{} // field-id => KeyVal.Val
	for _, arg := range f.input.ArgList {
		keyvalidx[arg.Key] = arg.Val
	}

	// for each defined arg,
	for i := range len(f.Service.Interface.ArgDefList) {
		argdef := f.Service.Interface.ArgDefList[i]
		argval, has_val := keyvalidx[argdef.ID]
		if !has_val {
			argval = argdef.Default
		}
		err := ValidateArgDef(argdef, argval)
		if err != nil {
			fe.FieldErrorList[argdef.ID] = err
		}
	}

	if len(fe.FieldErrorList) > 0 {
		fe.Error = fmt.Errorf("form has errors")
		return &fe
	}

	return nil
}

// ---

// resetting a form clears any updates and returns each field to their default values.
func (f *Form) Reset() {
	*f = MakeForm(f.Service)
}

// submitting a form yields a valid set of service function arguments,
// or a FormError with form-level and field-level error information.
func (f *Form) Submit() (ServiceFnArgs, *FormError) {
	empty_result := ServiceFnArgs{}

	ferr := f.Validate()
	if ferr != nil {
		return empty_result, ferr
	}

	return f.input, nil
}
