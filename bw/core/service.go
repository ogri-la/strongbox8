// Services. defining services, calling services

package core

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
)

// calling a `Service` must return a list of results or an error.
type ServiceResult struct {
	Err    error    `json:",omitempty"`
	Result []Result `json:",omitempty"`
}

// returns an empty `ServiceResult`.
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

// returns an empty `ServiceFnArgs` struct
func NewServiceFnArgs() ServiceFnArgs {
	return ServiceFnArgs{
		ArgList: []KeyVal{},
	}
}

// returns a `ServiceFnArgs` struct populated with a single `key` and it's `val`.
// todo: delete? doesn't seem too useful
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
	ChoiceList  []any            // a hardcoded list of things that the user can pick from
	ChoiceFn    func(*App) []any // a function that can be called that yields things the user can pick from
	Exclusivity ArgExclusivity   // can a single or multiple choices be selected?
	LabelFn     func(any) string // the label shown for a choice. nil is `fmt.Sprint`. labels must be unique.

	// yields the choices given the form's other values, such as the selected items, for
	// choices that depend on what was selected. preferred over `ChoiceFn`.
	ChoiceArgsFn func(app *App, args map[string]any) []any

	args map[string]any // the form values given to `ChoiceArgsFn`, see `WithArgs`
}

// returns a copy of the choice that computes its choices from the form values `args`.
func (c *ArgChoice) WithArgs(args map[string]any) *ArgChoice {
	with_args := *c
	with_args.args = args
	return &with_args
}

// returns the choices the user can pick from: `ChoiceArgsFn` when set, then `ChoiceFn`,
// otherwise `ChoiceList`.
func (c *ArgChoice) Choices(app *App) []any {
	if c.ChoiceArgsFn != nil {
		return c.ChoiceArgsFn(app, c.args)
	}
	if c.ChoiceFn != nil {
		return c.ChoiceFn(app)
	}
	return c.ChoiceList
}

// returns the label shown for `choice`.
func (c *ArgChoice) Label(choice any) string {
	if c.LabelFn != nil {
		return c.LabelFn(choice)
	}
	return fmt.Sprint(choice)
}

// returns the labels of every choice, in order.
func (c *ArgChoice) Labels(app *App) []string {
	label_list := []string{}
	for _, choice := range c.Choices(app) {
		label_list = append(label_list, c.Label(choice))
	}
	return label_list
}

// returns the choice whose label is `label`, or an error when there is none.
func (c *ArgChoice) Find(app *App, label string) (any, error) {
	for _, choice := range c.Choices(app) {
		if c.Label(choice) == label {
			return choice, nil
		}
	}
	return nil, fmt.Errorf("not one of the available choices: %s", label)
}

// returns `val` resolved to the choice(s) it names. a label is looked up, a list of
// labels is looked up item by item for a non-exclusive choice, and any other value must
// be one of the choices itself.
func (c *ArgChoice) Resolve(app *App, val any) (any, error) {
	switch v := val.(type) {
	case string:
		choice, err := c.Find(app, v)
		if err == nil {
			return choice, nil
		}
		// a string may be the choice itself rather than its label
		for _, choice := range c.Choices(app) {
			if choice_str, is_str := choice.(string); is_str && choice_str == v {
				return choice, nil
			}
		}
		return nil, err
	case []string:
		if c.Exclusivity != ArgChoiceNonExclusive {
			return nil, fmt.Errorf("only one choice can be selected")
		}
		resolved := []any{}
		for _, label := range v {
			choice, err := c.Find(app, label)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, choice)
		}
		return resolved, nil
	default:
		return c.Find(app, c.Label(v))
	}
}

// options for a file or directory picker.
type FilePickerOpts struct {
	Multiple     bool              // allow several files to be chosen
	Extensions   []string          // only offer files with these extensions, ".zip". empty offers all.
	InitialDirFn func(*App) string // where the picker opens. nil opens wherever the desktop chooses.
}

type InputWidget string

var (
	InputWidgetTextField      InputWidget = "text-field"
	InputWidgetTextBox        InputWidget = "text-box"
	InputWidgetSelection      InputWidget = "choice-list"
	InputWidgetMultiSelection InputWidget = "multi-choice-list"
	InputWidgetFileSelection  InputWidget = "file-picker"
	InputWidgetDirSelection   InputWidget = "dir-picker" // just like a file-picker but limited to directories
	InputWidgetCheckbox       InputWidget = "checkbox"   // a yes/no value, parse with `ParseTruthyFalseyAsBool`
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
	FromSelection bool              // filled from the items the user selected, never shown as a form field
	FilePicker    *FilePickerOpts   // options for the file-picker and dir-picker widgets
}

// a description of a function's list of arguments.
type ServiceInterface struct {
	ArgDefList []ArgDef
	//ValidatorList []PredicateFn // validates a list of args, not individual args.
}

// the item types a service accepts from a selection of results.
// a service accepting `T` with `Many` is offered for a selection of one `T` and for a
// selection of several. without `Many`, only for a selection of exactly one.
type Accepts struct {
	Types []reflect.Type
	Many  bool
}

