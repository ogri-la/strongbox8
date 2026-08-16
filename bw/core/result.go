// logic around the `core.Result` type

package core

import (
	"log/slog"
	"time"
)

// how long an on-demand load of an item's children may take before it is abandoned.
// a variable rather than a constant so tests can shorten it.
var LazyRealiseTimeout = 5 * time.Second

// namespace for results standing in for children that failed to load.
var NS_LOAD_FAILURE = MakeNS("bw", "error", "timeout")

// terminal item shown in place of an item's children when loading them was abandoned.
// it has no children of its own.
type LoadFailure struct {
	Reason string
}

func (lf LoadFailure) ItemKeys() []string {
	return []string{ITEM_FIELD_NAME}
}

func (lf LoadFailure) ItemMap() map[string]string {
	return map[string]string{ITEM_FIELD_NAME: lf.Reason}
}

func (lf LoadFailure) ItemHasChildren() ITEM_CHILDREN_LOAD {
	return ITEM_CHILDREN_LOAD_FALSE
}

func (lf LoadFailure) ItemChildren(*App) []Result {
	return []Result{}
}

var _ ItemInfo = (*LoadFailure)(nil)

// returns a `LoadFailure` result for the given `parent`.
// the ID derives from the parent's ID so repeated failures replace one another.
func MakeLoadFailureResult(parent Result, reason string) Result {
	return MakeResult(NS_LOAD_FAILURE, LoadFailure{Reason: reason}, parent.ID+"/load-failure")
}

// returns true when `result` holds an item whose children load on demand.
func item_is_lazy(result Result) bool {
	if result.Item == nil || !HasItemInfo(result.Item) {
		return false
	}
	return result.Item.(ItemInfo).ItemHasChildren() == ITEM_CHILDREN_LOAD_LAZY
}

// returns the descendents of the given `result` as a flat list, each with its `ParentID` set.
// recursive.
// the item's own policy decides whether its children are loaded: an empty list is returned
// when the result has no item, has already been realised, does not implement `ItemInfo`,
// or its policy is do-not-load or lazy. lazy children are only loaded on demand, see `Children`.
func _realise_children(app *App, result Result) []Result {
	empty := []Result{}

	// item is missing! could be a dummy row or bad programming
	if result.Item == nil {
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

	var children []Result
	item_as_row := result.Item.(ItemInfo)
	load_child_policy := item_as_row.ItemHasChildren()

	if load_child_policy != ITEM_CHILDREN_LOAD_TRUE {
		return empty
	}

	for _, child := range item_as_row.ItemChildren(app) {
		child.ParentID = result.ID

		grandchildren := _realise_children(app, child)
		children = append(children, grandchildren...)
		// a result cannot be said to be realised until all of its descendants are realised.
		// if we try to realise a result's children, and it returns grandchildren, then we know
		// they have been realised.
		if len(grandchildren) != 0 {
			child.ChildrenRealised = true
		}
		children = append(children, child)
	}

	return children
}

// returns each result in `result` followed by its descendents, as one flat list.
// each returned parent is marked as realised, except lazy parents that have not
// been realised on demand yet.
func realise_children(app *App, result ...Result) []Result {
	slog.Debug("realising children", "num-results", len(result))

	child_list := []Result{}
	for _, r := range result {
		children := _realise_children(app, r)
		if !item_is_lazy(r) {
			r.ChildrenRealised = true
		}
		child_list = append(child_list, r)
		child_list = append(child_list, children...)
	}

	slog.Debug("done realising children", "num-results", len(child_list))

	return child_list
}

// loads the children of `result`'s `item`, abandoning the attempt after `LazyRealiseTimeout`.
// an abandoned load yields a single terminal `LoadFailure` result; whatever the load
// eventually returns is discarded and never enters state.
func item_children_with_timeout(app *App, result Result, item ItemInfo) []Result {
	ch := make(chan []Result, 1)
	go func() {
		ch <- item.ItemChildren(app)
	}()

	select {
	case children := <-ch:
		return children
	case <-time.After(LazyRealiseTimeout):
		slog.Warn("loading children took too long, load abandoned", "id", result.ID, "ns", result.NS, "timeout", LazyRealiseTimeout)
		return []Result{MakeLoadFailureResult(result, "failed to load: took longer than "+LazyRealiseTimeout.String())}
	}
}

// returns the child results of the given `result`, realising them first if necessary.
// realisation loads one level: the result's immediate children are added to state and
// the result is marked realised; lazy grandchildren stay unrealised. a result that is
// already realised has its children read from state, not loaded again.
// realisation is bounded by `LazyRealiseTimeout`: when exceeded the sole child is a
// terminal `LoadFailure` result.
// the error is always nil.
func Children(app *App, result Result) ([]Result, error) {
	if result.ChildrenRealised {
		return app.FilterResultList(func(r Result) bool {
			return r.ParentID == result.ID
		}), nil
	}

	empty := []Result{}

	if result.Item == nil || !HasItemInfo(result.Item) {
		return empty, nil
	}

	item := result.Item.(ItemInfo)
	if item.ItemHasChildren() == ITEM_CHILDREN_LOAD_FALSE {
		return empty, nil
	}

	children := item_children_with_timeout(app, result, item)
	for i := range children {
		children[i].ParentID = result.ID
	}

	// realise any eager descendants of the loaded children.
	flat := realise_children(app, children...)
	result.ChildrenRealised = true
	app.AddReplaceResults(append([]Result{result}, flat...)...)

	direct := []Result{}
	for _, child := range flat {
		if child.ParentID == result.ID {
			direct = append(direct, child)
		}
	}

	return direct, nil
}
