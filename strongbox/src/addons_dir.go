package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"

	mapset "github.com/deckarep/golang-set/v2"
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

	// derived, never stored
	selected  bool // dynamically set as settings change
	available bool // the directory exists. an unavailable addons dir may be on an unmounted drive
}

// returns `true` when the addons dir's directory existed when it was last checked.
func (ad AddonsDir) Available() bool {
	return ad.available
}

// returns an `AddonsDir` for the given `path`, defaulting to strict retail.
func MakeAddonsDir(path PathToDir) AddonsDir {
	return AddonsDir{
		Path:        path,
		GameTrackID: GAMETRACK_RETAIL,
		Strict:      true,
	}
}

// returns the given `addons_dir` as a `core.Result`, using its path as the result ID.
func MakeAddonsDirResult(addons_dir AddonsDir) core.Result {
	return core.MakeResult(NS_ADDONS_DIR, addons_dir, addons_dir.Path)
}

func (ad AddonsDir) ItemKeys() []string {
	return []string{
		"selected",
		"available",
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
		"GameTrackID",
		"Strict",
	}
}

func (ad AddonsDir) ItemMap() map[string]string {
	return map[string]string{
		"selected":           map[bool]string{true: "true", false: ""}[ad.selected],
		"available":          map[bool]string{true: "", false: "unavailable"}[ad.available],
		core.ITEM_FIELD_NAME: ad.Path,             // "/path/to/addons/dir"
		core.ITEM_FIELD_URL:  "file://" + ad.Path, // "file:///path/to/addons/dir"
		"GameTrackID":        string(ad.GameTrackID),
		"Strict":             strconv.FormatBool(ad.Strict),
	}
}

// the selected addons dir's addons are loaded straight away. other addons dirs load
// theirs when expanded, so starting is not slowed by addons dirs the user isn't using.
// an unavailable addons dir has nothing to load.
func (ad AddonsDir) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	switch {
	case !ad.available:
		return core.ITEM_CHILDREN_LOAD_FALSE
	case ad.selected:
		return core.ITEM_CHILDREN_LOAD_TRUE
	default:
		return core.ITEM_CHILDREN_LOAD_LAZY
	}
}

// the addons in the addons dir, each with a stable ID, see `addons_dir_results`.
// an addons dir that cannot be read has no children and the failure is logged.
func (ad AddonsDir) ItemChildren(app *core.App) []core.Result {
	result_list, err := addons_dir_results(ad)
	if err != nil {
		slog.Error("failed to load addons dir", "addons-dir", ad.Path, "error", err)
	}
	return result_list
}

var _ core.ItemInfo = (*AddonsDir)(nil)

// ---

// --- settings changes
// each change to the addons dirs is a pure function from settings to settings, applied
// with `apply_settings`.

// returns `settings` with an addons dir for `path` added and selected. its game track is
// guessed from the path and it is strict.
// a path that is already an addons dir changes nothing and returns `false`.
func settings_add_addons_dir(settings Settings, path PathToDir) (Settings, bool) {
	path = filepath.Clean(path)
	if slices.ContainsFunc(settings.AddonsDirList, func(ad AddonsDir) bool { return ad.Path == path }) {
		return settings, false
	}
	ad := MakeAddonsDir(path)
	ad.GameTrackID = guess_game_track_from_path(path)
	settings.AddonsDirList = append(slices.Clone(settings.AddonsDirList), ad)
	settings.Preferences.SelectedAddonsDir = path
	return settings, true
}

// returns `settings` with the addons dir at `path` selected, or unchanged and `false`
// when there is no such addons dir.
func settings_select_addons_dir(settings Settings, path PathToDir) (Settings, bool) {
	if !slices.ContainsFunc(settings.AddonsDirList, func(ad AddonsDir) bool { return ad.Path == path }) {
		return settings, false
	}
	settings.Preferences.SelectedAddonsDir = path
	return settings, true
}

