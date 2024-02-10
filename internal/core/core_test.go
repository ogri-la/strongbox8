package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var test_ns = NewNS("bw", "test", "ns")

// new items can be added to results
func TestAppAddResults(t *testing.T) {
	expected_old_state := []Result{}
	expected_new_state := []Result{NewResult(test_ns, "", "dummy-id")}

	a := NewApp()
	old_state := a.StateRoot()

	assert.Equal(t, expected_old_state, old_state)

	a.AddResults(NewResult(test_ns, "", "dummy-id"))

	assert.Equal(t, expected_new_state, a.StateRoot())
	assert.Equal(t, expected_old_state, old_state)
}

// many new items can be added to results
func TestAppAddResults__many(t *testing.T) {
	expected := []Result{
		NewResult(test_ns, "", "dummy-id1"),
		NewResult(test_ns, "", "dummy-id2"),
		NewResult(test_ns, "", "dummy-id3"),
	}

	a := NewApp()
	a.AddResults(NewResult(test_ns, "", "dummy-id1"))
	a.AddResults(NewResult(test_ns, "", "dummy-id2"), NewResult(test_ns, "", "dummy-id3"))

	assert.Equal(t, expected, a.StateRoot())
}

// duplicate items (items sharing an ID) are not added to results.
func TestAppAddResults__duplicates(t *testing.T) {
	expected := []Result{
		NewResult(test_ns, "foo", "dummy-id1"),
	}

	a := NewApp()
	a.AddResults(NewResult(test_ns, "foo", "dummy-id1"))
	a.AddResults(NewResult(test_ns, "bar", "dummy-id1"))

	assert.Equal(t, expected, a.GetResultList())
}

// duplicate items replace results
func TestAppSetResults(t *testing.T) {
	expected := []Result{
		NewResult(test_ns, "bar", "dummy-id"),
	}

	a := NewApp()
	a.SetResults(NewResult(test_ns, "foo", "dummy-id"))

	a.SetResults(NewResult(test_ns, "bar", "dummy-id"))
	assert.Equal(t, expected, a.StateRoot())
}

// new items are added to, and duplicate items replace, results
func TestAppSetResults__many(t *testing.T) {
	expected := []Result{
		NewResult(test_ns, "bar", "dummy-id1"),
		NewResult(test_ns, "baz", "dummy-id2"),
	}

	a := NewApp()
	a.SetResults(NewResult(test_ns, "foo", "dummy-id1"))

	a.SetResults(NewResult(test_ns, "bar", "dummy-id1"), NewResult(test_ns, "baz", "dummy-id2"))
	assert.Equal(t, expected, a.StateRoot())
}

//

func TestFindResultByID(t *testing.T) {
	expected := NewResult(test_ns, "", "bar")

	a := NewApp()
	a.AddResults(NewResult(test_ns, "", "foo"), NewResult(test_ns, "", "bar"))

	actual := a.FindResultByID("bar")
	assert.Equal(t, expected, actual)
}

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

func Test_realise_children(t *testing.T) {
	fi := NewFooItem(NewFoo())
	a := NewApp()
	a.SetResults(fi)

	expected := []Result{fi}
	assert.Equal(t, expected, a.GetResultList())

	res := realise_children(fi)

	fmt.Println("actual:", QuickJSON(res))

	fi.ChildrenRealised = true
	fc := NewFoo()
	fc.Nom = "foo.o"
	fci := NewFooItem(fc)
	fci.Parent = &fi
	expected = []Result{
		fci,
		fi,
	}

	fmt.Println("expected:", QuickJSON(expected))

	assert.Equal(t, expected, res)
}
