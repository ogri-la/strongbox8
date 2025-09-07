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
