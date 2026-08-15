// the Boardwalk framework: an `App` holds a single `State` of `Result` items,
// contributed by registered `Provider` implementations.
// all state changes pass through the app's update channel and are applied one at a
// time by `App.ProcessUpdateLoop`, so behaviour stays deterministic and testable.
// read state with the `App.Get*`/`App.Find*` methods, never by writing to `App.State`.
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

// todo: we're 'in development' when the current version is 'unreleased'.
// it is expected that certain debug features won't work from inside a binary.
var VERSION = "unreleased"

// returns `true` when the app is a development build.
// debug-only features may not work from inside a released binary.
func Debug() bool {
	return VERSION == "unreleased"
}

func DebugRes(prefix string, idx int, result Result) {
	if result.ParentID == "" {
		fmt.Printf("%s[%v] id:%v parent:nil\n", prefix, idx, result.ID)
	} else {
		fmt.Printf("%s[%v] id:%v parent:%s\n", prefix, idx, result.ID, result.ParentID)
	}
}

// prints each result in `result_list` to stdout.
// stops after 300 results.
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

func MakeNS(major string, minor string, ttype string) NS {
	return NS{Major: major, Minor: minor, Type: ttype}
}

// ---

// simple key+val. val can be anything.
type KeyVal struct {
	//Help string // useful?
	Key string
	Val any
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

	// provider is hinting to app that the children of this result should be shown/expanded
	TAG_SHOW_CHILDREN = "show-children"
)

// ---

type ActionType = string

var (
	ACTION_SWITCH_TAB ActionType = "switch-tab" // payload is `string` (tab title)
)

type Action struct {
	Type    ActionType
	Payload any
}

// ---

// a single item of application state, wrapped with the metadata the app and UI need.
// results are stored in one flat list; a tree is expressed with `ParentID`.
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

// a single atomic change to application state.
// exactly one of `Fn` or `Action` is set: `Fn` transforms the state, `Action`
// is dispatched to observers without touching state.
// `Wg` is signalled once the update has been processed.
type StateUpdate struct {
	Fn     func(State) State // transformation function applied to current state
	Wg     *sync.WaitGroup   // signals when this update has been processed
	Action *Action           // non-nil = action dispatch (no state transform)
}

// the channel all state updates flow through.
// updates are processed one at a time to eliminate race conditions and non-determinism.
type StateUpdateChan chan StateUpdate

// ---

