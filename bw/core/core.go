package core

import (
	"bw/http_utils"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
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

// ---

type Tag = string

var (
	// I fancy I'd like to tag results with the 1,2,3 keys like I do with email sometimes
	TAG_1 Tag = "one"
	TAG_2 Tag = "two"
	TAG_3 Tag = "three"

	// provider is hinting to app that the result can be updated (somehow)
	TAG_HAS_UPDATE = "has-update"

	// provider is hinting to app that the children of this result should be shown/expanded
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

func NewResult() Result {
	return Result{
		Tags: mapset.NewSet[Tag](),
	}
}

func MakeResult(ns NS, item any, id string) Result {
	r := NewResult()
	r.NS = ns
	r.Item = item
	r.ID = id
	return r
}

// ---

type IApp interface {
	/*
		RegisterService(service Service)
		AddResults(result ...Result)
		GetResult(name string) *Result
		FunctionList() ([]Fn, error)
		ResetState()
	*/
}

type StateUpdate struct {
	Fn func(State) State
	Wg *sync.WaitGroup
}

type StateUpdateChan chan StateUpdate

// ---

type App struct {
	IApp
	State            *State // state not exported. access state with GetState, update with UpdateState
	ProviderList     []Provider
	ServiceGroupList []ServiceGroup // superset of each provider's ServiceGroupList
	FailedProviders  mapset.Set[Provider]

	// a mapping of types to provider services that accept them.
	// used when right-clicking an item (context menu) to find available services
	TypeMap map[reflect.Type][]Service // rename ServiceTypeMap or something

	Menu []Menu

	update_chan StateUpdateChan

	atomic *sync.Mutex

	Downloader IDownloader // "Downloader.DownloadFile(...)". simple/simplistic interface for downloading files. Can be swapped out with something that yields dummy files during testing

	// shared HTTP client for persistent connections.
	// see `bw.http_utils.Request`
	HTTPClient *http.Client
}

// todo: NewApp => MakeApp
func NewApp() *App {
	state := NewState()
	state.KeyVals = map[string]any{
		"bw.app.name":    "bw",
		"bw.app.version": "0.1.0",
	}
	app := App{
		State:            &state,
		ServiceGroupList: []ServiceGroup{},
		FailedProviders:  mapset.NewSet[Provider](),
		TypeMap:          make(map[reflect.Type][]Service),
		Menu:             []Menu{},
		Downloader:       &HTTPDownloader{},
		HTTPClient:       &http.Client{},
		update_chan:      make(chan StateUpdate, 100),
		atomic:           &sync.Mutex{},
	}
	app.HTTPClient.Transport = &http_utils.FileCachingRequest{
		CWD:             "/tmp",
		UseExpiredCache: true,
	}
	return &app
}

// returns a copy of the app state // TODO: does it though? TODO: rename app.ResultList()
func (app *App) StateRoot() []Result {
	return app.State.Root.Item.([]Result)
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

func (app *App) process_update(update StateUpdate) {
	app.atomic.Lock()
	defer app.atomic.Unlock()

	update.Wg.Add(1)
	defer update.Wg.Done()

	//old_state := state      //*state.state
	new_state := update.Fn(*app.State) // fn's waitgroup is unlocked here
	app.State = &new_state             // replace the state we're acting upon with the new state

	app.State.index = results_list_index(app.State.Root.Item.([]Result))
	app.State.ListenerList = process_listeners(*app.State, app.State.ListenerList)

	// update's waitgroup unlocked here
}

// processes a single pending state update,
// calling `fn`, modifying `app`, and executing it's list of listeners
func (app *App) ProcessUpdate() {
	app.process_update(<-app.update_chan)
}

// pulls state updates off of app's internal update channel,
// processes it and then repeats, forever.
func (app *App) ProcessUpdateLoop() {
	for update := range app.update_chan {
		app.process_update(update)
	}
	slog.Debug("app update chan closed")
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

		original := state.Root.Item.([]Result)[result_idx]
		clone := clone.Clone(original)
		someval := xform(clone)

		slog.Debug("updating result with new values", "id", someid, "oldval", original, "newval", someval)
		state.Root.Item.([]Result)[result_idx] = someval

		return state
	}

	update := StateUpdate{
		Fn: fn,
		Wg: &wg,
	}

	app.update_chan <- update

	return &wg
}

// update the app state by applying a function to a copy of the current state,
// returning the new state to be set.
func (app *App) UpdateState(fn func(old_state State) State) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	update_fn := func(state State) State {
		defer wg.Done()
		// I imagine this is _super_ expensive :(
		// - don't use UpdateState unless you can avoid it.
		// - target the results you want to update with UpdateResult
		c := clone.Clone(state)
		new_state := fn(c)
		new_state.SetRoot(realise_children(app, new_state.GetResults()...))
		return new_state
	}

	app.update_chan <- StateUpdate{
		Fn: update_fn,
		Wg: &wg,
	}
	return &wg
}

// ---

func (app *App) RealiseChildren(parent Result) []Result {
	return realise_children(app, parent)
}

// ---

// adds new results, replaces existing results.
func add_replace_result(old_state State, new_result_list ...Result) State {
	if len(new_result_list) == 0 {
		return old_state
	}

	// excludes any results that are being replaced,
	// then concats the remaining keepers with the new result list.

	keepers := []Result{}
	tmp_idx := map[string]Result{}

	for _, r := range new_result_list {
		tmp_idx[r.ID] = r
	}
	for _, old_result := range old_state.Root.Item.([]Result) {
		_, present := tmp_idx[old_result.ID]
		if !present {
			// old result not present in list of new results, preserve it
			keepers = append(keepers, old_result)
		} else {
			// old result *is* present in list of new results, skip it - it will be replaced
		}
	}

	keepers = append(keepers, new_result_list...)

	old_state.Root.Item = keepers

	return old_state
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
		return add_result(old_state, result_list...)
	})
}

