package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock struct that implements ItemInfo
type MockItemInfo struct {
	name string
}

func (m MockItemInfo) ItemKeys() []string {
	return []string{"name"}
}

func (m MockItemInfo) ItemMap() map[string]string {
	return map[string]string{"name": m.name}
}

func (m MockItemInfo) ItemHasChildren() ITEM_CHILDREN_LOAD {
	return ITEM_CHILDREN_LOAD_FALSE
}

func (m MockItemInfo) ItemChildren(*App) []Result {
	return []Result{}
}

// Mock struct that does NOT implement ItemInfo
type MockNonItemInfo struct {
	value string
}

func TestHasItemInfo(t *testing.T) {
	// Test with a struct that implements ItemInfo
	itemInfoStruct := MockItemInfo{name: "test"}
	assert.True(t, HasItemInfo(itemInfoStruct), "MockItemInfo should implement ItemInfo")

	// Test with a struct that doesn't implement ItemInfo
	nonItemInfoStruct := MockNonItemInfo{value: "test"}
	assert.False(t, HasItemInfo(nonItemInfoStruct), "MockNonItemInfo should not implement ItemInfo")

	// Test with basic types
	assert.False(t, HasItemInfo("string"), "string should not implement ItemInfo")
	assert.False(t, HasItemInfo(42), "int should not implement ItemInfo")
	assert.False(t, HasItemInfo([]string{}), "slice should not implement ItemInfo")
}

func TestItemChildrenLoadConstants(t *testing.T) {
	assert.Equal(t, ITEM_CHILDREN_LOAD("load"), ITEM_CHILDREN_LOAD_TRUE)
	assert.Equal(t, ITEM_CHILDREN_LOAD("do-not-load"), ITEM_CHILDREN_LOAD_FALSE)
	assert.Equal(t, ITEM_CHILDREN_LOAD("lazy-load"), ITEM_CHILDREN_LOAD_LAZY)
}

func TestItemFieldConstants(t *testing.T) {
	assert.Equal(t, ItemField("name"), ITEM_FIELD_NAME)
	assert.Equal(t, ItemField("description"), ITEM_FIELD_DESC)
	assert.Equal(t, ItemField("url"), ITEM_FIELD_URL)
	assert.Equal(t, ItemField("version"), ITEM_FIELD_VERSION)
	assert.Equal(t, ItemField("updated-date"), ITEM_FIELD_DATE_UPDATED)
	assert.Equal(t, ItemField("created-date"), ITEM_FIELD_DATE_CREATED)
}

func TestMockItemInfo(t *testing.T) {
	mock := MockItemInfo{name: "test-name"}

	// Test ItemKeys
	keys := mock.ItemKeys()
	assert.Equal(t, []string{"name"}, keys)

	// Test ItemMap
	itemMap := mock.ItemMap()
	expected := map[string]string{"name": "test-name"}
	assert.Equal(t, expected, itemMap)

	// Test ItemHasChildren
	hasChildren := mock.ItemHasChildren()
	assert.Equal(t, ITEM_CHILDREN_LOAD_FALSE, hasChildren)

	// Test ItemChildren
	children := mock.ItemChildren(nil) // App can be nil for this test
	assert.Equal(t, []Result{}, children)
}
