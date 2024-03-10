package core

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
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

type ITEM_CHILDREN_LOAD string

const (
	ITEM_CHILDREN_LOAD_TRUE  ITEM_CHILDREN_LOAD = "load"
	ITEM_CHILDREN_LOAD_FALSE ITEM_CHILDREN_LOAD = "do-not-load"
	ITEM_CHILDREN_LOAD_LAZY  ITEM_CHILDREN_LOAD = "lazy-load"
)

// an interface Result.Items can implement to get lazy nested results
type ItemInfo interface {
	// returns a list of fields available to the table in their preferred order.
	ItemKeys() []string
	// returns a map of fields to their stringified values.
	ItemMap() map[string]string
	// returns true if a row *could* have children.
	ItemHasChildren() ITEM_CHILDREN_LOAD
	// returns a list of child rows for this row, if any
	ItemChildren() []Result // has to be a Result so a unique ID+NS can be set :( it would be more natural if a thing could just yield child-things and we wrap them in a Result later. Perhaps instead of Result.Item == any, it equals 'Item' that has a method ID() and NS() ?
}

func HasItemInfo(thing any) bool {
	table_row_interface := reflect.TypeOf((*ItemInfo)(nil)).Elem()
	return reflect.TypeOf(thing).Implements(table_row_interface)
}

type Result struct {
	ID               string  `json:"id"`
	NS               NS      `json:"ns"`
	Item             any     `json:"item"`
	Parent           *Result `json:"-"`
	ChildrenRealised bool
}

func realise_children(result Result) []Result {
	children := _realise_children(result, "")
	result.ChildrenRealised = true
	return append(children, result)
}

func _realise_children(parent Result, load_child_policy ITEM_CHILDREN_LOAD) []Result {

	empty := []Result{}

	// item is missing! could be a dummy row or bad programming
	if parent.Item == nil {
		return empty
	}

	// this is a recursive function and at this level we've been told to stop, so stop.
	if load_child_policy == ITEM_CHILDREN_LOAD_FALSE {
		// policy is set to do-not-load.
		// do not descend any further.
		return empty
	}

	// work already done.
	if parent.ChildrenRealised {
		return empty
	}

	// don't know what it is, but it can't have children.
	if !HasItemInfo(parent.Item) {
		return empty
	}

	//fmt.Println("realising children", parent.ID, parent.NS)

	var children []Result
	item_as_row := parent.Item.(ItemInfo)
	parent__load_child_policy := item_as_row.ItemHasChildren()

	//fmt.Println("function policy:", load_child_policy, ", parent policy:", parent__load_child_policy)

	if load_child_policy == "" {
		load_child_policy = ITEM_CHILDREN_LOAD_TRUE
	} else {
		load_child_policy = parent__load_child_policy
	}

	//fmt.Println("final policy", load_child_policy)

	// parent explicitly has no children to load,
	// short circuit. do not bother looking for them.
	if load_child_policy == ITEM_CHILDREN_LOAD_FALSE {
		return empty
	}

	// parent has lazy or eager children,
	// either way, load them

	if load_child_policy == ITEM_CHILDREN_LOAD_LAZY {
		return empty
	}

	for _, child := range item_as_row.ItemChildren() {
		child.Parent = &parent

		//if function__load_child_policy == ITEM_CHILDREN_LOAD_TRUE {
		if load_child_policy == ITEM_CHILDREN_LOAD_TRUE {
			grandchildren := _realise_children(child, load_child_policy)
			children = append(children, grandchildren...)
			// a result cannot be said to be realised until all of it's descendants are realised.
			// if we try to realise a result's children, and it returns grandchildren, then we know
			// they have been realised.
			// this check only works *here* if the parent policy is "lazy" and this section is skipped altogether.
			if len(grandchildren) != 0 {
				child.ChildrenRealised = true
			}
		} else {
			//fmt.Println("skipping grandchildren, policy is:", load_child_policy)
			// no, because the parent policy is LAZY at this point.
			//child.ChildrenRealised = true
		}
		children = append(children, child)
	}

	// else, load_children = lazy, do not descend any further

	return children
}

// returns a `Result` struct's list of child results.
// returns an error if the children have not been realised yet and there is no childer loader fn.
func Children(app *App, result Result) ([]Result, error) {

	if !result.ChildrenRealised {
		children := realise_children(result)
		app.SetResults(children...) // todo: does this work??
	}

	return app.FilterResultList(func(r Result) bool {
		return r.Parent != nil && r.Parent.ID == result.ID
	}), nil
}

