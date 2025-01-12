package strongbox

import (
	"bw/internal/core"
	"log/slog"
	"strconv"
)

// --- AddonsDir
// todo: rename AddonDir

// a directory containing addons.
// a typical WoW installation will have multiple of these, one for retail, classic, etc.
// a user may have multiple WoW installations.
type AddonsDir struct {
	Path        string      `json:"addon-dir"`
	GameTrackID GameTrackID `json:"game-track"`
	Strict      bool        `json:"strict?"`
}

func (ad AddonsDir) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_URL,
		"GameTrackID",
		"Strict",
	}
}

func (ad AddonsDir) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_URL: "file://" + ad.Path,
		"GameTrackID":       string(ad.GameTrackID),
		"Strict?":           strconv.FormatBool(ad.Strict),
	}
}

func (ad AddonsDir) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (ad AddonsDir) ItemChildren(app *core.App) []core.Result {
	result_list, err := load_addons_dir(ad.Path)
	if err != nil {
		slog.Error("failed to load addons dir: %w", err)
	}
	return result_list
}

var _ core.ItemInfo = (*AddonsDir)(nil)
