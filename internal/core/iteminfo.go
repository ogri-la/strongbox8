package core

import (
	"log/slog"
	"reflect"
)

/*
   implementing ItemInfo allows us to find fields,
   preferred field order and a way to load children.
*/

type ITEM_CHILDREN_LOAD string

const (
	ITEM_CHILDREN_LOAD_TRUE  ITEM_CHILDREN_LOAD = "load"
	ITEM_CHILDREN_LOAD_FALSE ITEM_CHILDREN_LOAD = "do-not-load"
	ITEM_CHILDREN_LOAD_LAZY  ITEM_CHILDREN_LOAD = "lazy-load"
)

type ItemInfo interface {
	// returns a list of fields available to the table in their preferred order.
	ItemKeys() []string
	// returns a map of fields to their stringified values.
	ItemMap() map[string]string
	// returns how to load children *if* a row has children.
	ItemHasChildren() ITEM_CHILDREN_LOAD
	// returns a list of child rows for this row, if any
	ItemChildren(*App) []Result // has to be a Result so a unique ID+NS can be set :( it would be more natural if a thing could just yield child-things and we wrap them in a Result later. Perhaps instead of Result.Item == any, it equals 'Item' that has a method ID() and NS() ?
}

// returns true if a given `thing` implements `ItemInfo`.
func HasItemInfo(thing any) bool {
	table_row_interface := reflect.TypeOf((*ItemInfo)(nil)).Elem()
	does := reflect.TypeOf(thing).Implements(table_row_interface)
	if !does {
		slog.Debug("thing does NOT implement ItemInfo", "thing-type", reflect.TypeOf(thing)) //  "thing", thing)
	}
	return does
}

// common fields for structs implementing the ItemInfo interface

type ItemField = string

const (
	ITEM_FIELD_NAME         ItemField = "name"
	ITEM_FIELD_DESC         ItemField = "description"
	ITEM_FIELD_URL          ItemField = "url"
	ITEM_FIELD_VERSION      ItemField = "version"
	ITEM_FIELD_DATE_UPDATED ItemField = "updated"
)
