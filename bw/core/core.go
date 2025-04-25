package core

import (
	"bw/http_utils"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
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
	//Help string // useful?
	Key string
	Val any
}

// ---

// the payload a service function must return
type ServiceResult struct {
	Err    error    `json:",omitempty"`
	Result []Result `json:",omitempty"`
}

func NewServiceResult(result ...Result) ServiceResult {
	return ServiceResult{Result: result}
}

func (fr *ServiceResult) IsEmpty() bool {
	return len(fr.Result) == 0
}

func NewServiceResultError(err error, msg string) ServiceResult {
	// "could not load settings: file does not exist: /path/to/settings"
	return ServiceResult{Err: fmt.Errorf("%s: %w", msg, err)}
}

// ---

// the key+vals a service function must take as input.
type ServiceArgs struct {
	ArgList []KeyVal
	//Validator PredicateFn // validates sets of args
}

func NewServiceArgs(key string, val any) ServiceArgs {
	return ServiceArgs{ArgList: []KeyVal{{Key: key, Val: val}}}
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

type InputWidget = string

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
	ID            string        // "name", same requirements as a golang function
	Label         string        // "Name"
	Default       string        // value to use when input is blank. value goes through parser and validator.
	Widget        InputWidget   // type of widget to use for input. defaults to 'text input'
	Choice        *ArgChoice    // if non-nil, user's input is limited to these choices
	Parser        ParseFn       // parses user input, returning a 'normal' value or an error. string-to-int, string-to-int64, etc
	ValidatorList []PredicateFn // "required", "not-blank", "not-super-long", etc
}

// a description of a function's list of arguments.
type ServiceInterface struct {
	ArgDefList []ArgDef
}

// describes a function that accepts a FnArgList derived from a FnInterface
type Service struct {
	ServiceGroup *ServiceGroup                         // optional, the group this fn belongs to. provides context, grouping, nothing else.
	ID           string                                // unique identifier within a group of services
	Label        string                                // friendly name for this function
	Description  string                                // friendly description of this function's behaviour
	Interface    ServiceInterface                      // argument interface for this fn.
	Fn           func(*App, ServiceArgs) ServiceResult // the callable.
}

// a ServiceGroup is a natural grouping of services with a unique, descriptive, namespace `NS`
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