// returns `settings` without the addons dir at `path`. when it was selected, the first
// remaining addons dir that is `available` is selected, or none.
func settings_remove_addons_dir(settings Settings, path PathToDir, available func(string) bool) Settings {
	settings.AddonsDirList = slices.DeleteFunc(slices.Clone(settings.AddonsDirList), func(ad AddonsDir) bool { return ad.Path == path })
	if settings.Preferences.SelectedAddonsDir == path {
		settings.Preferences.SelectedAddonsDir = ""
		for _, ad := range settings.AddonsDirList {
			if available(ad.Path) {
				settings.Preferences.SelectedAddonsDir = ad.Path
				break
			}
		}
	}
	return settings
}

// returns `settings` with the addons dir at `path` changed by `fn`.
func settings_update_addons_dir(settings Settings, path PathToDir, fn func(AddonsDir) AddonsDir) Settings {
	ad_list := slices.Clone(settings.AddonsDirList)
	for i, ad := range ad_list {
		if ad.Path == path {
			ad_list[i] = fn(ad)
		}
	}
	settings.AddonsDirList = ad_list
	return settings
}

// returns `settings` with the game track of the addons dir at `path` set to `game_track`.
func settings_set_game_track(settings Settings, path PathToDir, game_track GameTrackID) Settings {
	return settings_update_addons_dir(settings, path, func(ad AddonsDir) AddonsDir {
		ad.GameTrackID = game_track
		return ad
	})
}

// returns `settings` with the strictness of the addons dir at `path` set to `strict`.
func settings_set_strict(settings Settings, path PathToDir, strict bool) Settings {
	return settings_update_addons_dir(settings, path, func(ad AddonsDir) AddonsDir {
		ad.Strict = strict
		return ad
	})
}

// returns `ad` with its derived fields set from `settings` and the filesystem.
func derive_addons_dir(ad AddonsDir, settings Settings, available func(string) bool) AddonsDir {
	ad.selected = settings.Preferences.SelectedAddonsDir == ad.Path
	ad.available = available(ad.Path)
	return ad
}

// returns the addons dir result `r` holding `ad`, tagged to match: expanded when
// selected, muted when unavailable.
func tag_addons_dir_result(r core.Result, ad AddonsDir) core.Result {
	r.Item = ad
	r.Tags.Remove(core.TAG_SHOW_CHILDREN)
	r.Tags.Remove(core.TAG_MUTED)
	if ad.selected {
		r.Tags.Add(core.TAG_SHOW_CHILDREN)
	}
	if !ad.available {
		r.Tags.Add(core.TAG_MUTED)
	}
	return r
}

// returns `state` updated to match `settings`: the settings replaced, each addons dir
// result updated, removed addons dirs dropped along with their addons, new ones added,
// and the addons of an addons dir whose game track or strictness changed re-evaluated.
// a pure function of its inputs, applied by `apply_settings`.
func settings_into_state(old_state core.State, settings Settings, available func(string) bool) core.State {
	ad_idx := map[PathToDir]AddonsDir{} // path => addons dir
	for _, ad := range settings.AddonsDirList {
		ad_idx[ad.Path] = derive_addons_dir(ad, settings, available)
	}

	old_list := old_state.GetResults()
	removed := mapset.NewSet[string]()
	seen := mapset.NewSet[string]()
	changed := map[PathToDir]AddonsDir{} // addons dirs whose addons must be re-evaluated
	new_list := []core.Result{}

	for _, r := range old_list {
		switch item := r.Item.(type) {
		case Settings:
			r.Item = settings
		case AddonsDir:
			ad, present := ad_idx[item.Path]
			if !present {
				removed.Add(r.ID)
				removed = removed.Union(descendent_ids(old_list, r.ID))
				continue
			}
			seen.Add(item.Path)
			if ad.GameTrackID != item.GameTrackID || ad.Strict != item.Strict {
				changed[ad.Path] = ad
			}
			r = tag_addons_dir_result(r, ad)
		}
		new_list = append(new_list, r)
	}

	final_list := []core.Result{}
	for _, r := range new_list {
		if removed.Contains(r.ID) {
			continue
		}
		if a, is_addon := r.Item.(Addon); is_addon && a.AddonsDir != nil {
			if ad, present := changed[a.AddonsDir.Path]; present {
				r.Item = reevaluate_addon(a, ad)
				r = tag_addon_result(r)
			}
		}
		final_list = append(final_list, r)
	}

	for _, ad := range settings.AddonsDirList {
		if !seen.Contains(ad.Path) {
			final_list = append(final_list, tag_addons_dir_result(MakeAddonsDirResult(ad), ad_idx[ad.Path]))
		}
	}

	old_state.Root.Item = final_list
	return old_state
}

