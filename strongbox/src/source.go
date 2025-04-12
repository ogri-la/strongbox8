package strongbox

import "bw/core"

type AddonSource interface {
	// todo: rename
	// fetches available releases for given `addon`
	ExpandSummary(app *core.App, addon Addon) ([]SourceUpdate, error)

	// downloads an addon for an AddonSource.
	DownloadUpdate(app *core.App, addon Addon) (output_path string, err error)
}
