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

func (ns NS) String() string {
	return fmt.Sprintf("%s/%s/%s", ns.Major, ns.Minor, ns.Type)
}

func NewNS(major string, minor string, ttype string) NS {
	return NS{Major: major, Minor: minor, Type: ttype}
}

// ----

// simple key+val. val can be anything.
type Arg struct {
	Key string
	Val interface{}
}

// the payload a service function must return
type FnResult struct {
	Err    error    `json:",omitempty"`
	Result []Result `json:",omitempty"`
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

func NewFnResult(result ...Result) FnResult {
	return FnResult{Result: result}
}

func EmptyFnResult(fr FnResult) bool {
	return len(fr.Result) == 0
}

func NewErrorFnResult(err error, msg string) FnResult {
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

func AsFnArgs(id string, someval interface{}) FnArgs {
	return FnArgs{ArgList: []Arg{{Key: id, Val: someval}}}
}

// ---

type TableRow interface {
	// returns a list of fields available to the table in their preferred order.
	RowKeys() []string
	// returns a map of fields to their stringified values.
	RowMap() map[string]string
	// returns a list of child rows for this row, if any
	RowChildren() []Result
}

type Result struct {
	ID   string `json:"id"`
	NS   NS     `json:"ns"`
	Item any    `json:"item"`
}

func EmptyResult(r Result) bool {
	return r.ID == ""
}

func NewResult(ns NS, item any, id string) Result {
	return Result{
		ID:   id,
		NS:   ns,
		Item: item,
	}
}

// the application's moving parts.
type State struct {
	//                     bw: config: no-colour: "true"
	KeyVals map[string]map[string]map[string]string
	Root    Result
	Index   map[string]*Result
}

type IApp interface {
	RegisterService(service Service)
	AddResult(result Result)
	GetResult(name string) *Result
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
	// {major: minor: key: val}
	state.KeyVals = map[string]map[string]map[string]string{
		"bw": {"app": {"name": "bw", "version": "0.0.1"}},
	}
	state.Root = Result{NS: NS{}, Item: []Result{}}
	state.Index = map[string]*Result{}
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
	}
}

func (app *App) SetKeyVal(major string, minor string, key string, val string) {
	app.SetKeyVals(major, minor, map[string]string{key: val})
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

func add_result_to_state(state *State, replace bool, result_list ...Result) *State {
	if state == nil {
		return state
	}
	if len(result_list) == 0 {
		return state
	}
	root := state.Root.Item.([]Result)

	/*
		// clever, but nothing needs a flat index right now.
		var index func(result *Result)
		index = func(result *Result) {
			result_list, is_result_list := result.Item.([]Result)
			if !is_result_list {
				state.Index[result.ID] = result
			}
		        for _, sub_result := range result_list {
				index(&sub_result)
			}
		}
	*/
	index := func(result *Result) {
		state.Index[result.ID] = result
	}

	for _, result := range result_list {
		result := result
		_, in_idx := state.Index[result.ID]
		if in_idx && replace {
			// a thing with this id already exists and we want to replace them.
			// find it's memory address and replace what it is pointing to
			old_result_ptr := state.Index[result.ID]
			*old_result_ptr = result

			// index the new thing
			index(&result)
			continue
		}

		// item not in index or we're not replacing items,
		// append it to the list of results and (possibly) overwrite anything with that id in the index.
		root = append(root, result)
		index(&result)
	}

	state.Root.Item = root

	return state
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be duplicated.
func (app *App) AddResult(result_list ...Result) {
	replace := false
	app.State = add_result_to_state(app.State, replace, result_list...)
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be replaced in-place by the new item.
func (app *App) SetResult(result_list ...Result) {
	replace := true
	app.State = add_result_to_state(app.State, replace, result_list...)
}

func (app *App) ResultList() []Result {
	return app.State.Root.Item.([]Result)
}

// returns a list of results where `filter_fn(result)` is true
func (app *App) FilterResultList(filter_fn func(Result) bool) []Result {
	result_list := []Result{}
	for _, result := range app.State.Root.Item.([]Result) {
		if filter_fn(result) {
			result_list = append(result_list, result)
		}
	}
	return result_list
}

// applies `fn` to a result's pointer, presumably for side effects.
func (app *App) RunResultPtr(fn func(*Result)) {
	for _, result := range app.State.Root.Item.([]Result) {
		result := result
		fn(&result)
	}
}

func ItemList[T any](result_list ...Result) []T {
	t_list := []T{}
	for _, res := range result_list {
		t_list = append(t_list, res.Item.(T))
	}
	return t_list
}

func (app *App) FilterResultListByNS(ns NS) []Result {
	result_list := []Result{}
	for _, result := range app.State.Root.Item.([]Result) {
		if result.NS == ns {
			result_list = append(result_list, result)
		}
	}
	return result_list
}

// removes all results where `filter_fn(result)` is true
func (app *App) RemoveResultList(filter_fn func(Result) bool) {
	result_list := []Result{}
	for _, result := range app.State.Root.Item.([]Result) {
		if !filter_fn(result) {
			result_list = append(result_list, result)
		}
	}
	app.State.Root.Item = result_list
}

// gets a result by it's ID, returning nil if not found
func (app *App) GetResult(id string) *Result {
	// acquire lock
	result_ptr, present := app.State.Index[id]
	if !present {
		return nil
	}
	return result_ptr
}

// returns `true` if a result with the given `id` is present in state.
func (app *App) HasResult(id string) bool {
	_, present := app.State.Index[id]
	return present
}

// recursive.
// I imagine it's going to be very easy to create infinite recursion with pointers ...
func find_result_by_id(result Result, id string) Result {
	if EmptyResult(result) {
		return result
	}

	if result.ID == id {
		//common.Stderr(fmt.Sprintf("found match: %s\n", common.QuickJSON(result)))
		return result
	}

	switch t := result.Item.(type) {
	case Result:
		// we have a Result.Result, recurse
		return find_result_by_id(t, id)

	case []Result:
		// we have a Result.[]Result, recurse on each
		for _, r := range t {
			rr := find_result_by_id(r, id)
			if EmptyResult(rr) {
				continue
			}
			// match! return what was found
			return rr
		}

	default:
		//stderr(fmt.Sprintf("can't inspect Result.Payload of type: %T\n", t))
	}

	return Result{}
}

func (app *App) FindResultByID(id string) Result {
	return find_result_by_id(app.State.Root, id)
}

func (app *App) RegisterService(service Service) {
	app.ServiceList = append(app.ServiceList, service)
}

// ---------

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
