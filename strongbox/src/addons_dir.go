package strongbox

import (
	"bw/core"
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
	Strict      bool        `json:"strict"` // new in 8.0 (removed '?')

	// deprecated, use `Strict` instead
	StrictPtr *bool `json:"strict?,omitempty"`
}

func NewAddonsDir() AddonsDir {
	return AddonsDir{
		GameTrackID: GAMETRACK_RETAIL,
		Strict:      true,
	}
}

func (ad AddonsDir) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
		"GameTrackID",
		"Strict",
	}
}

func (ad AddonsDir) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_NAME: ad.Path,             // "/path/to/addons/dir"
		core.ITEM_FIELD_URL:  "file://" + ad.Path, // "file:///path/to/addons/dir"
		"GameTrackID":        string(ad.GameTrackID),
		"Strict":             strconv.FormatBool(ad.Strict),
	}
}

func (ad AddonsDir) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (ad AddonsDir) ItemChildren(app *core.App) []core.Result {
	result_list, err := load_addons_dir(ad)
	if err != nil {
		slog.Error("failed to load addons dir", "error", err)
	}
	return result_list
}

var _ core.ItemInfo = (*AddonsDir)(nil)

// ---

func select_addons_dir(app *core.App, addons_dir AddonsDir) {

}
