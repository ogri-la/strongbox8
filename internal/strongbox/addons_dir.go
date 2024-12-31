package strongbox

import (
	"bw/internal/core"
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
		"Path",
		"GameTrackID",
		"Strict",
	}
}

func (ad AddonsDir) ItemMap() map[string]string {
	return map[string]string{
		"Path":        ad.Path,
		"GameTrackID": string(ad.GameTrackID),
		"Strict?":     strconv.FormatBool(ad.Strict),
	}
}

func (ad AddonsDir) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (ad AddonsDir) ItemChildren(app *core.App) []core.Result {
	fnresult := core.CallServiceFnWithArgs(app, core.Fn{TheFn: strongbox_addon_dir_load}, core.FnArgs{
		ArgList: []core.KeyVal{
			{
				Key: "addon-dir",
				Val: ad.Path,
			},
		},
	})
	return fnresult.Result
}

var _ core.ItemInfo = (*AddonsDir)(nil)
