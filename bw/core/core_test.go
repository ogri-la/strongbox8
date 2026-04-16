package core

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// Result.Tags are just maps and modifying a pass-by-balue Result is modifying the tags everywhere, so they always pass a deep equality test.
// what else is silently going under the radar?
// if core.Result.Item is a struct with a map that has changes, is it captured?? tests!!!

var test_ns = MakeNS("bw", "test", "ns")

// create app, update state
func TestAppAddResults(t *testing.T) {
	expected := []Result{
		{
			NS:               test_ns,
			ID:               "dummy-id",
			Item:             "",
			ChildrenRealised: true,
			Tags:             mapset.NewSet[Tag](),
		},
	}

	a := NewApp()
	assert.Equal(t, []Result{}, a.StateRoot())

	r2 := Result{
		NS:               test_ns,
		ID:               "dummy-id",
		Item:             "",
		ChildrenRealised: false, // !important
		Tags:             mapset.NewSet[Tag](),
	}

	a.AppendResults(r2)
	a.ProcessUpdate()
	assert.Equal(t, expected, a.StateRoot())
}

// many new items can be added to results
func TestAppAddResults__many(t *testing.T) {
	expected := []Result{
		{
			NS:               test_ns,
			ID:               "dummy-id1",
			Item:             "",
			ChildrenRealised: true,
			Tags:             mapset.NewSet[Tag](),
		},
		{
			NS:               test_ns,
			ID:               "dummy-id2",
			Item:             "",
			ChildrenRealised: true,
			Tags:             mapset.NewSet[Tag](),
		},
		{
			NS:               test_ns,
			ID:               "dummy-id3",
			Item:             "",
			ChildrenRealised: true,
			Tags:             mapset.NewSet[Tag](),
		},
	}

	a := NewApp()

	a.AppendResults([]Result{
		{
			NS:               test_ns,
			ID:               "dummy-id1",
			Item:             "",
			ChildrenRealised: false, // !important
			Tags:             mapset.NewSet[Tag](),
		},
	}...)

	a.AppendResults([]Result{
		{
			NS:               test_ns,
			ID:               "dummy-id2",
			Item:             "",
			ChildrenRealised: false,
			Tags:             mapset.NewSet[Tag](),
		},
		{
			NS:               test_ns,
			ID:               "dummy-id3",
			Item:             "",
			ChildrenRealised: false,
			Tags:             mapset.NewSet[Tag](),
		},
	}...)

	a.ProcessUpdate()
	assert.Equal(t, 1, len(a.StateRoot()))

	a.ProcessUpdate()
	assert.Equal(t, expected, a.StateRoot())
}

// duplicate items (items sharing an ID) are not added to results.
func TestAppAddResults__duplicates(t *testing.T) {
	r := MakeResult(test_ns, "foo", "dummy-id1")
	r.ChildrenRealised = true
	expected := []Result{r}

	a := NewApp()
	a.AppendResults(MakeResult(test_ns, "foo", "dummy-id1"))
	a.ProcessUpdate()

	a.AppendResults(MakeResult(test_ns, "bar", "dummy-id1"))
	a.ProcessUpdate()

	assert.Equal(t, expected, a.GetResultList())
}

// duplicate items replace results
func TestAppSetResults(t *testing.T) {
	r := MakeResult(test_ns, "bar", "dummy-id")
	r.ChildrenRealised = true
	expected := []Result{r}

	a := NewApp()
	a.AddReplaceResults(MakeResult(test_ns, "foo", "dummy-id"))
	a.ProcessUpdate()

	a.AddReplaceResults(MakeResult(test_ns, "bar", "dummy-id"))
	a.ProcessUpdate()

	assert.Equal(t, expected, a.StateRoot())
}

// new items are added to, and duplicate items replace, results
func TestAppSetResults__many(t *testing.T) {
	r1 := MakeResult(test_ns, "bar", "dummy-id1")
	r1.ChildrenRealised = true
	r2 := MakeResult(test_ns, "baz", "dummy-id2")
	r2.ChildrenRealised = true
	expected := []Result{r1, r2}

	a := NewApp()
	a.AddReplaceResults(MakeResult(test_ns, "foo", "dummy-id1"))
	a.ProcessUpdate()

	a.AddReplaceResults(MakeResult(test_ns, "bar", "dummy-id1"), MakeResult(test_ns, "baz", "dummy-id2"))
	a.ProcessUpdate()

	assert.Equal(t, expected, a.StateRoot())
}

// a Result can be removed by ID
func TestAppRemoveResult(t *testing.T) {
	r := MakeResult(test_ns, "bar", "dummy-id1")

	a := NewApp()
	a.AppendResults(r)
	a.ProcessUpdate()

	assert.Equal(t, 1, len(a.GetResultList()))

	a.RemoveResult("dummy-id1")
	a.ProcessUpdate()
	assert.Equal(t, 0, len(a.GetResultList()))
}

