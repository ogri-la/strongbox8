package strongbox

import "bw/core"

type AddonSource interface {
	// todo: rename
	// fetches available releases for given `source_id`
	ExpandSummary(app *core.App, source_id string) ([]SourceUpdate, error)
}