// returns the addon `a` composed again for the addons dir `ad`, keeping its catalogue
// match and the updates already found: a different game track or strictness picks a
// different .toc file and update from the same data.
func reevaluate_addon(a Addon, ad AddonsDir) Addon {
	return MakeAddon(ad, a.InstalledAddonGroup, a.Primary, a.NFO, a.CatalogueAddon, a.SourceUpdateList)
}

// replaces the settings in app state with the result of `fn`, updates the addons dirs and
// their addons to match in one state update, and saves the settings.
// returns the new settings.
func apply_settings(app *core.App, fn func(Settings) Settings) Settings {
	var new_settings Settings
	app.UpdateState(func(old_state core.State) core.State {
		settings, err := find_settings(&old_state)
		if err != nil {
			slog.Error("settings not found in state, nothing changed", "error", err)
			return old_state
		}
		new_settings = fn(settings)
		return settings_into_state(old_state, new_settings, core.DirExists)
	}).Wait()
	SaveSettings(app)
	return new_settings
}

// adds an addons dir at `path` and selects it, then saves the settings.
// returns an error when `path` is not a directory. a path that is already an addons dir
// is selected.
func AddAddonsDir(app *core.App, path PathToDir) error {
	if !core.DirExists(path) {
		return fmt.Errorf("not a directory: %s", path)
	}
	path = filepath.Clean(path)
	apply_settings(app, func(s Settings) Settings {
		s, added := settings_add_addons_dir(s, path)
		if !added {
			s, _ = settings_select_addons_dir(s, path)
		}
		return s
	})
	return nil
}

// selects the addons dir at `path`, loading its addons, then saves the settings.
// returns an error when there is no such addons dir, or its directory does not exist.
func SelectAddonsDir(app *core.App, path PathToDir) error {
	if !core.DirExists(path) {
		return fmt.Errorf("addons directory is unavailable: %s", path)
	}
	var found bool
	apply_settings(app, func(s Settings) Settings {
		s, found = settings_select_addons_dir(s, path)
		return s
	})
	if !found {
		return fmt.Errorf("not an addons directory: %s", path)
	}
	return nil
}

// removes the addons dir at `path` from the settings and from app state, then saves the
// settings. nothing on disk is changed.
func RemoveAddonsDir(app *core.App, path PathToDir) {
	slog.Info("removing addons dir", "path", path)
	apply_settings(app, func(s Settings) Settings {
		return settings_remove_addons_dir(s, path, core.DirExists)
	})
}

// sets the game track of the addons dir at `path`, re-evaluates its addons, then saves
// the settings.
// returns an error when `game_track` is not supported.
func SetAddonsDirGameTrack(app *core.App, path PathToDir, game_track GameTrackID) error {
	if !SUPPORTED_GAME_TRACKS.Contains(game_track) {
		return fmt.Errorf("unsupported game track: %s", game_track)
	}
	apply_settings(app, func(s Settings) Settings {
		return settings_set_game_track(s, path, game_track)
	})
	return nil
}

// sets the strictness of the addons dir at `path`, re-evaluates its addons, then saves
// the settings.
func SetAddonsDirStrict(app *core.App, path PathToDir, strict bool) {
	apply_settings(app, func(s Settings) Settings {
		return settings_set_strict(s, path, strict)
	})
}

// --- loading addons into state