func EmptyResult(r Result) bool {
	return r == Result{}
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
	Root Result `json:"-"`
	// a map of Result.IDs to Result pointers wasn't working as I expected:
	// - https://utcc.utoronto.ca/~cks/space/blog/programming/GoSlicesVsPointers
	//Index map[string]*Result
	// instead, Index is now a simple indicator if a result exists
	//Index map[string]bool

	// index is now a literal index into the Root.Item.([]Result) list
	Index map[string]int

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
	state       *State // state not exported. access state with GetState, update with UpdateState
	ServiceList []Service
	//ListenerList []Listener
	ListenerList []Listener2
}

func NewState() *State {
	state := State{}
	state.Root = Result{NS: NS{}, Item: []Result{}}
	state.Index = map[string]int{}
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
	app.ListenerList = []Listener2{}
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

// returns a simple map of Result.ID => pos for all 'top-level' results.
func results_list_index(results_list []Result) map[string]int {
	idx := map[string]int{}
	for i, res := range results_list {
		res := res
		idx[res.ID] = i
	}
	return idx
}

// update the app state by applying a function to a copy of the current state,
// returning the new state to be set.
func (app *App) UpdateState(fn func(old_state State) State) {
	app.lock.Lock()
	defer app.lock.Unlock()

	slog.Debug("updating state")

	num_old_listeners := len(app.ListenerList)
	old_state := *app.state
	new_state, updated_listener_list := update_state2(old_state, fn, app.ListenerList)
	if num_old_listeners != len(updated_listener_list) {
		panic(fmt.Sprintf("programming error, the number of updated listeners returned by update_state2 (%v) does not equal the number of listeners passed in (%v)", num_old_listeners, len(updated_listener_list)))
	}

	app.state = new_state

	// callbacks in listeners may add new listeners to the app,
	// so there may in fact be *more* listeners after a call to `update_state2`.
	// doing this would wipe them out:
	//app.ListenerList = updated_listener_list

	// instead, all of the updated_listeners must stay,
	// and any listeners in current app state not present in the updated_listeners must be preserved.

	updated_listener_idx := map[string]Listener2{}
	for _, listener := range updated_listener_list {
		updated_listener_idx[listener.ID] = listener
	}

	ll := []Listener2{}
	for _, listener := range app.ListenerList {
		updated_listener, present := updated_listener_idx[listener.ID]
		if !present {
			slog.Debug("new listener detected")
			ll = append(ll, listener)
		} else {
			ll = append(ll, updated_listener)
		}
	}

	app.ListenerList = ll

	app.state.Index = results_list_index(app.state.Root.Item.([]Result))
}

func (app *App) AddListener(new_listener Listener2) {
	slog.Debug("adding listener", "id", new_listener.ID)
	app.ListenerList = append(app.ListenerList, new_listener)
}

// an empty update.
// used in the UI for initial population of widgets.
func (app *App) KickState() {
	slog.Debug("kicking state")
	app.UpdateState(func(s State) State {
		return s
	})
}

// an empty update.
// used in the UI for initial population of widgets.
func (app *App) RealiseAllChildren() {

	// would be nice, but root doesn't implement ItemInfo
	//children := realise_children(&app.state.Root, ITEM_CHILDREN_LOAD_TRUE)

	_children := []Result{}
	for _, c := range app.StateRoot() {
		_children = append(_children, realise_children(c)...)
	}
	app.SetResults(_children...)
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
func (app *App) KeyAnyVal(key string) any {
	return app.State().KeyAnyVal(key)
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

	new_results = append(new_results, result_list...)

	state.Root.Item = new_results
	return state
}

func add_result(state State, result_list ...Result) State {
	if len(result_list) == 0 {
		return state
	}

	for _, r := range result_list {
		extant, present := state.Index[r.ID]
		if present {
			slog.Error("refusing to add result(s), an item with that ID already exists", "id", r.ID, "extant", extant, "new", r)
			return state
		}
	}

	root := state.Root.Item.([]Result)
	root = append(root, result_list...)

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

func FilterResultList(result_list []Result, filter_fn func(Result) bool) []Result {
	new_result_list := []Result{}
	for _, result := range result_list {
		if filter_fn(result) {
			new_result_list = append(new_result_list, result)
		}
	}

	sort.Slice(new_result_list, func(i, j int) bool {
		return new_result_list[i].ID < new_result_list[j].ID
	})

	return new_result_list
}

// returns a list of results where `filter_fn(result)` is true
func (app *App) FilterResultList(filter_fn func(Result) bool) []Result {
	return FilterResultList(app.state.Root.Item.([]Result), filter_fn)
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
	idx, present := app.state.Index[id]
	if !present {
		return nil
	}
	return &app.state.Root.Item.([]Result)[idx]
}

/* unused
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
*/
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

// really expensive and naive. optimise
func (app *App) FindResultByIDList(id_list []string) []Result {
	result_list := []Result{}
	for _, id := range id_list {
		r := find_result_by_id(app.state.Root, id)
		if !EmptyResult(r) {
			result_list = append(result_list, r)
		}
	}
	return result_list
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
