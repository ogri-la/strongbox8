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

func MakeAddonsDir() AddonsDir {
	return AddonsDir{
		GameTrackID: GAMETRACK_RETAIL,
		Strict:      true,
	}
}

func MakeAddonsDirResult(addons_dir AddonsDir) core.Result {
	return core.MakeResult(NS_ADDONS_DIR, addons_dir, addons_dir.Path)
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
	addon_list, err := load_addons_dir(ad)
	if err != nil {
		slog.Error("failed to load addons dir", "error", err)
	}

	result_list := []core.Result{}
	for _, addon := range addon_list {
		result_list = append(result_list, core.MakeResult(NS_ADDON, addon, core.UniqueID()))
	}

	return result_list
}

var _ core.ItemInfo = (*AddonsDir)(nil)

// ---

// updates application state to select the given `addons_dir`,
// but only if the `addons_dir` already exists.
// hints GUI to expand result's children.
// DOES NOT save state.
func SelectAddonsDir(app *core.App, addons_dir AddonsDir) {
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
// DOES NOT save settings.
func CreateAddonsDir(app *core.App, path PathToDir) *sync.WaitGroup {
	return app.UpdateState(func(old_state core.State) core.State {
		i := old_state.GetIndex()[ID_SETTINGS]
		rl := old_state.GetResults()
		r := rl[i]
		settings := r.Item.(Settings)

		// todo: should this be pushed into a validtor? if so, is it safe to assume `path` these exports fns are safe?
		for _, ad := range settings.AddonsDirList {
			if ad.Path == path {
				// AddonsDir with that path already exists. noop
				return old_state
			}
		}

		// create a new addons dir
		ad := MakeAddonsDir()
		ad.Path = path

		// update the settings
		settings.AddonsDirList = append(settings.AddonsDirList, ad)
		r.Item = settings
		rl[i] = r

		// add it to the state (children will be realised)
		adr := MakeAddonsDirResult(ad)
		rl = append(rl, adr)

		// replace state
		old_state.Root.Item = rl
		return old_state
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
