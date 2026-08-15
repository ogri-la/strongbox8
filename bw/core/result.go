// logic around the `core.Result` type

package core

import "log/slog"

// returns the descendents of the given `result` as a flat list, each with its `ParentID` set.
// recursive.
// returns an empty list when the result has no item, has already been realised, does not
// implement `ItemInfo`, or its policy says not to load children.
// `load_child_policy` is the policy inherited from the caller; pass "" at the top level to
// take the policy from the result's own item.
func _realise_children(app *App, result Result, load_child_policy ITEM_CHILDREN_LOAD) []Result {
	empty := []Result{}

	// item is missing! could be a dummy row or bad programming
	if result.Item == nil {
		return empty
	}

	// the caller has told us to stop descending.
	if load_child_policy == ITEM_CHILDREN_LOAD_FALSE {
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

	// lazy children are not loaded here.
	// todo: 2024-07-21 - something amiss here
	if load_child_policy == ITEM_CHILDREN_LOAD_LAZY {
		return empty
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

	return children
}

// returns each result in `result` followed by its descendents, as one flat list.
// each returned parent is marked as realised.
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

// returns the child results of the given `result`, realising them first if necessary.
// the error is always nil.
func Children(app *App, result Result) ([]Result, error) {
	if !result.ChildrenRealised {
		slog.Debug("children not realised")
		children := realise_children(app, result)
		app.AddReplaceResults(children...)
	}

	foo := app.FilterResultList(func(r Result) bool {
		return r.ParentID == result.ID
	})

	return foo, nil
}
