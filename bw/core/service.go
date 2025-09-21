// Services. defining services, calling services

package core

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"
)

// calling a `Service` must return a list of results or an error.
type ServiceResult struct {
	Err    error    `json:",omitempty"`
	Result []Result `json:",omitempty"`
}

// returns an empty, `ServiceResult`.
func NewServiceResult() ServiceResult {
	return ServiceResult{
		Err:    nil,
		Result: []Result{},
	}
}

// a `ServiceResult` is empty if there are no results and no error.
func (fr *ServiceResult) IsEmpty() bool {
	return len(fr.Result) == 0 && fr.Err == nil
}

// convenience. returns a new `ServiceResult` populated from the given `result` list.
func MakeServiceResult(result ...Result) ServiceResult {
	return ServiceResult{Result: result}
}

// convenience. returns a new `ServiceResult` populated from the given `err` and `msg`.
func MakeServiceResultError(err error, msg string) ServiceResult {
	// "could not load settings: file does not exist: /path/to/settings"
	if err != nil {
		return ServiceResult{Err: fmt.Errorf("%s: %w", msg, err)}
	}
	return ServiceResult{Err: errors.New(msg)}
}

// ---

// a `Service` must be called with a `ServiceFnArgs`,
// an ordered list of keys and values,
// whose key maps to a Service.Interface.*.ID value.
// these values are the parsed, validated values that are used as input to the service function.
// todo: does this need to be a struct?
type ServiceFnArgs struct {
	ArgList []KeyVal
}

// returns an empty `ServiceArgs` struct
func NewServiceFnArgs() ServiceFnArgs {
	return ServiceFnArgs{
		ArgList: []KeyVal{},
	}
}

// returns a `ServiceArgs` struct populated with a single `key` and it's `val`.
// todo: delete? doesn't seem to useful
func MakeServiceFnArgs(key string, val any) ServiceFnArgs {
	return ServiceFnArgs{ArgList: []KeyVal{{Key: key, Val: val}}}
}

// ---

// takes a string and returns a value with the intended type
type ParseFn func(*App, string) (any, error)

// take a thing and returns an error or nil
// given thing should be parsed user input (the output of a `ParseFn`).
type PredicateFn func(any) error

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
	ID            string            // "name", same requirements as a golang function argument
	Label         string            // "Name"
	Description   string            // Optional. a short helpful description of the field
	Default       string            // value to use when input is blank. value will go through parser and validator.
	DefaultFn     func(*App) string // if set, called at form creation time to fetch a dynamic default value. preferred over Default.
	Widget        InputWidget       // type of widget to use for input
	Choice        *ArgChoice        // if non-nil, user's input is limited to these choices
	Parser        ParseFn           // parses user input, returning a 'normal' value or an error. string-to-int, string-to-int64, etc
	ValidatorList []PredicateFn     // "required", "not-blank", "not-super-long", etc
}

// a description of a function's list of arguments.
type ServiceInterface struct {
	ArgDefList []ArgDef
	//ValidatorList []PredicateFn // validates a list of args, not individual args.
}

// describes a function that accepts a FnArgList derived from a FnInterface
type Service struct {
	ServiceGroup *ServiceGroup // optional, the group this fn belongs to. provides context, grouping, nothing else.
	ID           string
	Label        string
	Description  string
	Interface    ServiceInterface                        // argument interface for this fn.
	Fn           func(*App, ServiceFnArgs) ServiceResult // the callable.
}

// a ServiceGroup is a natural grouping of services with a unique, descriptive, namespace `NS`.
// a provider may provide many groups of services
type ServiceGroup struct {
	// major group: 'bw', 'os', 'github'
	// minor group: 'state' (bw/state), 'fs' (os/fs), 'orgs' (github/orgs)
	NS          NS
	ServiceList []Service // list of functions within the major/minor group: 'bw/state/print', 'os/fs/list', 'github/orgs/list'
}

// ---

func ParseArgDef(app *App, arg ArgDef, raw_uin string) (any, error) {
	var err error
	defer func() {
		r := recover()
		if r != nil {
			slog.Error("programming error. parser panicked", "arg-def", arg, "raw-uin", raw_uin)
			err = errors.New("validator failed")
		}
	}()
	parsed_val, err := arg.Parser(app, raw_uin)
	if err != nil {
		return nil, fmt.Errorf("error parsing user input: %w", err)
	}
	return parsed_val, err
}

func get_function_name(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func ValidateArgDef(arg ArgDef, parsed_uin any) error {
	var err error
	defer func() {
		r := recover()
		if r != nil {
			slog.Error("programming error. validator panicked", "arg-def", arg, "parsed-uin", parsed_uin)
			err = errors.New("validator failed")
		}
	}()
	if len(arg.ValidatorList) > 0 {
		for _, validator := range arg.ValidatorList {
			slog.Debug("validing value", "validator", get_function_name(validator), "value", parsed_uin)
			err = validator(parsed_uin)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func CallServiceFnWithArgs(app *App, service Service, args ServiceFnArgs) ServiceResult {
	if service.Fn == nil {
		return MakeServiceResultError(nil, "Service has no callback")
	}
	var result ServiceResult
	defer func() {
		r := recover()
		if r != nil {
			slog.Error("recovered from service function panic", "fn", service, "panic", r)
			fmt.Println(string(debug.Stack()))
			result = ServiceResult{Err: errors.New("panicked")}
		}
	}()
	result = service.Fn(app, args)
	return result
}
