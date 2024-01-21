package core

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	Val any
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
type PredicateFn func(any) error

// takes a string and returns a value with the intended type
type ParseFn func(*App, string) (any, error)

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

func AsFnArgs(id string, someval any) FnArgs {
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
	Root Result
	// a map of Result.IDs to Result pointers wasn't working as I expected:
	// - https://utcc.utoronto.ca/~cks/space/blog/programming/GoSlicesVsPointers
	//Index map[string]*Result
	// instead, Index is now a simple indicator if a result exists
	Index map[string]bool

	// maps-in-structs are still refs and require a copy so lets not make this difficult.
	//KeyVals map[string]map[string]map[string]string
	KeyVals map[string]any
}

type IApp interface {
	/*
		RegisterService(service Service)
		AddResults(result ...Result)
		GetResult(name string) *Result
		FunctionList() ([]Fn, error)
		ResetState()
	*/
}

// a listener fn is called with the old and new `State` structs whenever the state changes.
type Listener func(State, State)

type App struct {
	lock sync.Mutex
	IApp
	state        *State // state not exported. access state with GetState, update with UpdateState
	ServiceList  []Service
	ListenerList []Listener
}

func NewState() *State {
	state := State{}
	state.Root = Result{NS: NS{}, Item: []Result{}}
	state.Index = map[string]bool{}
	state.KeyVals = map[string]any{
		"bw.app.name":    "bw",
		"bw.app.version": "0.0.1",
	}
	return &state
}

func copy_state(s State) State {
	new_state := NewState()
	new_state.Root = s.Root

	for key, val := range s.Index {
		new_state.Index[key] = val
	}

	for key, val := range s.KeyVals {
		new_state.KeyVals[key] = val
	}

	return *new_state
}

func NewApp() *App {
	app := App{}
	app.state = NewState()
	app.ServiceList = []Service{}
	app.ListenerList = []Listener{}
	return &app
}

// returns a copy of the app state
func (app *App) State() State {
	return *app.state
}

// returns a copy of the app state
func (app *App) StateRoot() []Result {
	return app.state.Root.Item.([]Result)
}

func ReIndex(state State) map[string]bool {
	idx := map[string]bool{}
	for _, res := range state.Root.Item.([]Result) {
		res := res
		idx[res.ID] = true
	}
	return idx
}

// update the app state by applying a function to a copy of the current state,
// returning the new state to be set.
func (app *App) UpdateState(fn func(old_state State) State) {
	app.lock.Lock()
	defer app.lock.Unlock()
	old_state := *app.state

	old_state_copy := copy_state(old_state)
	new_state := fn(old_state_copy)

	app.state = &new_state

	// not needed right now
	//app.state.ReIndex()

	for _, listener_fn := range app.ListenerList {
		listener_fn(old_state, new_state)
	}
}

func (app *App) AddListener(fn Listener) {
	app.ListenerList = append(app.ListenerList, fn)
}

// an empty update.
// used in the UI for initial population of widgets.
func (app *App) KickState() {
	app.lock.Lock()
	defer app.lock.Unlock()
	dummy_old_state := NewState()
	for _, listener_fn := range app.ListenerList {
		listener_fn(*dummy_old_state, *app.state)
	}
}

// returns the value stored for the given `key` as a string.
// returns an empty string if the value doesn't exist.
// returns an empty string if the value stored isn't a string.
func (state State) KeyVal(key string) string {
	val, present := state.KeyVals[key]
	if !present {
		return ""
	}
	str, isstr := val.(string)
	if !isstr {
		return ""
	}
	return str
}

// returns the value stored for the given `key`.
// return nil if the value doesn't exist.
func (state State) KeyAnyVal(key string) any {
	val, present := state.KeyVals[key]
	if !present {
		return nil
	}
	return val
}

// convenience. see `state.KeyVal`.
func (app *App) KeyVal(key string) string {
	return app.State().KeyVal(key)
}

// returns a subset of `state.KeyVals` for all keys starting with given `prefix`.
// `state.KeyVals` contains mixed typed values so use with caution!
func (state State) SomeKeyVals(prefix string) map[string]any {
	subset := make(map[string]any)
	for key, val := range state.KeyVals {
		if strings.HasPrefix(key, prefix) {
			subset[key] = val
		}
	}
	return subset
}

// convenience. see `state.SomeKeyVals`.
func (app *App) SomeKeyVals(prefix string) map[string]any {
	return app.state.SomeKeyVals(prefix)
}

