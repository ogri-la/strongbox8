package core

import (
	"errors"
	"fmt"
	"log/slog"
)

//

type NS struct {
	Major string
	Minor string
	Type  string
}

var (
	BW_NS_RESULT_LIST = NS{Major: "bw", Minor: "core", Type: "result-list"}
	BW_NS_ERROR       = NS{"bw", "core", "error"}
	BW_NS_STATE       = NS{"bw", "core", "state"}
	BW_NS_SERVICE     = NS{"bw", "core", "service"}
	BW_NS_FS_FILE     = NS{"bw", "fs", "file"}
	BW_NS_FS_DIR      = NS{"bw", "fs", "dir"}
)

// ----

// simple key+val. val can be anything.
type Arg struct {
	Key string
	Val interface{}
}

// the payload a service function must return
type FnResult struct {
	Err    error `json:"-"` // omitempty?
	Result Result
}

// the key+vals a service function must take as input.
type FnArgs struct {
	ArgList []Arg
}

// take a thing and returns an error or nil
// given thing should be parsed user input.
type PredicateFn func(interface{}) error

// takes a string and returns a value with the intended type
type ParseFn func(*App, string) (interface{}, error)

// a description of a single function argument,
// including a parser and a set of validator functions.
type ArgDef struct {
	ID            string        // "name", same requirements as a golang function
	Label         string        // "Name"
	Default       string        // value to use when input is blank. value goes through parser and validator.
	Parser        ParseFn       // parses user input, returning a 'normal' value or an error. string-to-int, string-to-int64, etc
	ValidatorList []PredicateFn // "required", "not-blank", "not-super-long", etc
}

// a description of a function's list of arguments.
type FnInterface struct {
	ArgDefList []ArgDef
}

// describes a function that accepts a FnArgList derived from a FnInterface
type Fn struct {
	Service     *Service                    // optional, the group this fn belongs to. provides context, grouping, nothing else.
	Label       string                      // friendly name for this function
	Description string                      // friendly description of this function's behaviour
	Interface   FnInterface                 // argument interface for this fn.
	TheFn       func(*App, FnArgs) FnResult // the callable.
}

// a service has a unique namespace 'NS', a friendly label and a collection of functions.
type Service struct {
	// major group: 'bw', 'os', 'github'
	// minor group: 'state' (bw/state), 'fs' (os/fs), 'orgs' (github/orgs)
	NS     NS
	FnList []Fn // list of functions within the major/minor group: 'bw/state/print', 'os/fs/list', 'github/orgs/list'
}

// ---

func ErrorFnResult(err error, msg string) FnResult {
	// "could not load settings: file does not exist: /path/to/settings"
	return FnResult{Err: fmt.Errorf("%s: %w", msg, err)}
}

func ParseArgDef(app *App, arg ArgDef, raw_uin string) (interface{}, error) {
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

func ValidateArgDef(arg ArgDef, parsed_uin interface{}) error {
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
			err = validator(parsed_uin)
			if err != nil {
				break
			}
		}
	}
	return nil
}

func CallServiceFnWithArgs(app *App, fn Fn, args FnArgs) FnResult {
	var result FnResult
	defer func() {
		r := recover()
		if r != nil {
			slog.Warn("recovered from service function panic", "fn", fn, "panic", r)
			result = FnResult{Err: errors.New("panicked")}
		}
	}()
	result = fn.TheFn(app, args)
	return result
}

// ---

type Result struct {
	ID   string `json:"id"`
	NS   NS     `json:"ns"`
	Item any    `json:"item"`
}

func EmptyResult(r Result) bool {
	return r.ID == ""
}

func NewResult(ns NS, i any) Result {
	return Result{
		ID:   UniqueID(),
		NS:   ns,
		Item: i,
	}
}

// the application's moving parts.
type State struct {
	// bw: config: no-colour: "true"
	KeyVals    map[string]map[string]map[string]string
	ResultList Result
}

type IApp interface {
	RegisterService(service Service)
	UpdateResultList(result Result)
	FunctionList() ([]Fn, error)
	ResetState()
}

type App struct {
	IApp
	State       *State
	ServiceList []Service
}

func NewState() *State {
	state := State{}
	state.KeyVals = map[string]map[string]map[string]string{}
	state.ResultList = NewResult(BW_NS_STATE, []Result{})
	return &state
}

func NewApp() *App {
	app := App{}
	app.State = NewState()
	app.ServiceList = []Service{}
	return &app
}

func (app *App) SetKeyVals(major string, minor string, keyvals map[string]string) {
	for key, val := range keyvals {
		mj, present := app.State.KeyVals[major]
		if !present {
			mj = map[string]map[string]string{}
		}
		mn, present := mj[minor]
		if !present {
			mn = map[string]string{}
		}
		mn[key] = val
		mj[minor] = mn
		app.State.KeyVals[major] = mj

		//app.State.KeyVals[major][minor][key] = val
	}
}

// returns a specific keyval for the given major+minor+key
func (app *App) KeyVal(major, minor, key string) string {
	mj, present := app.State.KeyVals[major]
	if !present {
		return ""
	}
	mn, present := mj[minor]
	if !present {
		return ""
	}
	v, present := mn[key]
	if !present {
		return ""
	}
	return v
}

// returns all keyvals for the given major+minor ns.
func (app *App) KeyVals(major, minor string) map[string]string {
	empty_map := map[string]string{}
	mj, present := app.State.KeyVals[major]
	if !present {
		return empty_map
	}
	mn, present := mj[minor]
	if !present {
		return empty_map
	}
	return mn
}

// adds `result` to app state.
// empty results are skipped.
func (app *App) UpdateResultList(result Result) {
	if EmptyResult(result) {
		return
	}
	root := app.State.ResultList.Item.([]Result)

	// little bit of a hack. if the result's payload is a list of Results,
	// 'flatten' the list and add each individually
	flatten := NS{}
	result_list, ok_to_flatten := result.Item.([]Result)
	if result.NS == flatten && ok_to_flatten {
		// if NS is empty and the result's payload is something the can be flattened, add each one individually
		root = append(root, result_list...)
	} else {
		root = append(root, result)
		app.State.ResultList.Item = root
	}

}

func (app *App) RegisterService(service Service) {
	app.ServiceList = append(app.ServiceList, service)
}

// ---------

func FullyQualifiedServiceName(s Service) string {
	return fmt.Sprintf("%s/%s", s.NS.Major, s.NS.Minor)
}

// "os/fs/list", "os/hardware/cpus",
// "github/orgs/list-repos", "github/users/list-repos"
func FullyQualifiedFnName(f Fn) string {
	if f.Service != nil {
		return fmt.Sprintf("%s/%s/%s", f.Service.NS.Major, f.Service.NS.Minor, f.Label)
	}
	return f.Label
}

// TODO: turn this into a stop + restart thing.
// throw an error, have main.main catch it and call stop() then start()
func (a *App) ResetState() {
	a.State = NewState()
}

func (a *App) FunctionList() []Fn {
	var fn_list []Fn
	for _, service := range a.ServiceList {
		service := service
		for _, fn := range service.FnList {
			fn.Service = &service
			fn_list = append(fn_list, fn)
		}
	}
	return fn_list
}

func Start() *App {
	return NewApp()
}