// the application itself: state, the providers contributing to it and the
// services they offer.
// build one with `NewApp` or `Start`.
type App struct {
	State            *State // read with the Get*/Find* methods, update with UpdateState or UpdateResult
	ProviderList     []Provider
	ServiceGroupList []ServiceGroup // superset of each provider's ServiceGroupList
	FailedProviders  mapset.Set[Provider]

	// a mapping of types to provider services that accept them.
	// used when right-clicking an item (context menu) to find available services
	TypeMap map[reflect.Type][]Service // rename ServiceTypeMap or something

	Menu []Menu

	observers []StateObserver

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
		TypeMap:          map[reflect.Type][]Service{},
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

func (app *App) StateRoot() []Result {
	return app.State.Root.Item.([]Result)
}

// --- state management

type StateObserver interface {
	OnResultsChanged(old_snapshot, new_snapshot *Snapshot)
	OnAction(action Action)
}

func (app *App) AddObserver(obs StateObserver) {
	app.observers = append(app.observers, obs)
}

func clone_results(results []Result) []Result {
	out := make([]Result, len(results))
	copy(out, results)
	return out
}

// ---

// returns a simple map of Result.ID => pos for all 'top-level' results.
func results_list_index(results_list []Result) map[string]int {
	slog.Debug("rebuilding index")

	idx := map[string]int{}
	for i, res := range results_list {
		idx[res.ID] = i
	}
	return idx
}

func (app *App) process_update(update StateUpdate) {
	if update.Action != nil {
		for _, obs := range app.observers {
			obs.OnAction(*update.Action)
		}
		update.Wg.Done()
		return
	}

	update.Wg.Add(1)

	app.atomic.Lock()
	old_snapshot := MakeSnapshot(clone_results(app.State.GetResults()))

	new_state := update.Fn(*app.State)
	app.State = &new_state
	app.State.index = results_list_index(app.State.Root.Item.([]Result))

	new_snapshot := MakeSnapshot(app.State.GetResults())
	app.atomic.Unlock()

	for _, obs := range app.observers {
		obs.OnResultsChanged(old_snapshot, new_snapshot)
	}

	update.Wg.Done()
}

// takes a single update off the app's update channel and processes it,
// modifying state and notifying observers.
// blocks until an update is available.
// used by tests to step through updates one at a time.
func (app *App) ProcessUpdate() {
	app.process_update(<-app.update_chan)
}

// processes state updates as they arrive, returning when the update channel is closed.
// run this in a goroutine. `App.Stop` closes the channel and ends the loop.
func (app *App) ProcessUpdateLoop() {
	for update := range app.update_chan {
		app.process_update(update)
	}
	slog.Debug("app update chan closed")
}

// queues an update of the single result with the given `someid`, replacing it with
// the return value of `xform`.
// the update is a no-op, logged as an error, when no result has that ID.
// children are not realised, so `xform` must not change the ID or the parent.
// the returned `WaitGroup` is done once the update has been applied.
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

// queues a transformation of the entire application state, realising the children of
// the results `fn` returns.
// `fn` is given a deep copy, so the old state is never modified in place.
// the returned `WaitGroup` is done once the update has been applied.
// copying the whole state is expensive: prefer `UpdateResult` when only some results change.
func (app *App) UpdateState(fn func(old_state State) State) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	update_fn := func(state State) State {
		defer wg.Done()
		c := clone.Clone(state)
		new_state := fn(c)
		new_result_list := new_state.GetResults()
		new_state.SetRoot(realise_children(app, new_result_list...))
		return new_state
	}

	app.update_chan <- StateUpdate{
		Fn: update_fn,
		Wg: &wg,
	}
	return &wg
}

// sends `action` to every observer without changing state.
// blocks until all observers have been notified.
func (app *App) DispatchAction(action Action) {
	var wg sync.WaitGroup
	wg.Add(1)
	app.update_chan <- StateUpdate{Action: &action, Wg: &wg}
	wg.Wait()
}

// ---

func (app *App) RealiseChildren(parent Result) []Result {
	return realise_children(app, parent)
}

// ---

// adds new results, replaces existing results.
// replaced results move to the end of the result list.
func add_replace_result(old_state State, new_result_list ...Result) State {
	if len(new_result_list) == 0 {
		return old_state
	}

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

// queues an addition of every result in `result_list` to app state.
// the whole addition is refused, and logged as an error, if any result has an ID
// that is already in use.
func (app *App) AppendResults(result_list ...Result) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		return add_result(old_state, result_list...)
	})
}

// queues an addition of every result in `result_list` to app state.
// a result whose ID is already in use replaces the existing result.
// replaced results move to the end of the result list.
func (app *App) AddReplaceResults(result_list ...Result) *sync.WaitGroup {
	return app.UpdateState(func(old_state State) State {
		return add_replace_result(old_state, result_list...)
	})
}

// convenience. add a thing to app state, returning a pointer to that thing wrapped in a `Result`
func (app *App) AddItem(ns NS, item any) (*Result, *sync.WaitGroup) {
	iid := UniqueID()
	r := MakeResult(ns, item, iid)
	wg := app.AppendResults(r) //.Wait() // don't do this. when testing we process updates manually
	return &r, wg
}

// returns a map of {parent-id: [child-id, ...], ...}
func _build_tree_map(nodes []Result) map[string][]string {
	idx := map[string][]string{}
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

// queues a removal of every result where `filter_fn(result)` is true,
// along with the descendents of those results.
// a filter that matches nothing is a no-op.
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

	sort.Slice(new_result_list, func(i, j int) bool {
		return new_result_list[i].ID < new_result_list[j].ID
	})

	return new_result_list
}