func CallServiceFnWithArgs(app *App, service Service, args ServiceArgs) ServiceResult {
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

// ---

type Tag = string

var (
	// I fancy I'd like to tag results with the 1,2,3 keys like I do with email sometimes
	TAG_1 Tag = "one"
	TAG_2 Tag = "two"
	TAG_3 Tag = "three"

	// provider is hinting to app that the result can be updated (somehow)
	TAG_HAS_UPDATE = "has-update"

	// provider is hinting to app that the children of this result should be shown
	TAG_SHOW_CHILDREN = "show-children"
)

type Result struct {
	ID               string `json:"id"`   // unique per *app-instance*
	NS               NS     `json:"ns"`   // simple major/minor/type categorisation
	Item             any    `json:"item"` // the payload itself
	ParentID         string `json:"parent-id"`
	ChildrenRealised bool   `json:"children-realised"` // children are lazily loaded. once loaded, they are not loaded again.
	Tags             mapset.Set[Tag]
}

func (r *Result) IsEmpty() bool {
	return r == &Result{}
}

func NewResult(ns NS, item any, id string) Result {
	return Result{
		ID:   id,
		NS:   ns,
		Item: item,
		Tags: mapset.NewSet[Tag](),
	}
}

type State struct {
	Root Result `json:"-"`

	// a literal index into the Root.Item.([]Result) result list
	index map[string]int

	// a bucket of key+vals. complete free for all state modification. be careful.
	KeyVals map[string]any

	// shared HTTP client for persistent connections.
	// see `bw.http_utils.Request`
	HTTPClient *http.Client
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
	IApp
	update_chan  AppUpdateChan
	atomic       sync.Mutex // force sequential behavior when necessary
	state        *State     // state not exported. access state with GetState, update with UpdateState
	ServiceList  []ServiceGroup
	TypeMap      map[reflect.Type][]Service // sdf
	ListenerList []Listener
}

func NewState() *State {
	state := State{}
	state.Root = Result{NS: NS{}, Item: []Result{}}
	state.index = map[string]int{} // internal map of Result.ID => state.Root.i
	state.KeyVals = map[string]any{
		"bw.app.name":    "bw",
		"bw.app.version": "0.1.0",
	}
	state.HTTPClient = &http.Client{}
	state.HTTPClient.Transport = &http_utils.FileCachingRequest{
		CWD:             "/tmp",
		UseExpiredCache: true,
	}
	return &state
}

func NewApp() *App {
	app := App{
		update_chan:  make(chan AppUpdate, 100),
		state:        NewState(),
		ServiceList:  []ServiceGroup{},
		ListenerList: []Listener{},
		TypeMap:      make(map[reflect.Type][]Service),
	}
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

type Listener struct {
	ID                string
	ReducerFn         func(Result) bool
	CallbackFn        func(old_results []Result, new_results []Result)
	WrappedCallbackFn func([]Result)
}

// calls each `Listener.ReducerFn` in `listener_list` on each item in the state,
// before finally calling each `Listener.CallbackFn` on each listener's list of filtered results.
func process_listeners(new_state State, listener_list []Listener) []Listener {

	slog.Debug("processing listeners")

	var listener_list_results = make([][]Result, len(listener_list))

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

	updated_listener_list := []Listener{}
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
	slog.Debug("rebuilding index")

	idx := map[string]int{}
	for i, res := range results_list {
		res := res
		idx[res.ID] = i
	}
	return idx
}

type AppUpdate struct {
	UpdateFn func(State) State
	Wg       *sync.WaitGroup
}

type AppUpdateChan chan AppUpdate

// processes a single pending state update,
// calling `fn`, modifying `app`, and executing it's list of listeners
func (app *App) ProcessUpdate() {
	au := <-app.update_chan

	app.atomic.Lock()
	defer app.atomic.Unlock()

	fn := au.UpdateFn
	wg := au.Wg

	wg.Add(1)
	defer wg.Done()

	old_state := *app.state
	new_state := fn(old_state) // fn's waitgroup is unlocked here

	app.state = &new_state
	app.state.index = results_list_index(app.state.Root.Item.([]Result))
	app.ListenerList = process_listeners(*app.state, app.ListenerList)

	// update's waitgroup unlocked here
}

// pulls state updates off of app's internal update channel,
// processes it and then repeats, forever.
func (app *App) ProcessUpdateLoop() {
	for {
		app.ProcessUpdate()
	}
}

// update a single result with a specific ID.
// the ID can't change.
// the parent can't change.
// children are not realised.
func (app *App) UpdateResult(someid string, xform func(Result) Result) *sync.WaitGroup {

	var wg sync.WaitGroup
	wg.Add(1)
	fn := func(state State) State {
		defer wg.Done()

		result_idx, present := state.index[someid]
		if !present {
			slog.Error("could not update result, result not found", "id", someid)
			//panic("programming error")
			return state
		}

		original := app.state.Root.Item.([]Result)[result_idx]
		clone := clone.Clone(original)
		someval := xform(clone)

		slog.Debug("updating result with new values", "id", someid, "oldval", original, "newval", someval)
		state.Root.Item.([]Result)[result_idx] = someval

		return state
	}

	au := AppUpdate{
		UpdateFn: fn,
		Wg:       &wg,
	}

	app.update_chan <- au

	return &wg
}

// update the app state by applying a function to a copy of the current state,
// returning the new state to be set.
func (app *App) UpdateState(fn func(old_state State) State) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	update_fn := func(state State) State {
		defer wg.Done()
		return fn(state)
	}

	app.update_chan <- AppUpdate{
		UpdateFn: update_fn,
		Wg:       &wg,
	}
	return &wg
}

func (app *App) AddListener(new_listener Listener) {
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

// returns a subset of `state.KeyVals` for all keys starting with given `prefix` whose values are strings.
func (state State) SomeKeyVals(prefix string) map[string]string {
	subset := make(map[string]string)
	for key, val := range state.KeyVals {
		valstr, isstr := val.(string)
		if isstr && strings.HasPrefix(key, prefix) {
			subset[key] = valstr
		}
	}
	return subset
}

// returns a subset of `state.KeyVals` for all keys starting with given `prefix`.
// `state.KeyVals` contains mixed typed values so use with caution!
func (state State) SomeKeyAnyVals(prefix string) map[string]any {
	subset := make(map[string]any)
	for key, val := range state.KeyVals {
		if strings.HasPrefix(key, prefix) {
			subset[key] = val
		}
	}
	return subset
}

// convenience. see `state.SomeKeyAnyVals`.
func (app *App) SomeKeyAnyVals(prefix string) map[string]any {
	return app.state.SomeKeyAnyVals(prefix)
}

// convenience. given a 'foo.bar' `root`, set each val in `keyvals` to `$root.$key=$val`.
// no guarantee of consistent state when using KeyVals in KV store in parallel.
// todo: investigate sync.Map
func (app *App) SetKeyAnyVals(root string, keyvals map[string]any) {
	if root != "" {
		root += "."
	}
	for key, val := range keyvals {
		app.state.KeyVals[root+key] = val
	}
}

// convenience. just like `SetKeyAnyVals` but only string vals.
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

func add_replace_result(state State, new_result_list ...Result) State {
	if len(new_result_list) == 0 {
		return state
	}

	// excludes any results that are being replaced,
	// then concats the remaining keepers with the new result list.

	keepers := []Result{}
	tmp_idx := map[string]Result{}

	for _, r := range new_result_list {
		tmp_idx[r.ID] = r
	}
	for _, old_result := range state.Root.Item.([]Result) {
		_, present := tmp_idx[old_result.ID]
		if !present {
			keepers = append(keepers, old_result)
		}
	}

	keepers = append(keepers, new_result_list...)

	state.Root.Item = keepers

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
	// deadlock. replaced with an id check below.
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
		slog.Debug("result not found in index", "id", id)
		return nil
	}
	result := &app.state.Root.Item.([]Result)[idx]
	if result.ID != id {
		// did the index or result list change between fetching the index and retrieving the result?
		slog.Error("id in index does not match id of result from result list", "given", id, "actual", result.ID)
		panic("programming error")
	}
	return result
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
	if result.IsEmpty() {
		return result
	}

	if result.ID == id {
		return result
	}

	switch t := result.Item.(type) {
	case Result:
		// we have a Result.Result, recurse
		return find_result_by_id1(t, id)

	case []Result:
		// we have a Result.[]Result, recurse on each
		for _, r := range t {
			rr := find_result_by_id1(r, id)
			if rr.IsEmpty() {
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
		if !r.IsEmpty() {
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
		if res.IsEmpty() {
			slog.Warn("failed to find parent")
			return nil
		}
		if res.ParentID == "" {
			slog.Debug("found top-most parent of id", "id", original_id, "root", res)
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
		if res.IsEmpty() {
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

func (app *App) RegisterService(service ServiceGroup) {
	app.ServiceList = append(app.ServiceList, service)
}

// ---------

// TODO: turn this into a stop + restart thing.
// throw an error, have main.main catch it and call stop() then start()
func (a *App) ResetState() {
	a.state = NewState()
}

func (a *App) FunctionList() []Service {
	var fn_list []Service
	for _, service := range a.ServiceList {
		service := service
		for _, fn := range service.ServiceList {
			fn.ServiceGroup = &service
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

func StartProviderService(thefn func(*App, ServiceArgs) ServiceResult) Service {
	return Service{
		Label:       START_PROVIDER_SERVICE,
		Description: "Initialises the provider, called during provider registration, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

func StopProviderService(thefn func(*App, ServiceArgs) ServiceResult) Service {
	return Service{
		Label:       STOP_PROVIDER_SERVICE,
		Description: "Stops the provider, called during provider cleanup, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

type Provider interface {
	// a list of services that this Provider provides.
	ServiceList() []ServiceGroup
	// a list of services keyed by item type
	ItemHandlerMap() map[reflect.Type][]Service
}

func (a *App) RegisterProvider(p Provider) {
	for _, service := range p.ServiceList() {
		a.RegisterService(service)
	}

	for itemtype, service_list := range p.ItemHandlerMap() {
		sl, present := a.TypeMap[itemtype]
		if !present {
			sl = []Service{}
		}
		sl = append(sl, service_list...)
		a.TypeMap[itemtype] = sl
	}
}

// initialisation hook for providers.
// if a provider has a registered service with the name `core.START_PROVIDER_SERVICE`
// it will be called here.
func (a *App) StartProviders() {
	slog.Debug("starting providers", "num-providers", len(a.ServiceList)) // bug: mismatch between len and num started
	for idx, service := range a.ServiceList {
		slog.Debug("starting provider", "num", idx, "provider", service)
		for _, service_fn := range service.ServiceList {
			if service_fn.Label == START_PROVIDER_SERVICE {
				service_fn.Fn(a, ServiceArgs{})
			}
		}
	}
}

// a shutdown hook for providers
func (a *App) StopProviders() {
	slog.Debug("cleaning up providers")

	// todo: reverse order

	for _, service := range a.ServiceList {
		for _, service_fn := range service.ServiceList {
			if service_fn.Label == STOP_PROVIDER_SERVICE {
				service_fn.Fn(a, ServiceArgs{})
			}
		}
	}

}

func Start() *App {
	app := NewApp()
	app.SetKeyVals("bw.app", map[string]string{
		"name":       "bw",
		"version":    "0.1.0",
		"data-dir":   "~/.local/share/bw/",
		"config-dir": "~/.config/bw/",
	})

	// todo: needs a ~/.local/share/bw/cache
	err := os.Mkdir("/tmp/http-cache", 0740)
	if err != nil {
		slog.Error("failed to create /tmp/http-cache", "error", err)
	}

	// ---

	slog.Info("app started", "app", app)
	go app.ProcessUpdateLoop()

	return app
}