// adds all items in `result_list` to app state and updates the index.
// if the same item already exists in app state, it will be replaced by the new item.
func (app *App) SetResults(result_list ...Result) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		return add_replace_result(old_state, result_list...)
	})
}

// returns a map of {parent-id: [child-id, ...], ...}
func _build_tree_map(nodes []Result) map[string][]string {
	idx := make(map[string][]string)
	for _, r := range nodes {
		idx[r.ParentID] = append(idx[r.ParentID], r.ID)
	}
	return idx
}

// find all descendants of `id`.
// recursive.
func _find_descendents(idx map[string][]string, id string) mapset.Set[string] {
	//to_be_removed := make(map[string]bool)
	to_be_removed := mapset.NewSet[string]()

	var recurse func(string)
	recurse = func(id string) {
		to_be_removed.Add(id)
		for _, child := range idx[id] {
			recurse(child)
		}
	}

	recurse(id)
	return to_be_removed
}

// removes all results where `filter_fn(result)` is true,
// including the descendents of those results.
func (app *App) RemoveResults(filter_fn func(Result) bool) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		target_list := []Result{}
		for _, result := range old_state.Root.Item.([]Result) {
			if filter_fn(result) {
				target_list = append(target_list, result)
			}
		}

		if len(target_list) == 0 {
			// nothing found, nothing to do.
			// todo: indicate a noop somehow?
			return old_state
		}

		idx := _build_tree_map(old_state.Root.Item.([]Result))
		to_be_removed := mapset.NewSet[string]()
		for _, r := range target_list {
			to_be_removed = to_be_removed.Union(_find_descendents(idx, r.ID))
		}

		result_list := []Result{}
		for _, r := range old_state.Root.Item.([]Result) {
			if !to_be_removed.Contains(r.ID) {
				result_list = append(result_list, r)
			}
		}

		old_state.Root.Item = result_list
		return old_state
	})
}

// removes a single result by ID
func (app *App) RemoveResult(id string) *sync.WaitGroup {
	return app.RemoveResults(func(r Result) bool {
		return r.ID == id
	})
}

