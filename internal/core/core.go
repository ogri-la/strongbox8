package core

import (
	"fmt"
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

// take a string and returns an error or nil
type PredicateFn func(string) error

// takes a string and returns a value with the intended type
type CoerceFn func(string) (interface{}, error)

// a description of a single function argument,
// including a set of validator functions and a final coerceion function.
type ArgDef struct {
	ID            string      // "name", same requirements as a golang function
	Label         string      // "Name"
	Default       string      // value to use when input is blank. value goes through validator and coercer as well.
	Validator     PredicateFn // "required", "not-blank", "not-super-long", etc. skipped if `ValidatorList` present.
	ValidatorList []PredicateFn
	Coercer       CoerceFn // string-to-int, string-to-int64, string-to-person-struct, etc
}

// a description of a function's list of arguments.
type FnInterface struct {
	ArgDefList []ArgDef
}

// describes a function that accepts a FnArgList derived from a FnInterface
type Fn struct {
	Service     *Service // optional, the group this fn belongs to. provides context, grouping, nothing else.
	Label       string   // 'list-files', 'clone', etc. becomes: 'os/list-files' and 'github/clone'
	Description string
	Interface   FnInterface           // argument interface for this fn.
	TheFn       func(FnArgs) FnResult // the callable.

}

// a service has a unique namespace 'NS', a friendly label and a collection of functions.
type Service struct {
	// major group: 'bw', 'os', 'github'
	// minor group: 'state' (bw/state), 'fs' (os/fs), 'orgs' (github/orgs)
	NS     NS
	FnList []Fn // list of functions within the major/minor group: 'bw/state/print', 'os/fs/list', 'github/orgs/list'
}

// ---

type Result struct {
	NS      NS     `json:"ns"`
	ID      string `json:"id"`
	Payload any    `json:"payload"`
}

func EmptyResult(r Result) bool {
	return r.ID == ""
}

func NewResult(ns NS, i any) Result {
	return Result{
		ID:      UniqueID(),
		NS:      ns,
		Payload: i,
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
}

type App struct {
	IApp
	State       *State
	ServiceList []Service
}

var APP *App = nil

func GetApp() *App {
	if APP == nil {
		app := App{}
		app.State = &State{}
		app.State.KeyVals = map[string]map[string]map[string]string{}
		app.State.ResultList = NewResult(BW_NS_STATE, []Result{})
		app.ServiceList = []Service{}

		APP = &app
	}
	return APP
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

func (app *App) UpdateResultList(result Result) {
	rs := app.State.ResultList.Payload.([]Result)
	rs = append(rs, result)
	app.State.ResultList.Payload = rs
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
func ResetState() {
	APP = nil
}

func (a App) FunctionList() []Fn {
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
	return GetApp()
}