func (app *App) SetKeyAnyVals(root string, keyvals map[string]any) {
	if root != "" {
		root += "."
	}
	app.UpdateState(func(old_state State) State {
		for key, val := range keyvals {
			old_state.KeyVals[root+key] = val
		}
		return old_state
	})
}

func (app *App) SetKeyVals(root string, keyvals map[string]string) {
	// urgh. no other way to go from map[string]string => map[string]any ?
	kva := make(map[string]any, len(keyvals))
	for k, v := range keyvals {
		kva[k] = v
	}
	app.SetKeyAnyVals(root, kva)
}

func (app *App) SetKeyVal(key string, val any) {
	app.SetKeyAnyVals("", map[string]any{key: val})
}

// ---

func add_replace_result(state State, result_list ...Result) State {
	if len(result_list) == 0 {
		return state
	}

	root := state.Root.Item.([]Result)

	// we have to traverse the entire result list
	new_results := []Result{}

	idx := map[string]Result{}
	for _, r := range result_list {
		idx[r.ID] = r
	}

	for _, old_result := range root {
		_, present := idx[old_result.ID]
		if !present {
			new_results = append(new_results, old_result)
		}
	}

	for _, r := range result_list {
		new_results = append(new_results, r)
		state.Index[r.ID] = true
	}

	state.Root.Item = new_results
	return state
}

func add_result(state State, result_list ...Result) State {
	if len(result_list) == 0 {
		return state
	}

	root := state.Root.Item.([]Result)

	for _, result := range result_list {
		root = append(root, result)
		state.Index[result.ID] = true
	}

	state.Root.Item = root

	return state
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be duplicated.
func (app *App) AddResults(result_list ...Result) {
	app.UpdateState(func(old_state State) State {
		return add_result(old_state, result_list...)
	})
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be replaced in-place by the new item.
func (app *App) SetResults(result_list ...Result) {
	app.UpdateState(func(old_state State) State {
		return add_replace_result(old_state, result_list...)
	})
}

// removes all results where `filter_fn(result)` is true
func (app *App) RemoveResults(filter_fn func(Result) bool) {
	app.UpdateState(func(old_state State) State {
		result_list := []Result{}
		for _, result := range app.state.Root.Item.([]Result) {
			if !filter_fn(result) {
				result_list = append(result_list, result)
			}
		}
		old_state.Root.Item = result_list
		return old_state
	})
}

func (app *App) GetResultList() []Result {
	return app.state.Root.Item.([]Result)
}

// returns a list of results where `filter_fn(result)` is true
func (app *App) FilterResultList(filter_fn func(Result) bool) []Result {
	result_list := []Result{}
	for _, result := range app.state.Root.Item.([]Result) {
		if filter_fn(result) {
			result_list = append(result_list, result)
		}
	}
	return result_list
}

// returns the item payload attached to each result in `result_list` as a slice of given `T`.
func ItemList[T any](result_list ...Result) []T {
	t_list := []T{}
	for _, res := range result_list {
		t_list = append(t_list, res.Item.(T))
	}
	return t_list
}

func (app *App) FilterResultListByNS(ns NS) []Result {
	result_list := []Result{}
	for _, result := range app.state.Root.Item.([]Result) {
		if result.NS == ns {
			result_list = append(result_list, result)
		}
	}
	return result_list
}

// gets a result by it's ID, returning nil if not found
func (app *App) GetResult(id string) *Result {
	// acquire lock ?
	_, present := app.state.Index[id]
	if !present {
		return nil
	}
	for _, r := range app.StateRoot() {
		if r.ID == id {
			return &r // todo: revisit return type.
		}
	}
	slog.Warn("index contained an ID not found in results", "id", id)
	return nil
}

// searches for a result by it's NS.
// returns nil if no results found.
// returns the first result if many found.
func (app *App) GetResultByNS(ns NS) *Result {
	// acquire lock
	for _, result := range app.state.Root.Item.([]Result) {
		if result.NS == ns {
			return &result
		}
	}
	return nil
}

// returns `true` if a result with the given `id` is present in state.
func (app *App) HasResult(id string) bool {
	_, present := app.state.Index[id]
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
	return find_result_by_id(app.state.Root, id)
}

func (app *App) RegisterService(service Service) {
	app.ServiceList = append(app.ServiceList, service)
}

// ---------

// TODO: turn this into a stop + restart thing.
// throw an error, have main.main catch it and call stop() then start()
func (a *App) ResetState() {
	a.state = NewState()
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
	app := NewApp()
	app.SetKeyVals("bw.app", map[string]string{
		"name":    "bw",
		"version": "0.0.1",
	})
	return app
}
