package core

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// Result.Tags are just maps and modifying a pass-by-balue Result is modifying the tags everywhere, so they always pass a deep equality test.
// what else is silently going under the radar?
// if core.Result.Item is a struct with a map that has changes, is it captured?? tests!!!

var test_ns = NewNS("bw", "test", "ns")

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

	a.AddResults(r2)
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

	a.AddResults([]Result{
		{
			NS:               test_ns,
			ID:               "dummy-id1",
			Item:             "",
			ChildrenRealised: false, // !important
			Tags:             mapset.NewSet[Tag](),
		},
	}...)

	a.AddResults([]Result{
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
	a.AddResults(MakeResult(test_ns, "foo", "dummy-id1"))
	a.ProcessUpdate()

	a.AddResults(MakeResult(test_ns, "bar", "dummy-id1"))
	a.ProcessUpdate()

	assert.Equal(t, expected, a.GetResultList())
}

// duplicate items replace results
func TestAppSetResults(t *testing.T) {
	r := MakeResult(test_ns, "bar", "dummy-id")
	r.ChildrenRealised = true
	expected := []Result{r}

	a := NewApp()
	a.SetResults(MakeResult(test_ns, "foo", "dummy-id"))
	a.ProcessUpdate()

	a.SetResults(MakeResult(test_ns, "bar", "dummy-id"))
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
	a.SetResults(MakeResult(test_ns, "foo", "dummy-id1"))
	a.ProcessUpdate()

	a.SetResults(MakeResult(test_ns, "bar", "dummy-id1"), MakeResult(test_ns, "baz", "dummy-id2"))
	a.ProcessUpdate()

	assert.Equal(t, expected, a.StateRoot())
}

// a Result can be removed by ID
func TestAppRemoveResult(t *testing.T) {
	r := MakeResult(test_ns, "bar", "dummy-id1")

	a := NewApp()
	a.AddResults(r)
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
	a.AddResults(s, gp, p, c)
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
	a.AddResults(MakeResult(test_ns, "", "foo"), MakeResult(test_ns, "", "bar"))
	a.ProcessUpdate()

	actual := a.FindResultByID("bar")
	assert.Equal(t, expected, actual)
}

//

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

// ---

func TestMergeMenu__append(t *testing.T) {
	a := []Menu{
		{Name: "File"},
	}
	b := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}

	expected := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}

func TestMergeMenu__empty_slice(t *testing.T) {
	a := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}
	b := []Menu{
		{Name: "File", MenuItemList: []MenuItem{}},
	}

	expected := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}

// updates in 'b' don't affect order
func TestMergeMenu__order_preserved(t *testing.T) {
	a := []Menu{
		{Name: "File"},
		{Name: "View"},
	}
	b := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}

	expected := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
		{Name: "View"},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}

// duplicates coalesce
func TestMergeMenu__duplicates(t *testing.T) {
	a := []Menu{
		{Name: "File"},
	}
	b := []Menu{
		{Name: "File"},
		{Name: "File"},
	}

	expected := []Menu{
		{Name: "File"},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}

// duplicates within the same slice will mask. don't do this
func TestMergeMenu__duplicates_dont_mask(t *testing.T) {
	a := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Foo"},
		}},
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Bar"},
		}},
	}
	b := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Baz"},
		}},
	}

	expected := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Foo"},
		}},
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Bar"},
			{Name: "Baz"},
		}},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}
