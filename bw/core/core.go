package core

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	clone "github.com/huandu/go-clone/generic"
)

// ---

func DebugRes(prefix string, idx int, result Result) {
	if result.ParentID == "" {
		fmt.Printf("%s[%v] id:%v parent:nil\n", prefix, idx, result.ID)
	} else {
		fmt.Printf("%s[%v] id:%v parent:%s\n", prefix, idx, result.ID, result.ParentID)
	}
}

func DebugResList(prefix string, result_list []Result) {
	fmt.Println("---")
	for i, r := range result_list {
		if i > 300 {
			break
		}
		DebugRes(prefix, i, r)
	}
	fmt.Println("---")
}

//

// a simple classifier for things, rendered as "major/minor/type".
// for example: "os/fs/file" to represent a 'file' provided via the 'fs' service grouped under the 'os' provider.
// or "git/repository/repo" is a 'git' provider offering a 'repository' service' that yields 'repo' types.
type NS struct {
	Major string // provider
	Minor string // service
	Type  string // type
}

func (ns NS) String() string {
	return fmt.Sprintf("%s/%s/%s", ns.Major, ns.Minor, ns.Type)
}

func NewNS(major string, minor string, ttype string) NS {
	return NS{Major: major, Minor: minor, Type: ttype}
}

// ----

// simple key+val. val can be anything.
type KeyVal struct {
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
	ArgList []KeyVal
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
			slog.Error("recovered from service function panic", "fn", fn, "panic", r)
			fmt.Println(string(debug.Stack()))
			result = FnResult{Err: errors.New("panicked")}
		}
	}()
	result = fn.TheFn(app, args)
	return result
}

func NewFnArgs(key string, val any) FnArgs {
	return FnArgs{ArgList: []KeyVal{{Key: key, Val: val}}}
}

// ---

type Result struct {
	ID               string `json:"id"`
	NS               NS     `json:"ns"`
	Item             any    `json:"item"`
	ParentID         string `json:"parent-id"`
	ChildrenRealised bool
}

