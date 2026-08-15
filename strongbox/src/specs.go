package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"reflect"

	z "github.com/Oudwins/zog"
)

// prints the given `data` and each of its validation issues to stdout.
func PrintSpecErr(err z.ZogIssueMap, data any) {
	fmt.Printf("Given:\n%v%v\n", reflect.TypeOf(data), core.QuickJSON(data))
	fmt.Println("Errors:")
	for field, issue_list := range err {
		if field == "$first" {
			continue
		}
		for _, issue := range issue_list {
			fmt.Printf(" Field '%v' (%v) %v: '%v'\n", field, issue.Dtype, issue.Message, issue.Value)
		}
	}
	//panic("nfo has issues")
}

// ---

func FlexStringSchema() *z.StringSchema[FlexString] {
	s := &z.StringSchema[FlexString]{}
	return s
}

var source_map_schema = z.Struct(z.Shape{
	"Source":   z.String().Required().OneOf(SUPPORTED_HOSTS_LIST),
	"SourceID": FlexStringSchema().Required(),
})

// --- NFO

// a complete nfo: the per-addon data written to an addon directory as `.strongbox.json`.
var _nfo_schema = z.Struct(z.Shape{
	"InstalledVersion":     z.String().Required(),
	"Name":                 z.String().Required(),
	"GroupID":              z.String().Required(),
	"Primary":              z.Bool(),
	"Source":               z.String().Required().OneOf(SUPPORTED_HOSTS_LIST),
	"InstalledGameTrackID": z.String().Required().OneOf(SUPPORTED_GAME_TRACKS_LIST),
	"SourceID":             FlexStringSchema().Required(),
	"SourceMapList":        z.Slice(source_map_schema),
	"Ignored":              z.Ptr(z.Bool().Optional()),
	"PinnedVersion":        z.String().Optional(),
})

// a partial nfo carrying grouping data only, written when the source data needed for a
// complete nfo is missing. every other field must be empty.
// `Pick` on `_nfo_schema` won't do: the remaining fields must be asserted empty, not
// merely dropped. the Clojure implementation needed its `limit-keys` macro for the
// same reason.
var _nfo_just_grouped_schema = z.Struct(z.Shape{
	"InstalledVersion":     z.String().Len(0),
	"Name":                 z.String().Len(0),
	"GroupID":              z.String().Required(),
	"Primary":              z.Bool(),
	"Source":               z.String().Len(0),
	"InstalledGameTrackID": z.String().Len(0),
	"SourceID":             FlexStringSchema().Len(0),
	"SourceMapList":        z.Slice(source_map_schema).Len(0),
	"Ignored":              z.Ptr(z.Bool().Optional()),
	"PinnedVersion":        z.String().Optional(),
})

func (nfo *NFO) Valid() z.ZogIssueMap {
	err1 := _nfo_schema.Validate(nfo)
	if err1 != nil {
		err2 := _nfo_just_grouped_schema.Validate(nfo)
		if err2 != nil {
			slog.Info("nfo not valid under full nor partial schema")
			PrintSpecErr(err1, nfo)
			slog.Error("nfo not valid under partial schema")
			PrintSpecErr(err2, nfo)
			return err2
		}
	}
	return nil
}