// when a Result is removed, so are it's children.
func TestAppRemoveResult__with_children(t *testing.T) {
	gp := MakeResult(test_ns, "bar", "grandparent")

	p := MakeResult(test_ns, "baz", "parent")
	p.ParentID = "grandparent"

	c := MakeResult(test_ns, "bup", "child")
	c.ParentID = "parent"

	// should be preserved
	s := MakeResult(test_ns, "boo", "stranger")

	a := NewApp()
	a.AppendResults(s, gp, p, c)
	a.ProcessUpdate()

	assert.Equal(t, 4, len(a.GetResultList()))

	a.RemoveResult("grandparent")
	a.ProcessUpdate()

	assert.Equal(t, 1, len(a.GetResultList()))
}

//

// any Result can be found by it's ID
func TestFindResultByID(t *testing.T) {
	expected := MakeResult(test_ns, "", "bar")
	expected.ChildrenRealised = true

	a := NewApp()
	a.AppendResults(MakeResult(test_ns, "", "foo"), MakeResult(test_ns, "", "bar"))
	a.ProcessUpdate()

	actual := a.FindResultByID("bar")
	assert.Equal(t, expected, actual)
}

// Demonstrates the two-read race in GetResult.
// GetResult does two reads through app.State: index lookup, then result list access.
// If app.State pointer is swapped between those reads, the index from state N
// is used against the result list from state N+1, causing a mismatch.
//
// This test simulates that by directly manipulating state between reads:
// we capture the old index, apply a state change that reshuffles positions,
// then show that the old index points to the wrong result.
func TestGetResult_IndexResultMismatch(t *testing.T) {
	a := NewApp()

	a.AppendResults(
		MakeResult(test_ns, "r1", "id-1"),
		MakeResult(test_ns, "r2", "id-2"),
		MakeResult(test_ns, "r3", "id-3"),
	)
	a.ProcessUpdate()
	// state: [id-1, id-2, id-3], index: id-1→0, id-2→1, id-3→2

	// capture the index from current state (simulates GetResult's first read)
	old_index := a.State.GetIndex()
	assert.Equal(t, 1, old_index["id-2"]) // id-2 is at position 1

	// now remove id-1, which shifts positions: [id-2, id-3]
	// new index: id-2→0, id-3→1
	a.RemoveResult("id-1")
	a.ProcessUpdate()

	// using the OLD index against the NEW result list:
	// old_index["id-2"] = 1, but new result list position 1 is id-3
	new_results := a.State.GetResults()
	result_at_old_position := new_results[old_index["id-2"]]
	assert.NotEqual(t, "id-2", result_at_old_position.ID,
		"old index position should point to wrong result after state change")
	assert.Equal(t, "id-3", result_at_old_position.ID,
		"position 1 now holds id-3, not id-2")

	// GetResult must handle this by reading both index and results from the same
	// state snapshot. Verify it returns the correct result despite the state change.
	result := a.GetResult("id-2")
	assert.NotNil(t, result)
	assert.Equal(t, "id-2", result.ID)
}

// Demonstrates that an observer receiving diff IDs from one state update
// can't safely look them up via GetResult after a subsequent state update.
// The fix: observers should use the new_results snapshot passed to them.
func TestGetResult_ObserverStaleIDs(t *testing.T) {
	a := NewApp()

	a.AppendResults(
		MakeResult(test_ns, "r1", "id-1"),
		MakeResult(test_ns, "r2", "id-2"),
	)
	a.ProcessUpdate()

	// observer captures diff IDs from each notification
	var captured_added_ids []string
	obs := &testObserver{fn: func(old_results, new_results []Result) {
		diff := DiffResults(old_results, new_results)
		captured_added_ids = append(captured_added_ids, diff.Added...)
	}}
	a.AddObserver(obs)

	// update: add id-3
	a.AppendResults(MakeResult(test_ns, "r3", "id-3"))
	a.ProcessUpdate()

	assert.Contains(t, captured_added_ids, "id-3")

	// another update: remove id-1, add id-4
	// this changes the result list and index entirely
	a.RemoveResult("id-1")
	a.ProcessUpdate()
	a.AppendResults(MakeResult(test_ns, "r4", "id-4"))
	a.ProcessUpdate()

	// captured_added_ids now has IDs from multiple notifications.
	// In the real app, a tk.Async callback from the first notification
	// might not run until after all these updates. Looking up "id-3" still
	// works here (it wasn't removed), but the index positions have shifted.
	// The snapshot approach is immune: observer captures result data at
	// notification time, never looks up from mutable state later.
	result := a.GetResult("id-3")
	assert.NotNil(t, result)
	assert.Equal(t, "id-3", result.ID)

	// id-1 was removed — observer might still have it in captured_added_ids
	// from the first notification. Looking it up now returns nil.
	removed_result := a.GetResult("id-1")
	assert.Nil(t, removed_result, "removed result should return nil")
}

