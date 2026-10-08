package strongbox

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	z "github.com/Oudwins/zog"
)

// returns the validation issues in `issues` as a single line, "field: message; ...".
func format_zog_issues(issues z.ZogIssueMap) string {
	bits := []string{}
	for field, issue_list := range issues {
		if field == "$first" {
			continue
		}
		for _, issue := range issue_list {
			bits = append(bits, fmt.Sprintf("%v: %v (%v)", field, issue.Message, issue.Value))
		}
	}
	slices.Sort(bits)
	return strings.Join(bits, "; ")
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
// strongbox only writes live hosts and game tracks. reading is more forgiving, see
// `valid_nfo_for_read`.
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

// returns the issues that stop `nfo` being written: it must be a complete nfo or a
// grouping-only nfo. nil when it is valid.
func (nfo *NFO) Valid() z.ZogIssueMap {
	err1 := _nfo_schema.Validate(nfo)
	if err1 == nil {
		return nil
	}
	err2 := _nfo_just_grouped_schema.Validate(nfo)
	if err2 == nil {
		return nil
	}
	slog.Debug("nfo is neither complete nor grouping-only", "complete-issues", format_zog_issues(err1), "grouping-issues", format_zog_issues(err2))
	return err1
}