func (app *App) GetResultList() []Result {
	return app.State.Root.Item.([]Result)
}

func filter_result_list(result_list []Result, filter_fn func(Result) bool) []Result {
	new_result_list := []Result{}
	for _, result := range result_list {
		if filter_fn(result) {
			new_result_list = append(new_result_list, result)
		}
	}

	// TODO: why ...?
	sort.Slice(new_result_list, func(i, j int) bool {
		return new_result_list[i].ID < new_result_list[j].ID
	})

	return new_result_list
}

// returns a list of results where `filter_fn(result)` is true
func (app *App) FilterResultList(filter_fn func(Result) bool) []Result {
	return filter_result_list(app.State.Root.Item.([]Result), filter_fn)
}

// returns the first result where `filter_fn(result)` is true
func (app *App) FindResult(filter_fn func(Result) bool) *Result {
	for _, result := range app.State.Root.Item.([]Result) {
		if filter_fn(result) {
			return &result
		}
	}
	return nil
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
	for _, result := range app.State.Root.Item.([]Result) {
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
	for _, result := range app.State.Root.Item.([]Result) {
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

	idx, present := app.State.index[id]
	if !present {
		slog.Debug("result not found in index", "id", id)
		return nil
	}
	result := &app.State.Root.Item.([]Result)[idx]
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
	_, present := app.State.index[id]
	return present
}

// find first result rooted in `result` (including `result`) whose ID matches `id`.
// recursive, naive and expensive.
/*
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
*/

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
	return find_result_by_id(app.State.Root, id)
}

// find all results whose ID is in `id_list`
func (app *App) FindResultByIDList(id_list []string) []Result {
	result_list := []Result{}
	for _, id := range id_list {
		r := find_result_by_id(app.State.Root, id)
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

// ---

func (app *App) DataDir() string {
	return app.State.KeyVal("app.data-dir")
}

func (app *App) ConfigDir() string {
	return app.State.KeyVal("app.config-dir")
}

// ---

func (app *App) RegisterService(service ServiceGroup) {
	app.ServiceGroupList = append(app.ServiceGroupList, service)
}

// urgh. this sucks. nested loops suck. get rid of ServiceGroup? add an index? is the uniqueness of IDs enforced?
func (app *App) FindService(service_id string) (Service, error) {
	for _, service_group := range app.ServiceGroupList {
		for _, service := range service_group.ServiceList {
			if service.ID == service_id {
				return service, nil
			}
		}
	}
	return Service{}, fmt.Errorf("service not found: %s", service_id)

}

// ---------

// TODO: turn this into a stop + restart thing.
// throw an error, have main.main catch it and call stop() then start()
func (a *App) ResetState() {
	s := NewState()
	a.State = &s
}

func (a *App) FunctionList() []Service {
	var fn_list []Service
	for _, service := range a.ServiceGroupList {
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

func StartProviderService(thefn func(*App, ServiceFnArgs) ServiceResult) Service {
	return Service{
		Label:       START_PROVIDER_SERVICE,
		Description: "Initialises the provider, called during provider registration, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

func StopProviderService(thefn func(*App, ServiceFnArgs) ServiceResult) Service {
	return Service{
		Label:       STOP_PROVIDER_SERVICE,
		Description: "Stops the provider, called during provider cleanup, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

// ---

// note: can't live in ./ui
// that would introduce a circular dependency between provider interface in core depending on ui and ui depending on core
// todo: can we squash all of boardwalk into a single namespace?

// a clickable menu entry of a `Menu`
type MenuItem struct {
	Name string
	//Accelerator ...
	Fn func(*App)
	//Parent MenuItem
	ServiceID string // id of the service to call
}

var MENU_SEP = MenuItem{Name: "sep"}

// a top-level menu item, like 'File' or 'View'.
type Menu struct {
	Name string
	//Accelerator ...
	MenuItemList []MenuItem
}

// append-merges the contents of `b` into `a`
func MergeMenus(a []Menu, b []Menu) []Menu {
	a_idx := map[string]*Menu{}
	for i := range a {
		a_idx[a[i].Name] = &a[i]
	}

	for _, mb := range b {
		ma, present := a_idx[mb.Name]
		if present {
			// menu b exists in menu a,
			// append the items from menu b to the end of the items in menu a
			ma.MenuItemList = append(ma.MenuItemList, mb.MenuItemList...)
			//a = append(a, ma)
		} else {
			// menu b does not exist in menu a
			// append the menu as-is and update the index
			a = append(a, mb)
			a_idx[mb.Name] = &mb
		}
	}
	return a
}

// ---

type Provider interface {
	ID() string
	// a list of services that this Provider provides.
	ServiceList() []ServiceGroup
	// a list of services keyed by item type
	ItemHandlerMap() map[reflect.Type][]Service
	Menu() []Menu
}

func (app *App) RegisterProvider(p Provider) {
	app.ProviderList = append(app.ProviderList, p) // TODO: uniqueness
}

func (app *App) ProviderStarted(p Provider) bool {
	return len(app.ProviderList) > 0 && !app.FailedProviders.Contains(p)
}

// initialisation hook for providers.
// if a provider has a registered service with the name `core.START_PROVIDER_SERVICE`
// it will be called here.
func (a *App) StartProviders() {
	slog.Debug("starting providers", "num-providers", len(a.ServiceGroupList)) // bug: mismatch between len and num started
	for i, provider := range a.ProviderList {
		slog.Debug("starting provider", "i", i, "provider", provider.ID)
		// TODO: can we remove this nesting of service function groups?
		for _, service := range provider.ServiceList() {
			for _, service_fn := range service.ServiceList {
				if service_fn.Label == START_PROVIDER_SERVICE {
					result := service_fn.Fn(a, ServiceFnArgs{})
					if result.Err != nil {
						slog.Error("failed to start provider", "error", result.Err)
						a.FailedProviders.Add(provider)
					}
				}
			}
		}
	}

	// associate native types with provider services
	for _, p := range a.ProviderList {
		if a.FailedProviders.Contains(p) {
			slog.Debug("provider failed to start, not registering services", "provider", p.ID())
			continue
		}

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

	// hook providers into the menu
	for _, p := range a.ProviderList {
		if a.FailedProviders.Contains(p) {
			slog.Debug("provider failed to start, not building menu", "provider", p.ID())
			continue
		}
		a.Menu = MergeMenus(a.Menu, p.Menu())
	}
}

// a shutdown hook for providers
func (a *App) StopProviders() {
	slog.Debug("cleaning up providers")

	// stop providers in reverse order.
	// providers shouldn't have dependencies on other providers but who knows
	for i := len(a.ServiceGroupList) - 1; i >= 0; i-- {
		service := a.ServiceGroupList[i]
		for _, service_fn := range service.ServiceList {
			if service_fn.Label == STOP_PROVIDER_SERVICE {
				service_fn.Fn(a, ServiceFnArgs{})
			}
		}
	}
}

func (app *App) Stop() {
	app.StopProviders()
	close(app.update_chan)
}

// ---

// TODO: this might be better off in some sort of bw.main module
func Start() *App {
	app := NewApp()
	keyvals := map[string]string{
		"app.name":       "bw",
		"app.version":    "0.1.0",
		"app.data-dir":   HomePath("/.local/share/bw/"),
		"app.config-dir": HomePath("/.config/bw/"),
	}
	for key, val := range keyvals {
		app.State.SetKeyVal(key, val)
	}

	// note: it's up to the app to ensure any dirs are created!

	// todo: needs a ~/.local/share/bw/cache
	/*
		err := os.Mkdir("/tmp/http-cache", 0740)
		if err != nil {
			slog.Error("failed to create /tmp/http-cache", "error", err)
		}
	*/
	// ---

	slog.Info("app started", "app", app)
	go app.ProcessUpdateLoop()

	// why not StartProviders()?
	// providers need to be registered with an app first, and this func provides that.

	return app
}