// Demonstrates that observers should use the new_results snapshot passed to them
// rather than looking up IDs from app state, which may have changed.
func TestObserver_SnapshotVsLiveState(t *testing.T) {
	a := NewApp()

	a.AppendResults(
		MakeResult(test_ns, "r1", "id-1"),
		MakeResult(test_ns, "r2", "id-2"),
	)
	a.ProcessUpdate()

	// track what the observer sees vs what app state has at lookup time
	var snapshot_results []Result
	var live_lookup_results []*Result

	obs := &testObserver{fn: func(old_results, new_results []Result) {
		diff := DiffResults(old_results, new_results)
		// capture snapshot from the notification (safe)
		snapshot_idx := map[string]Result{}
		for _, r := range new_results {
			snapshot_idx[r.ID] = r
		}
		for _, id := range diff.Modified {
			if r, ok := snapshot_idx[id]; ok {
				snapshot_results = append(snapshot_results, r)
			}
		}
		// also try live lookup (unsafe — this is what the bug does)
		for _, id := range diff.Modified {
			live_lookup_results = append(live_lookup_results, a.GetResult(id))
		}
	}}
	a.AddObserver(obs)

	// update id-1's item from "r1" to "r1-updated"
	a.UpdateResult("id-1", func(r Result) Result {
		r.Item = "r1-updated"
		return r
	})
	a.ProcessUpdate()

	// in this synchronous test, live lookup works because no interleaving.
	// the snapshot approach is still correct.
	assert.Len(t, snapshot_results, 1)
	assert.Equal(t, "r1-updated", snapshot_results[0].Item)
	assert.Len(t, live_lookup_results, 1)
	assert.NotNil(t, live_lookup_results[0])
	assert.Equal(t, "id-1", live_lookup_results[0].ID)

	// key point: the snapshot captured the exact state at notification time.
	// in the real GUI, the live lookup happens later (via tk.Async) when state
	// may have changed. The snapshot approach is immune to this.
}

// Observers receive shared slices from process_update for performance.
// Observers that need isolation (e.g. the GUI's async tk.Async callbacks)
// must deep-clone the results they capture. This test documents that
// observers sharing data IS the expected behavior at this level.
func TestObserver_SharedSlices(t *testing.T) {
	a := NewApp()

	var obs1_results []Result
	var obs2_results []Result

	obs1 := &testObserver{fn: func(_, new_results []Result) {
		obs1_results = new_results
	}}
	obs2 := &testObserver{fn: func(_, new_results []Result) {
		obs2_results = new_results
	}}

	a.AddObserver(obs1)
	a.AddObserver(obs2)

	a.AppendResults(MakeResult(test_ns, "original", "id-1"))
	a.ProcessUpdate()

	assert.Equal(t, "original", obs1_results[0].Item)
	assert.Equal(t, "original", obs2_results[0].Item)

	// both observers see the same underlying slice
	obs1_results[0].Item = "mutated"
	assert.Equal(t, "mutated", obs2_results[0].Item,
		"observers share the same slice — isolation is the observer's responsibility")
}

func TestFindResultByItem(t *testing.T) {
	expected := MakeResult(test_ns, "foo", "foo")
	expected.ChildrenRealised = true

	a := NewApp()
	a.AppendResults(MakeResult(test_ns, "foo", "foo"), MakeResult(test_ns, "foo", "bar"))
	a.ProcessUpdate()

	actual := a.FindResultByItem("foo")
	assert.Equal(t, &expected, actual)
}

/*
//

var FOO_NS = NS{Major: "bw", Minor: "test", Type: "thing"}

type Foo struct {
	Nom string `json:"nom"`
}

func NewFoo() Foo {
	return Foo{Nom: "foo"}
}

func NewFooItem(f Foo) Result {
	return Result{
		ID:   f.Nom,
		NS:   FOO_NS,
		Item: f,
	}
}

func (f Foo) ItemKeys() []string {
	return []string{"Name"}
}

func (f Foo) ItemMap() map[string]string {
	return map[string]string{
		"Name": f.Nom + "!",
	}
}

func (f Foo) ItemHasChildren() ITEM_CHILDREN_LOAD {
	//return ITEM_CHILDREN_LOAD_TRUE // infinite recursion. how to protect against this?
	return ITEM_CHILDREN_LOAD_LAZY
}

func (f Foo) ItemChildren() []Result {
	new_foo := NewFoo()
	new_foo.Nom = f.Nom + ".o" // "foo", "fooo" "foooo"
	return []Result{
		NewFooItem(new_foo),
	}
}

func _Test_realise_children(t *testing.T) {
	foo_item := NewFooItem(NewFoo())
	app := NewApp()
	app.SetResults(foo_item)

	expected := []Result{foo_item}
	assert.Equal(t, expected, app.GetResultList())

	actual := realise_children(app, foo_item)

	fmt.Println("actual:", QuickJSON(actual))

	foo_item.ChildrenRealised = true
	foo_child := NewFoo()
	foo_child.Nom = "foo.o"
	foo_child_item := NewFooItem(foo_child)
	foo_child_item.ParentID = foo_item.ID
	expected = []Result{
		foo_child_item,
		foo_item,
	}

	fmt.Println("expected:", QuickJSON(expected))

	assert.Equal(t, expected, actual)
}
*/
