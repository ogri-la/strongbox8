package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// a merging of two slices happens as expected
func TestMergeMenu__append(t *testing.T) {
	a := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Open"},
		}},
	}
	b := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Quit"},
		}},
	}

	expected := []Menu{
		{Name: "File", MenuItemList: []MenuItem{
			{Name: "Open"},
			{Name: "Quit"},
		}},
	}
	assert.Equal(t, expected, MergeMenus(a, b))
}

// empty slices don't overwrite
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

// added, modified and deleted IDs are reported in snapshot order, so sibling rows
// keep the order their parent produced them in
func TestDiffResults__snapshot_order_preserved(t *testing.T) {
	old_results := []Result{
		MakeResult(test_ns, "same", "keep-1"),
		MakeResult(test_ns, "old", "change-1"),
		MakeResult(test_ns, "", "delete-1"),
		MakeResult(test_ns, "", "delete-2"),
	}
	new_results := []Result{
		MakeResult(test_ns, "same", "keep-1"),
		MakeResult(test_ns, "new", "change-1"),
		MakeResult(test_ns, "", "add-3"),
		MakeResult(test_ns, "", "add-1"),
		MakeResult(test_ns, "", "add-2"),
	}

	expected := ResultDiff{
		Added:    []string{"add-3", "add-1", "add-2"},
		Modified: []string{"change-1"},
		Deleted:  []string{"delete-1", "delete-2"},
	}
	actual := DiffResults(MakeSnapshot(old_results), MakeSnapshot(new_results))

	assert.Equal(t, expected, actual)
}