func _realise_children(app *App, result Result, load_child_policy ITEM_CHILDREN_LOAD) []Result {
	empty := []Result{}

	// item is missing! could be a dummy row or bad programming
	if result.Item == nil {
		return empty
	}

	// this is a recursive function and at this level we've been told to stop, so stop.
	if load_child_policy == ITEM_CHILDREN_LOAD_FALSE {
		// policy is set to do-not-load.
		// do not descend any further.
		return empty
	}

	// work already done.
	if result.ChildrenRealised {
		return empty
	}

	// don't know what it is, but it can't have children.
	if !HasItemInfo(result.Item) {
		return empty
	}

	//fmt.Println("realising children", parent.ID, parent.NS)

	var children []Result
	item_as_row := result.Item.(ItemInfo)
	parent__load_child_policy := item_as_row.ItemHasChildren()

	//fmt.Println("function policy:", load_child_policy, ", parent policy:", parent__load_child_policy)

	if load_child_policy == "" {
		load_child_policy = ITEM_CHILDREN_LOAD_TRUE
	} else {
		load_child_policy = parent__load_child_policy
	}

	// parent explicitly has no children to load,
	// short circuit. do not bother looking for them.
	if load_child_policy == ITEM_CHILDREN_LOAD_FALSE {
		return empty
	}

	// parent has lazy or eager children,
	// either way, load them

	if load_child_policy == ITEM_CHILDREN_LOAD_LAZY {
		return empty // 2024-07-21 - something amiss here
	}

	for _, child := range item_as_row.ItemChildren(app) {
		child.ParentID = result.ID

		if load_child_policy == ITEM_CHILDREN_LOAD_TRUE {
			grandchildren := _realise_children(app, child, load_child_policy)
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

func realise_children(app *App, result ...Result) []Result {
	slog.Debug("realising children", "num-results", len(result)) //, "rl", result)

	child_list := []Result{}
	for _, r := range result {
		children := _realise_children(app, r, "")
		r.ChildrenRealised = true
		child_list = append(child_list, r)
		child_list = append(child_list, children...)
	}

	slog.Debug("done realising children", "num-results", len(child_list), "results", child_list)

	return child_list
}

// returns a `Result` struct's list of child results.
// returns an error if the children have not been realised yet and there is no childer loader fn.
func Children(app *App, result Result) ([]Result, error) {
	if !result.ChildrenRealised {
		slog.Info("children not realised")
		children := realise_children(app, result)
		app.SetResults(children...)
	}

	foo := app.FilterResultList(func(r Result) bool {
		return r.ParentID == result.ID
	})

	return foo, nil
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
	index map[string]int

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

type App struct {
	update_chan AppUpdateChan
	lock        sync.Mutex
	atomic      sync.Mutex
	IApp
	state        *State    // state not exported. access state with GetState, update with UpdateState
	ServiceList  []Service // todo: rename provider list
	ListenerList []Listener2
}

func NewState() *State {
	state := State{}
	state.Root = Result{NS: NS{}, Item: []Result{}}
	state.index = map[string]int{}
	state.KeyVals = map[string]any{
		"bw.app.name":    "bw",
		"bw.app.version": "0.0.1",
	}
	return &state
}

func NewApp() *App {
	app := App{}
	app.update_chan = make(chan func(State) State, 100)
	app.state = NewState()
	app.ServiceList = []Service{}
	app.ListenerList = []Listener2{}
	return &app
}

// returns a copy of the app state.
// see also `StatePTR`
func (app *App) State() State {
	return *app.state
}

// accessor for the app.state.
// I want:
// 1. to control access to the state to avoid updates bypassing app.UpdateState
// 2. not copy the state if I don't have to. premature optimisation?
func (app *App) StatePTR() *State {
	return app.state
}

// returns a copy of the app state
func (app *App) StateRoot() []Result {
	return app.state.Root.Item.([]Result)
}

// ---

// caveats: no support for state.keyvals yet
// does it even work??

type Listener2 struct {
	ID                string
	ReducerFn         func(Result) bool
	CallbackFn        func(old_results []Result, new_results []Result)
	WrappedCallbackFn func([]Result)
}

// calls each `Listener.ReducerFn` in `listener_list` on each item in the state,
// before finally calling each `Listener.CallbackFn` on each listener's list of filtered results.
func process_listeners(new_state State, listener_list []Listener2) []Listener2 {

	var listener_list_results = make([][]Result, len(listener_list))
	//listener_list_results := [][]Result{}

	// for each result in new state, apply every listener.reducer to it.
	// we could do N passes of the result list or we could do 1 pass of the result list with N iterations over the same item.
	// N passes over the result list lends itself to parallelism, N passes over an item is simpler for sequential access.
	for _, result := range new_state.Root.Item.([]Result) {
		for listener_idx, listener_struct := range listener_list {
			//slog.Debug("calling ReducerFn", "listener", listener_struct.ID)
			reducer_results := listener_list_results[listener_idx]
			if listener_struct.ReducerFn(result) {
				reducer_results = append(reducer_results, result)
			}
			listener_list_results[listener_idx] = reducer_results
		}
	}

	// call each listener callback with it's new set of results

	empty_results := []Result{}

	updated_listener_list := []Listener2{}
	for idx, listener_results := range listener_list_results {
		listener_results := listener_results
		listener := listener_list[idx]

		slog.Debug("calling listener with new results", "listener", listener.ID, "num-results", len(listener_results))

		if listener.WrappedCallbackFn == nil {
			// first time! no old results to compare to, call the listener
			slog.Debug("no wrapped callback for listener, calling listener for first time", "listener", listener.ID, "num-results", len(listener_results))
			listener.CallbackFn(empty_results, listener_results)

		} else {
			// listener has been called before.
			// todo: only call the original function if the results have changed
			slog.Debug("wrapped callback exists, calling that", "listener", listener.ID, "num-results", len(listener_results))
			listener.WrappedCallbackFn(listener_results)
		}

		// set/update the wrapped callback function using the current listener results
		listener.WrappedCallbackFn = func(old_results []Result) func(new_results []Result) {
			// note! the canonical form of a pointer is a pointer and *not* it's dereferenced value!
			// if a value isn't being detected as having changed, you might be using a pointer ...
			//old_results := listener_results
			return func(new_results []Result) {
				if reflect.DeepEqual(old_results, new_results) { // if there are any functions this will always be true
					slog.Debug("wrapped listener, not calling, old results and new results are identical", "id", listener.ID)
					//slog.Info("old and new", "old", old_results, "new", new_results)
				} else {
					slog.Debug("wrapped listener, calling, new results different to old results", "id", listener.ID)
					listener.CallbackFn(old_results, new_results)
				}
			}
		}(listener_results)

		updated_listener_list = append(updated_listener_list, listener)
	}

	return updated_listener_list
}

// ---

// returns a simple map of Result.ID => pos for all 'top-level' results.
func results_list_index(results_list []Result) map[string]int {
	slog.Info("rebuilding index")

	idx := map[string]int{}
	for i, res := range results_list {
		res := res
		idx[res.ID] = i
	}
	return idx
}

type AppUpdateChan chan func(State) State

// processes a single pending state update,
// calling `fn`, modifying `app`, and executing it's list of listeners
func (app *App) ProcessUpdate() {
	fn := <-app.update_chan

	app.atomic.Lock()
	defer app.atomic.Unlock()

	old_state := *app.state
	new_state := fn(old_state)

	app.state = &new_state
	app.state.index = results_list_index(app.state.Root.Item.([]Result))
	app.ListenerList = process_listeners(*app.state, app.ListenerList)
}

// pulls state updates off of app's internal update channel,
// processes it and then repeats, forever.
func (app *App) ProcessUpdates() {
	for {
		app.ProcessUpdate()
	}
}

// update a single result with a specific ID.
// the ID can't change.
// the parent can't change.
// children are not realised.
func (app *App) UpdateResult(someid string, xform func(x Result) Result) *sync.WaitGroup {

	var wg sync.WaitGroup
	wg.Add(1)
	fn := func(state State) State {
		defer wg.Done()

		result_idx, present := state.index[someid]
		if !present {
			slog.Warn("could not update result, result not found", "id", someid)
			return state
		}

		original := app.state.Root.Item.([]Result)[result_idx]
		clone := clone.Clone(original)
		someval := xform(clone)

		slog.Info("updating result with new values", "id", someid, "oldval", original, "newval", someval)
		state.Root.Item.([]Result)[result_idx] = someval

		return state
	}

	app.update_chan <- fn

	return &wg
}

// update the app state by applying a function to a copy of the current state,
// returning the new state to be set.
func (app *App) UpdateState(fn func(old_state State) State) *sync.WaitGroup {

	// problem is here:
	// we are updating state,
	// the listeners are called after state is updated,
	// the listeners are calling logic that is updating state,

	// possible solution: update state works at a distance,
	// pulling update requests off of a channel,
	// processing them,
	// calling the listeners,
	// the listeners will attempt to update state,
	// which puts requests on to the channel,
	//etc

	// the important thing is that we keep a shallow stack,
	// and process the updates and call the listeners in a predictable and deterministic manner

	//slog.Info("UpdateState, acquiring lock ...")
	//app.lock.Lock()
	//defer app.lock.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	app.update_chan <- func(state State) State {
		defer wg.Done()
		return fn(state)
	}

	return &wg

}

func (app *App) AtomicUpdates(fn func()) {
	app.atomic.Lock()
	defer app.atomic.Unlock()
	fn()
}

func (app *App) AddListener(new_listener Listener2) {
	slog.Info("adding listener", "id", new_listener.ID)
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
	for _, child := range app.StateRoot() {
		_children = append(_children, realise_children(app, child)...)
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
	return app.StatePTR().KeyAnyVal(key)
}

// convenience. see `state.KeyVal`.
func (app *App) KeyVal(key string) string {
	return app.StatePTR().KeyVal(key)
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

func (app *App) SetKeyAnyVals(root string, keyvals map[string]any) *sync.WaitGroup {
	if root != "" {
		root += "."
	}
	return app.UpdateState(func(old_state State) State {
		for key, val := range keyvals {
			old_state.KeyVals[root+key] = val
		}
		return old_state
	})
}

func (app *App) SetKeyVals(root string, keyvals map[string]string) *sync.WaitGroup {
	// urgh. no other way to go from map[string]string => map[string]any ?
	kva := make(map[string]any, len(keyvals))
	for k, v := range keyvals {
		kva[k] = v
	}
	return app.SetKeyAnyVals(root, kva)
}

func (app *App) SetKeyVal(key string, val any) *sync.WaitGroup {
	return app.SetKeyAnyVals("", map[string]any{key: val})
}

func (app *App) Set(key string, val any) *sync.WaitGroup {
	return app.SetKeyVal(key, val)
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
		extant, present := state.index[r.ID]
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
func (app *App) AddResults(result_list ...Result) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		return add_result(old_state, realise_children(app, result_list...)...)
	})
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be replaced by the new item.
func (app *App) SetResults(result_list ...Result) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		return add_replace_result(old_state, realise_children(app, result_list...)...)
	})
}

// removes all results where `filter_fn(result)` is true
// todo: remove children of results removed
func (app *App) RemoveResults(filter_fn func(Result) bool) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
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

// find first result whose NS equals the given `ns`.
// good for known singletons I suppose.
// todo: candidate for replacement.
func (app *App) FilterResultListByNSToResult(ns NS) Result {
	for _, result := range app.state.Root.Item.([]Result) {
		if result.NS == ns {
			return result
		}
	}
	return Result{}
}

// returns a result by it's ID, returning nil if not found
func (app *App) GetResult(id string) *Result {
	// necessary?
	//app.atomic.Lock()
	//defer app.atomic.Unlock()

	// find the result by sequentially going through results
	// this helped debug an issue with the index for a time.
	/*
		for _, r := range app.state.Root.Item.([]Result) {
			if r.ID == id {
				return &r
			}
		}

		return nil
	*/

	idx, present := app.state.index[id]
	if !present {
		return nil
	}
	return &app.state.Root.Item.([]Result)[idx]
}

/*
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
	_, present := app.state.index[id]
	return present
}

// find first result rooted in `result` (including `result`) whose ID matches `id`.
// recursive, naive and expensive.
func find_result_by_id1(result Result, id string) Result {
	if EmptyResult(result) {
		return result
	}

	if result.ID == id {
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

// find first result rooted in `result` (including `result`) whose ID matches `id`.
// assumes the result's Item is a []Result.
func find_result_by_id2(result Result, id string) Result {
	if result.ID == id {
		return result
	}

	empty_result := Result{}

	rl, is_rl := result.Item.([]Result)
	if !is_rl {
		return empty_result
	}

	for _, r := range rl {
		if r.ID == id {
			return r
		}
	}
	return empty_result
}

var find_result_by_id = find_result_by_id2

func (app *App) FindResultByID(id string) Result {
	return find_result_by_id(app.state.Root, id)
}

// find all results whose ID is in `id_list`
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

// find the top-most root result for the given id
func (app *App) FindRootResult(id string) *Result {
	var res Result
	original_id := id
	for {
		res = app.FindResultByID(id)
		if EmptyResult(res) {
			slog.Info("failed to find parent")
			return nil
		}
		if res.ParentID == "" {
			slog.Info("found top-most parent of id", "id", original_id, "root", res)
			return &res
		} else {
			id = res.ParentID
		}
		slog.Debug("looping")
	}
}

// find the top-most root result for the given id
func (app *App) FindParents(id string) []Result {
	var res Result
	original_id := id
	parent_list := []Result{}
	for {
		res = app.FindResultByID(id)
		if EmptyResult(res) {
			return parent_list
		}
		if id != original_id {
			// exclude given `id`
			parent_list = append(parent_list, res)
		}
		if res.ParentID == "" {
			return parent_list
		}
		id = res.ParentID
		slog.Debug("looping")
	}
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

// a 'view' (tab) needs to filter the results it returns.
type ViewFilter func(Result) bool

// ---

var START_PROVIDER_SERVICE = "Start Provider"
var STOP_PROVIDER_SERVICE = "Stop Provider"

func StartProviderService(thefn func(*App, FnArgs) FnResult) Fn {
	return Fn{
		Label:       START_PROVIDER_SERVICE,
		Description: "Initialises the provider, called during provider registration, should be idempotent",
		Interface:   FnInterface{}, // accepts no further args
		TheFn:       thefn,
	}
}

func StopProviderService(thefn func(*App, FnArgs) FnResult) Fn {
	return Fn{
		Label:       STOP_PROVIDER_SERVICE,
		Description: "Stops the provider, called during provider cleanup, should be idempotent",
		Interface:   FnInterface{}, // accepts no further args
		TheFn:       thefn,
	}
}

type Provider interface {
	// a list of services that this Provider provides.
	ServiceList() []Service
}

func (a *App) RegisterProvider(p Provider) {
	for _, service := range p.ServiceList() {
		a.RegisterService(service)
	}
}

// initialisation hook for providers.
// if a provider has a registered service with the name `core.START_PROVIDER_SERVICE`
// it will be called here.
func (a *App) StartProviders() {
	slog.Debug("starting providers", "num-providers", len(a.ServiceList))
	for idx, service := range a.ServiceList {
		slog.Debug("starting provider", "num", idx, "provider", service)
		for _, service_fn := range service.FnList {
			if service_fn.Label == START_PROVIDER_SERVICE {
				service_fn.TheFn(a, FnArgs{})
			}
		}
	}
}

// a shutdown hook for providers
func (a *App) StopProviders() {
	slog.Debug("cleaning up providers")

	// todo: reverse order

	for _, service := range a.ServiceList {
		for _, service_fn := range service.FnList {
			if service_fn.Label == STOP_PROVIDER_SERVICE {
				service_fn.TheFn(a, FnArgs{})
			}
		}
	}

}

func Start() *App {
	app := NewApp()
	app.Set("bw.app.name", "bw")
	app.Set("bw.app.version", "0.0.1")
	slog.Info("app started", "app", app)

	go app.ProcessUpdates()

	return app
}