// returns the results where `filter_fn(result)` is true, sorted by ID.
func (app *App) FilterResultList(filter_fn func(Result) bool) []Result {
	return filter_result_list(app.State.Root.Item.([]Result), filter_fn)
}

// returns the first result where `filter_fn(result)` is true,
// or nil when nothing matches.
func (app *App) FirstResult(filter_fn func(Result) bool) *Result {
	for _, result := range app.State.Root.Item.([]Result) {
		if filter_fn(result) {
			return &result
		}
	}
	return nil
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

// returns the first result whose NS equals the given `ns`,
// or an empty `Result` when nothing matches.
// intended for known singletons.
// todo: candidate for replacement.
func (app *App) FilterResultListByNSToResult(ns NS) Result {
	for _, result := range app.State.Root.Item.([]Result) {
		if result.NS == ns {
			return result
		}
	}
	return Result{}
}

// returns a result by it's ID, returning nil if not found.
// panics if the index and the result list disagree.
func (app *App) GetResult(id string) *Result {
	// capture the pointer once. the index lookup and the list access must read the
	// same state, otherwise a concurrent process_update can swap app.State between them.
	state := app.State
	idx, present := state.index[id]
	if !present {
		slog.Debug("result not found in index", "id", id)
		return nil
	}
	result := &state.Root.Item.([]Result)[idx]
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

// returns `result` itself, or the first of its immediate children, whose ID matches `id`.
// does not recurse.
// returns an empty `Result` on no match, or when `result.Item` is not a `[]Result`.
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

// returns the result with the given `id`,
// or an empty `Result` when not found.
func (app *App) FindResultByID(id string) Result {
	return find_result_by_id(app.State.Root, id)
}

// returns the results whose ID is in `id_list`, in the order the IDs are given.
// IDs that match nothing are skipped, so the result may be shorter than `id_list`.
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

// returns the first result whose `Item` matches `item`, or nil when nothing matches.
func (app *App) FindResultByItem(item any) *Result {
	for _, r := range app.GetResultList() {
		if r.Item == item {
			return &r
		}
	}
	return nil
}

// returns the top-most ancestor of the result with the given `id`,
// or the result itself when it has no parent.
// a missing `id` returns a pointer to an empty `Result`, never nil, because the
// `IsEmpty` guard below never fires. see ISSUES.md.
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
		}

		id = res.ParentID

		slog.Debug("looping")
	}
}

// returns the ancestors of the result with the given `id`, nearest parent first.
// excludes the result itself.
// returns an empty list when `id` is not found or has no parent.
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

// returns the item payload attached to each result in `result_list` as a slice of given `T`.
// panics if any item is not a `T`.
func ItemList[T any](result_list ...Result) []T {
	t_list := []T{}
	for _, res := range result_list {
		t_list = append(t_list, res.Item.(T))
	}
	return t_list
}

// ---

func (app *App) DataDir() string {
	return app.State.GetKeyVal("app.data-dir")
}

func (app *App) ConfigDir() string {
	return app.State.GetKeyVal("app.config-dir")
}

// ---

func (app *App) RegisterService(service ServiceGroup) {
	app.ServiceGroupList = append(app.ServiceGroupList, service)
}

// returns the registered service with the given `service_id`,
// or an error when no service has that ID.
// todo: nested loops. get rid of ServiceGroup? add an index? is the uniqueness of IDs enforced?
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

func (app *App) ResetState() {
	s := NewState()
	app.State = &s
}

