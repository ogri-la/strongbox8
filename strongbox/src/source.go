package strongbox

import "bw/core"

// an addon host that can be asked what updates an addon has available.
type AddonSource interface {
	// returns the releases available for the given `source_id`, newest first.
	// todo: rename
	ExpandSummary(app *core.App, source_id string) ([]SourceUpdate, error)
}
