package strongbox

import (
	"bw/core"
	"log/slog"
	"strconv"
	"sync"
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

// updates application state to select the given `addons_dir`,
// but only if the `addons_dir` already exists.
// hints GUI to expand result's children.
// DOES NOT save state.
func select_addons_dir(app *core.App, addons_dir AddonsDir) {
	app.UpdateState(func(old_state core.State) core.State {
		var settings *core.Result
		var ad *core.Result
		for idx, r := range old_state.Root.Item.([]core.Result) {
			r := r
			if settings == nil && r.ID == ID_SETTINGS {
				settings = &r
				s := r.Item.(Settings)
				s.Preferences.SelectedAddonsDir = addons_dir.Path
				r.Item = s
				old_state.Root.Item.([]core.Result)[idx] = r
			}
			if ad == nil {
				i, is_ad := r.Item.(AddonsDir)
				if is_ad && i.Path == addons_dir.Path {
					ad = &r
					old_state.Root.Item.([]core.Result)[idx].Tags.Add(core.TAG_SHOW_CHILDREN)
				}
			}
			if settings != nil && ad != nil {
				break
			}
		}
		return old_state
	}).Wait()
}

// updates application state to insert a new addons directory at `path`.
// DOES NOT save state.
func CreateAddonsDir(app *core.App, path PathToDir) *sync.WaitGroup {
	return app.UpdateResult(ID_SETTINGS, func(r core.Result) core.Result {
		settings := r.Item.(Settings)
		ad := NewAddonsDir()
		ad.Path = path
		settings.AddonsDirList = append(settings.AddonsDirList, ad)
		r.Item = settings
		return r
	})
}

// removes any addons dirs with the given `path`,
// also de-selecting the selected addons dir if it equals `path`
// DOES NOT save state.
func RemoveAddonsDir(app *core.App, path PathToDir) *sync.WaitGroup {

	// so, not working: item remains in gui. why?
	// the addons dir is now missing so it should generate a delete event, etc

	return app.UpdateState(func(old_state core.State) core.State {
		slog.Info("removing addons dir", "path", path)
		rl := old_state.Root.Item.([]core.Result)

		index := old_state.GetIndex()[ID_SETTINGS]
		r := rl[index]
		settings := r.Item.(Settings)

		// update the preferences
		new_addons_dirs := []AddonsDir{}
		for _, ad := range settings.AddonsDirList {
			if ad.Path != path {
				new_addons_dirs = append(new_addons_dirs, ad)
			}
		}
		settings.AddonsDirList = new_addons_dirs

		if settings.Preferences.SelectedAddonsDir == path {
			settings.Preferences.SelectedAddonsDir = ""
		}

		r.Item = settings
		rl[index] = r

		// remove the addon directory (if it exists)
		new_results := []core.Result{}
		for _, r := range rl {
			ad, is_ad := r.Item.(AddonsDir)
			if is_ad && ad.Path == path {
				continue // exclude
			}
			new_results = append(new_results, r)
		}

		old_state.Root.Item = new_results
		return old_state
	})
}