func (app *App) FunctionList() []Service {
	var fn_list []Service
	for _, service := range app.ServiceGroupList {
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

// returns a `Service` that `App.StartProviders` recognises as a provider's start hook.
func StartProviderService(thefn func(*App, ServiceFnArgs) ServiceResult) Service {
	return Service{
		Label:       START_PROVIDER_SERVICE,
		Description: "Initialises the provider, called during provider registration, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

// returns a `Service` that `App.StopProviders` recognises as a provider's stop hook.
func StopProviderService(thefn func(*App, ServiceFnArgs) ServiceResult) Service {
	return Service{
		Label:       STOP_PROVIDER_SERVICE,
		Description: "Stops the provider, called during provider cleanup, should be idempotent",
		Interface:   ServiceInterface{}, // accepts no further args
		Fn:          thefn,
	}
}

// a source of services, item handlers and menus.
// register one with `App.RegisterProvider`, then start it with `App.StartProviders`.
type Provider interface {
	ID() string
	// a list of services that this Provider provides.
	ServiceList() []ServiceGroup
	// a list of services keyed by item type
	ItemHandlerMap() map[reflect.Type][]Service
	Menu() []Menu
}

func (app *App) RegisterProvider(p Provider) {
	app.ProviderList = append(app.ProviderList, p)
}

func (app *App) ProviderStarted(p Provider) bool {
	return len(app.ProviderList) > 0 && !app.FailedProviders.Contains(p)
}

// starts every registered provider, then registers its services, item handlers and menus.
// a provider is started by calling its `core.START_PROVIDER_SERVICE` service, if it has one.
// a provider whose start service returns an error is added to `app.FailedProviders` and
// contributes no services, handlers or menus.
func (app *App) StartProviders() {
	slog.Debug("starting providers", "num-providers", len(app.ServiceGroupList)) // bug: mismatch between len and num started
	for i, provider := range app.ProviderList {
		slog.Debug("starting provider", "i", i, "provider", provider.ID)
		for _, service := range provider.ServiceList() {
			for _, service_fn := range service.ServiceList {
				if service_fn.Label == START_PROVIDER_SERVICE {
					result := service_fn.Fn(app, ServiceFnArgs{})
					if result.Err != nil {
						slog.Error("failed to start provider", "error", result.Err)
						app.FailedProviders.Add(provider)
					}
				}
			}
		}
	}

	// associate native types with provider services
	for _, p := range app.ProviderList {
		if app.FailedProviders.Contains(p) {
			slog.Debug("provider failed to start, not registering services", "provider", p.ID())
			continue
		}

		for _, service := range p.ServiceList() {
			app.RegisterService(service)
		}

		for itemtype, service_list := range p.ItemHandlerMap() {
			sl, present := app.TypeMap[itemtype]
			if !present {
				sl = []Service{}
			}
			sl = append(sl, service_list...)
			app.TypeMap[itemtype] = sl
		}
	}

	// hook providers into the menu
	for _, p := range app.ProviderList {
		if app.FailedProviders.Contains(p) {
			slog.Debug("provider failed to start, not building menu", "provider", p.ID())
			continue
		}
		app.Menu = MergeMenus(app.Menu, p.Menu())
	}
}

// stops every registered provider by calling its `core.STOP_PROVIDER_SERVICE` service,
// if it has one.
// providers are stopped in the reverse order they were registered.
func (app *App) StopProviders() {
	slog.Debug("cleaning up providers")

	// providers shouldn't have dependencies on other providers but who knows
	for i := len(app.ServiceGroupList) - 1; i >= 0; i-- {
		service := app.ServiceGroupList[i]
		for _, service_fn := range service.ServiceList {
			if service_fn.Label == STOP_PROVIDER_SERVICE {
				service_fn.Fn(app, ServiceFnArgs{})
			}
		}
	}
}

// stops the providers and closes the update channel, ending `ProcessUpdateLoop`.
// the app cannot be used afterwards.
func (app *App) Stop() {
	app.StopProviders()
	close(app.update_chan)
}

// ---

// returns a new `App` with default key-vals set and its update loop running.
// does not create the config or data directories, and does not start providers:
// providers must be registered against an app first, so the caller registers them
// and then calls `App.StartProviders`.
func Start() *App {
	app := NewApp()
	keyvals := map[string]string{
		"app.name":       "bw",
		"app.version":    "0.1.0",
		"app.data-dir":   HomePath("/.local/share/bw/"),
		"app.config-dir": HomePath("/.config/bw/"),
	}
	for key, val := range keyvals {
		app.State.SetKeyAnyVal(key, val)
	}

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

	return app
}