// describes a function that accepts a FnArgList derived from a FnInterface
type Service struct {
	ServiceGroup *ServiceGroup // optional, the group this fn belongs to. provides context, grouping, nothing else.
	ID           string
	Label        string
	Description  string
	Interface    ServiceInterface                        // argument interface for this fn.
	Fn           func(*App, ServiceFnArgs) ServiceResult // the callable.

	// the item types this service is offered for in context menus. nil is never offered.
	Accepts *Accepts

	// reports whether the service can be used with the `selected` results right now.
	// called with no results for a menu item. nil always applies.
	Applicable func(app *App, selected []Result) bool

	// returns a message the user must confirm before the service runs from the GUI.
	// nil, or an empty message, runs without asking.
	Confirm func(app *App, args ServiceFnArgs) string
}

// returns `true` when the service needs input beyond the items selected, so a form must
// be shown before it can be called.
func (s Service) NeedsInput() bool {
	for _, argdef := range s.Interface.ArgDefList {
		if !argdef.FromSelection {
			return true
		}
	}
	return false
}

// returns `true` when the service can be used with the `selected` results.
// a service without a callable never applies.
func (s Service) IsApplicable(app *App, selected []Result) bool {
	if s.Fn == nil {
		return false
	}
	if s.Applicable == nil {
		return true
	}
	return s.Applicable(app, selected)
}

// returns the confirmation message for calling the service with `args`, or "" when none
// is needed.
func (s Service) ConfirmMessage(app *App, args ServiceFnArgs) string {
	if s.Confirm == nil {
		return ""
	}
	return s.Confirm(app, args)
}

// returns the services offered for a selection, keyed by item type: `T` for a selection
// of one, `[]T` for a selection of several.
// a map from type to services, so the GUI finds a selection's services by one lookup.
// services without a callable or without `Accepts` are left out.
func DeriveTypeMap(group_list []ServiceGroup) map[reflect.Type][]Service {
	type_map := map[reflect.Type][]Service{}
	for _, group := range group_list {
		for _, service := range group.ServiceList {
			if service.Fn == nil || service.Accepts == nil {
				continue
			}
			for _, t := range service.Accepts.Types {
				type_map[t] = append(type_map[t], service)
				if service.Accepts.Many {
					many := reflect.SliceOf(t)
					type_map[many] = append(type_map[many], service)
				}
			}
		}
	}
	return type_map
}

// a service offered for some selected results, and whether it can be used with them.
type SelectionService struct {
	Service Service
	Enabled bool
}

// the selected results of one item type and the services offered for them.
type SelectionGroup struct {
	Type        reflect.Type
	Items       []Result
	ServiceList []SelectionService
}

// returns the selected results grouped by item type, each group with the services
// offered for it and whether each is enabled.
// groups are ordered by type name and keep the services' declared order, so menus built
// from them are stable. groups with no services are left out.
func SelectionGroups(type_map map[reflect.Type][]Service, app *App, selected []Result) []SelectionGroup {
	grouped := map[reflect.Type][]Result{}
	for _, r := range selected {
		t := reflect.TypeOf(r.Item)
		grouped[t] = append(grouped[t], r)
	}

	group_list := []SelectionGroup{}
	for t, items := range grouped {
		key := t
		if len(items) > 1 {
			key = reflect.SliceOf(t) // T => []T
		}
		service_list := type_map[key]
		if len(service_list) == 0 {
			continue
		}
		group := SelectionGroup{Type: t, Items: items}
		for _, service := range service_list {
			group.ServiceList = append(group.ServiceList, SelectionService{
				Service: service,
				Enabled: service.IsApplicable(app, items),
			})
		}
		group_list = append(group_list, group)
	}
	slices.SortFunc(group_list, func(a, b SelectionGroup) int {
		return strings.Compare(a.Type.String(), b.Type.String())
	})
	return group_list
}

// returns the args for calling `service` with the `selected` results: the argument
// marked `FromSelection` gets one result when one is selected, or the list when several
// are.
func SelectionArgs(service Service, selected []Result) ServiceFnArgs {
	args := NewServiceFnArgs()
	for _, argdef := range service.Interface.ArgDefList {
		if !argdef.FromSelection {
			continue
		}
		ptr_list := make([]*Result, len(selected))
		for i := range selected {
			ptr_list[i] = &selected[i]
		}
		var val any = ptr_list
		if len(ptr_list) == 1 {
			val = ptr_list[0]
		}
		args.ArgList = append(args.ArgList, KeyVal{Key: argdef.ID, Val: val})
	}
	return args
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

// parses the raw user input `raw_uin` with the parser of the given `arg`,
// returning the parsed value or an error.
// a parser that panics is a programming error: it is logged and reported as an error.
func ParseArgDef(app *App, arg ArgDef, raw_uin string) (parsed_val any, err error) {
	defer func() {
		r := recover()
		if r != nil {
			slog.Error("programming error. parser panicked", "arg-def", arg.ID, "raw-uin", raw_uin, "panic", r)
			parsed_val = nil
			err = errors.New("parser failed")
		}
	}()
	parsed_val, err = arg.Parser(app, raw_uin)
	if err != nil {
		return nil, fmt.Errorf("error parsing user input: %w", err)
	}
	return parsed_val, nil
}

func get_function_name(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// runs each of the given `arg`'s validators against `parsed_uin`,
// returning the first error, or nil when all pass.
// a validator that panics is a programming error: it is logged and reported as an error.
func ValidateArgDef(arg ArgDef, parsed_uin any) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			slog.Error("programming error. validator panicked", "arg-def", arg.ID, "parsed-uin", parsed_uin, "panic", r)
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

// calls the given `service` with `args` and returns its result.
// a service with no callback, or one that panics, returns a `ServiceResult` with an
// error set rather than bringing down the app.
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