// returns the result ID of the installed addon `a` in the addons dir at `addons_dir_path`:
// the same every time the addons dir is loaded, so a reload replaces an addon rather than
// adding it again.
// a strongbox-installed addon is identified by its group ID, any other by its directory.
func addon_result_id(addons_dir_path PathToDir, a Addon) string {
	key := "dir:" + a.DirName
	if a.NFO != nil && a.NFO.GroupID != "" {
		key = "group:" + a.NFO.GroupID
	}
	return "addon:" + addons_dir_path + "#" + key
}

// returns the installed addon `a` as a result with its stable ID, tagged by its state.
func addon_result(a Addon) core.Result {
	r := core.MakeResult(NS_ADDON, a, addon_result_id(a.AddonsDir.Path, a))
	return tag_addon_result(r)
}

// returns `r` with its tags matching the state of the addon it holds: has-update when it
// can be updated, muted when ignored.
func tag_addon_result(r core.Result) core.Result {
	a := r.Item.(Addon)
	r.Tags.Remove(core.TAG_HAS_UPDATE)
	r.Tags.Remove(core.TAG_MUTED)
	if Updateable(a) {
		r.Tags.Add(core.TAG_HAS_UPDATE)
	}
	if a.IsIgnored {
		r.Tags.Add(core.TAG_MUTED)
	}
	return r
}

// returns a result per addon in the addons dir `ad`, each with a stable ID.
// returns an error when the directory cannot be read.
func addons_dir_results(ad AddonsDir) ([]core.Result, error) {
	addon_list, err := LoadAllInstalledAddons(ad)
	if err != nil {
		return []core.Result{}, err
	}
	result_list := []core.Result{}
	for _, a := range addon_list {
		result_list = append(result_list, addon_result(a))
	}
	return result_list, nil
}

// returns the IDs of every descendent of the result `parent_id` in `result_list`.
// a map from parent ID to child IDs, so the tree is walked without nested loops.
func descendent_ids(result_list []core.Result, parent_id string) mapset.Set[string] {
	children := map[string][]string{}
	for _, r := range result_list {
		children[r.ParentID] = append(children[r.ParentID], r.ID)
	}
	found := mapset.NewSet[string]()
	queue := []string{parent_id}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, child_id := range children[id] {
			if found.Add(child_id) {
				queue = append(queue, child_id)
			}
		}
	}
	return found
}

// reads the addons dir `ad` from disk and replaces its addons in app state, in a single
// state update so the GUI never shows the addons dir half loaded.
// addons gone from disk disappear and new ones appear. an addon that was there before,
// by its stable ID, keeps its catalogue match and the updates already found, composed
// again with what is now on disk: reloading after one install must not lose what is
// known about every other addon.
// returns an error when the directory cannot be read, or the addons dir is not in state.
func ReloadAddonsDir(app *core.App, ad AddonsDir) error {
	result_list, err := addons_dir_results(ad)
	if err != nil {
		return fmt.Errorf("failed to load addons dir: %w", err)
	}
	if app.GetResult(ad.Path) == nil {
		return fmt.Errorf("failed to find addons directory in application state: %s", ad.Path)
	}
	for i := range result_list {
		result_list[i].ParentID = ad.Path
	}

	app.UpdateState(func(old_state core.State) core.State {
		old_list := old_state.GetResults()
		known := map[string]Addon{} // result ID => addon as it was
		for _, r := range old_list {
			if a, is_addon := r.Item.(Addon); is_addon {
				known[r.ID] = a
			}
		}
		for i, r := range result_list {
			if old, present := known[r.ID]; present {
				a := r.Item.(Addon)
				r.Item = MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, a.NFO, old.CatalogueAddon, old.SourceUpdateList)
				result_list[i] = tag_addon_result(r)
			}
		}
		stale := descendent_ids(old_list, ad.Path)
		new_list := []core.Result{}
		for _, r := range old_list {
			if stale.Contains(r.ID) {
				continue
			}
			if r.ID == ad.Path {
				// its children are being replaced here, they must not be loaded again
				r.ChildrenRealised = true
			}
			new_list = append(new_list, r)
		}
		old_state.Root.Item = append(new_list, result_list...)
		return old_state
	}).Wait()
	mark_installed_in_state(app)
	return nil
}
