package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"reflect"

	z "github.com/Oudwins/zog"
)

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

// specs/:addon/-nfo
// ;; nfo files contain extra per-addon data written to addon directories as .strongbox.json.
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

// can't do this because we also need the other fields to be empty:
// var _nfo_just_grouped_schema = _nfo_schema.Pick("GroupID", "Primary", "Ignored", "PinnedVersion")
// even in clojure.spec with our specs.clj, we had to have the `limit-keys` macro to pare away empty fields
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

