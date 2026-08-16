package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// test fixture: an item whose child-loading policy, children and load duration are configurable.
type TreeItem struct {
	Name     string
	Policy   ITEM_CHILDREN_LOAD
	Children []TreeItem
	Delay    time.Duration
}

// counts calls to `TreeItem.ItemChildren` across a single test. reset it at test start.
var tree_item_children_calls int

func (ti TreeItem) ItemKeys() []string {
	return []string{ITEM_FIELD_NAME}
}

func (ti TreeItem) ItemMap() map[string]string {
	return map[string]string{ITEM_FIELD_NAME: ti.Name}
}

func (ti TreeItem) ItemHasChildren() ITEM_CHILDREN_LOAD {
	return ti.Policy
}

func (ti TreeItem) ItemChildren(*App) []Result {
	tree_item_children_calls++
	if ti.Delay > 0 {
		time.Sleep(ti.Delay)
	}
	result_list := []Result{}
	for _, child := range ti.Children {
		result_list = append(result_list, MakeResult(test_ns, child, "tree-item-"+child.Name))
	}
	return result_list
}

// an eager item added to state has its children loaded immediately
func TestRealiseChildrenOnInsert__eager(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_TRUE,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual := a.GetResultList()
	assert.Equal(t, 2, len(actual))
	assert.True(t, actual[0].ChildrenRealised)
	assert.Equal(t, "parent", actual[1].ParentID)
	assert.Equal(t, 1, tree_item_children_calls)
}

// a do-not-load item added to state never has its children loaded
func TestRealiseChildrenOnInsert__do_not_load(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_FALSE,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual := a.GetResultList()
	assert.Equal(t, 1, len(actual))
	assert.True(t, actual[0].ChildrenRealised)
	assert.Equal(t, 0, tree_item_children_calls)
}

// a lazy item added to state at the top level defers loading its children
func TestRealiseChildrenOnInsert__lazy_defers(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_LAZY,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual := a.GetResultList()
	assert.Equal(t, 1, len(actual))
	assert.False(t, actual[0].ChildrenRealised)
	assert.Equal(t, 0, tree_item_children_calls)
}

// a lazy item stays unrealised across unrelated state updates
func TestRealiseChildrenOnInsert__lazy_survives_updates(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_LAZY,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	a.AppendResults(MakeResult(test_ns, "unrelated", "unrelated-id"))
	a.ProcessUpdate()

	actual := a.FindResultByID("parent")
	assert.False(t, actual.ChildrenRealised)
	assert.Equal(t, 0, tree_item_children_calls)
}

// realising a lazy item on demand loads one level and marks it realised
func TestChildren__realises_one_level(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:   "parent",
		Policy: ITEM_CHILDREN_LOAD_LAZY,
		Children: []TreeItem{{
			Name:     "child",
			Policy:   ITEM_CHILDREN_LOAD_LAZY,
			Children: []TreeItem{{Name: "grandchild", Policy: ITEM_CHILDREN_LOAD_FALSE}},
		}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual, err := Children(a, a.FindResultByID("parent"))
	a.ProcessUpdate()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(actual))
	assert.Equal(t, "tree-item-child", actual[0].ID)
	assert.Equal(t, "parent", actual[0].ParentID)
	assert.Equal(t, 1, tree_item_children_calls) // the parent's call only, the lazy child is untouched

	assert.Equal(t, 2, len(a.GetResultList()))
	assert.True(t, a.FindResultByID("parent").ChildrenRealised)
	assert.False(t, a.FindResultByID("tree-item-child").ChildrenRealised)
}

// realising a lazy item a second time reads from state instead of loading again
func TestChildren__realises_once(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_LAZY,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	expected, _ := Children(a, a.FindResultByID("parent"))
	a.ProcessUpdate()

	actual, err := Children(a, a.FindResultByID("parent"))

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
	assert.Equal(t, 1, tree_item_children_calls)
}

// realising a do-not-load item on demand loads nothing
func TestChildren__do_not_load(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_FALSE,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")
	given.ChildrenRealised = false

	a := NewApp()

	actual, err := Children(a, given)

	assert.Nil(t, err)
	assert.Equal(t, []Result{}, actual)
	assert.Equal(t, 0, tree_item_children_calls)
}

// a load exceeding the timeout is abandoned: the sole child is a terminal failure
// and the late results never enter state
func TestChildren__timeout(t *testing.T) {
	tree_item_children_calls = 0
	original_timeout := LazyRealiseTimeout
	LazyRealiseTimeout = 20 * time.Millisecond
	defer func() { LazyRealiseTimeout = original_timeout }()

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_LAZY,
		Delay:    100 * time.Millisecond,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual, err := Children(a, a.FindResultByID("parent"))
	a.ProcessUpdate()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(actual))
	assert.Equal(t, "parent/load-failure", actual[0].ID)
	failure, is_failure := actual[0].Item.(LoadFailure)
	assert.True(t, is_failure)
	assert.Contains(t, failure.Reason, "failed to load")
	assert.Equal(t, ITEM_CHILDREN_LOAD_FALSE, failure.ItemHasChildren())
	assert.True(t, a.FindResultByID("parent").ChildrenRealised)

	// the abandoned load completes late, queues no state update and its results are discarded
	time.Sleep(150 * time.Millisecond)
	assert.False(t, a.HasResult("tree-item-child"))
	assert.Equal(t, 2, len(a.GetResultList()))
}

// a load finishing within the timeout is unaffected by it
func TestChildren__timeout_fast_load_unaffected(t *testing.T) {
	tree_item_children_calls = 0
	original_timeout := LazyRealiseTimeout
	LazyRealiseTimeout = 500 * time.Millisecond
	defer func() { LazyRealiseTimeout = original_timeout }()

	given := MakeResult(test_ns, TreeItem{
		Name:     "parent",
		Policy:   ITEM_CHILDREN_LOAD_LAZY,
		Children: []TreeItem{{Name: "child", Policy: ITEM_CHILDREN_LOAD_FALSE}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual, err := Children(a, a.FindResultByID("parent"))
	a.ProcessUpdate()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(actual))
	assert.Equal(t, "tree-item-child", actual[0].ID)
}

// a lazy item nested under an eager item is added unrealised, its own children unloaded
func TestRealiseChildrenOnInsert__nested_lazy_defers(t *testing.T) {
	tree_item_children_calls = 0

	given := MakeResult(test_ns, TreeItem{
		Name:   "parent",
		Policy: ITEM_CHILDREN_LOAD_TRUE,
		Children: []TreeItem{{
			Name:     "child",
			Policy:   ITEM_CHILDREN_LOAD_LAZY,
			Children: []TreeItem{{Name: "grandchild", Policy: ITEM_CHILDREN_LOAD_FALSE}},
		}},
	}, "parent")

	a := NewApp()
	a.AppendResults(given)
	a.ProcessUpdate()

	actual := a.GetResultList()
	assert.Equal(t, 2, len(actual))
	assert.Equal(t, "parent", actual[1].ParentID)
	assert.False(t, actual[1].ChildrenRealised)
	assert.Equal(t, 1, tree_item_children_calls) // the parent's call only
}
