package strongbox

import (
	"bw/core"
	"errors"
	"log/slog"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

/*
   addon data loading, merging, wrangling.
*/

// --- Source Map
// used to know where an addon came from and other locations it may live.

type SourceMap struct {
	Source   Source     `json:"source"`
	SourceID FlexString `json:"source-id"`
}

// --- Source Updates
// extra data a source (wowinterface, github, etc) provides about an addon.

// todo: rename 'release' or similar? release.type: 'lib', 'nolib'. release.stability: 'stable', 'beta', 'alpha', etc.
type SourceUpdate struct {
	//Type string // lib, nolib
	//Stability string // beta, alpha, etc
	//ReleaseJSON ReleaseJSON
	Version          string `json:"version"`
	DownloadURL      string
	GameTrackIDSet   mapset.Set[GameTrackID] // the game tracks this update supports
	InterfaceVersion int
	PublishedDate    time.Time // when was this update made available

	//---

	AssetName string // an update is essentially a remote file that will be unzipped. this is that file's name.
}

// --- InstalledAddon
// the collection of .toc data and .strongbox.json data for an addon directory.

type InstalledAddon struct {
	URL string

	// an addon may have many .toc files, keyed by game track.
	// the toc data eventually used is determined by the selected addon dir's game track.
	TOCMap map[GameTrackID]TOC // required, >= 1

	// an installed addon has zero or one `strongbox.json` 'nfo' files,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	// new in v8, all nfo data is now a list
	NFOList []NFO // optional

	// --- derived fields

	Name           string                  // derived from NFOList[0].Name, see NewInstalledAddon
	GametrackIDSet mapset.Set[GameTrackID] // derived from TOCMap keys
}

func NewInstalledAddon(url string, toc_map map[GameTrackID]TOC, nfo_list []NFO) *InstalledAddon {
	ia := &InstalledAddon{
		URL:            url,
		TOCMap:         toc_map,
		NFOList:        nfo_list,
		GametrackIDSet: mapset.NewSetFromMapKeys[GameTrackID](toc_map),
	}

	// warning! non-deterministic code
	for _, val := range toc_map {
		ia.Name = val.Name
		break
	}

	return ia
}

var _ core.ItemInfo = (*InstalledAddon)(nil)

func (a InstalledAddon) SomeTOC() (TOC, error) {
	var some_toc TOC
	if len(a.TOCMap) < 1 {
		return some_toc, errors.New("InstalledAddon has an empty tocmap")
	}
	for _, toc := range a.TOCMap {
		some_toc = toc
		break
	}
	return some_toc, nil
}

// an InstalledAddon has 1+ .toc files that can be loaded immediately.
func (ia InstalledAddon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (ia InstalledAddon) ItemKeys() []string {
	return []string{
		"source",
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
	}
}

func (ia InstalledAddon) ItemMap() map[string]string {
	row := map[string]string{
		"source":             "",
		core.ITEM_FIELD_NAME: ia.Name,
		core.ITEM_FIELD_URL:  ia.URL,
	}
	return row
}

// the natural children for an InstalledAddon,
// from the pov of an addon manager,
// are .toc files.
func (ia InstalledAddon) ItemChildren(_ *core.App) []core.Result {
	toc_result_list := []core.Result{}
	for _, toc := range ia.TOCMap {
		toc_result := core.NewResult(NS_TOC, toc, core.UniqueID())
		toc_result_list = append(toc_result_list, toc_result)
	}
	return toc_result_list
}

// --- Addon

// an 'addon' represents one or a group of installed addons.
// the group has a representative 'primary' addon.
// the majority of it's fields are derived from it's constituents. See `NewAddon`.
type Addon struct {
	InstalledAddonGroup []InstalledAddon // required >= 1
	CatalogueAddon      *CatalogueAddon  // optional, the catalogue match
	SourceUpdateList    []SourceUpdate
	AddonsDir           *AddonsDir // where the `InstalledAddonGroup` came from. Also gives us GameTrackID and Strict

	// --- fields derived from the above.

	Primary      *InstalledAddon // required, one of Addon.AddonGroup
	NFO          *NFO            // optional, Addon.Primary.NFO[-1] // todo: make this a list of NFO
	TOC          *TOC            // required, Addon.Primary.TOC[$gametrack]
	SourceUpdate *SourceUpdate   // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
	Ignored      bool            // required, `Addon.Primary.NFO[-1].Ignored` or `Addon.Primary.TOC[$gametrack].Ignored`

	// an addon may support many game tracks.
	// a SourceUpdate may support many game tracks.
	// many SourceUpdates may support many game tracks between them.
	// these can be collapsed to a single game track and a filtered set of source updates if a AddonsDirGameTrackID is provided.
	// this value comes from `AddonsDir.AddonsDirGameTrackID`.
	AddonsDirGameTrackID *GameTrackID // `AddonsDir.GameTrackID`
	Strict               bool         // `AddonsDir.Strict`

	// --- formerly only accessible for Addon.Attr.
	// for now these values are just the stringified versions of the original values. may change!

	ID       string
	Source   Source
	SourceID string
	DirName  string
	//Title            string // ???
	Name             string // normalised label
	Label            string // preferred label
	Description      string
	URL              string
	Tags             string
	Created          string
	Updated          string
	Size             string
	PinnedVersion    string
	InstalledVersion string
	AvailableVersion string // Addon.SourceUpdate.Version, previously just 'Version'
	GameVersion      string
	InterfaceVersion string
}

// `NewAddon` helper. Find the correct `TOC` data file given a bunch of conditions.
func _new_addon_find_toc(game_track_id *GameTrackID, primary_addon *InstalledAddon, strict bool) *TOC {
	var final_toc *TOC

	if game_track_id == nil || primary_addon == nil {
		return final_toc
	}

	gt_pref_list := GAMETRACK_PREF_MAP[*game_track_id]

	// set a toc file. only possible when we have a primary addon and a game track id.
	if game_track_id != nil && primary_addon != nil {
		if strict {
			// in strict mode, toc data for selected game track is either present or it's not.
			toc, present := primary_addon.TOCMap[*game_track_id]
			if present {
				final_toc = &toc
			}
		} else {
			// in relaxed mode, if there is *any* toc data it will be used.
			// use the preference map to decide the best one to use.
			for _, gt := range gt_pref_list {
				toc, present := primary_addon.TOCMap[gt]
				if present {
					final_toc = &toc
					break
				}
			}
		}
	}
	return final_toc
}

// given a list of updates, a game track and a strictness flag,
// return the best update available.
// assumes list of updates is sorted newest to oldest.
func _new_addon_pick_source_update(source_update_list []SourceUpdate, game_track_id *GameTrackID, strict bool) *SourceUpdate {
	var empty_result *SourceUpdate

	if game_track_id == nil || len(source_update_list) == 0 {
		return empty_result
	}

	if strict {
		for _, source_update := range source_update_list {
			if source_update.GameTrackIDSet.Contains(*game_track_id) {
				return &source_update
			}
		}
	} else {
		game_track_pref_list := GAMETRACK_PREF_MAP[*game_track_id]
		for _, game_track_id_pref := range game_track_pref_list {
			for _, source_update := range source_update_list {
				if source_update.GameTrackIDSet.Contains(game_track_id_pref) {
					return &source_update
				}
			}
		}
	}

	return empty_result
}

// mega constructor for the complex struct `Addon`.
// keep it simple and farm complex bits out to testable functions.
// previously this logic was a series of disparate deep-merges into a single 'addon' map.
// It was this unmaintainable blob of data that caused me to switch languages.
// I developed myself into a corner and a rebuild seemed necessary.
func NewAddon(addons_dir AddonsDir, installed_addon_list []InstalledAddon, primary_addon *InstalledAddon, nfo *NFO, catalogue_addon *CatalogueAddon, source_update_list []SourceUpdate) Addon {
	a := Addon{
		InstalledAddonGroup: installed_addon_list,
		Primary:             primary_addon,
		CatalogueAddon:      catalogue_addon,
		SourceUpdateList:    source_update_list,
		AddonsDir:           &addons_dir,

		// --- fields we can derive immediately

		AddonsDirGameTrackID: &addons_dir.GameTrackID,
		Strict:               addons_dir.Strict,
		NFO:                  nfo,
	}

	// sanity checks
	if addons_dir == (AddonsDir{}) {
		slog.Error("an `Addon` is tied to a specific `AddonsDir` and cannot be empty")
		panic("programming error")
	}

	if len(installed_addon_list) == 0 && primary_addon != nil {
		slog.Error("no list of installed addons given, yet a primary addon was specified")
		panic("programming error")
	}

	a.TOC = _new_addon_find_toc(a.AddonsDirGameTrackID, primary_addon, a.Strict)

	// ---

	has_toc := a.TOC != nil
	has_nfo := a.NFO != nil
	has_match := a.CatalogueAddon != nil
	has_updates_available := len(source_update_list) > 0
	has_game_track := a.AddonsDirGameTrackID != nil

	if has_nfo {
		a.Ignored = NFOIgnored(*nfo)
	}

	// pick a `SourceUpdate` from a list of updates.
	if has_updates_available && has_game_track {
		// choose a specific update from a list of updates.
		// assumes `source_update_list` is sorted newest to oldest.
		a.SourceUpdate = _new_addon_pick_source_update(source_update_list, a.AddonsDirGameTrackID, a.Strict)
	}

	has_update := a.SourceUpdate != nil

	// raw title. does anything use this?
	/*
		if has_match {
			a.Title = a.CatalogueAddon.Label // "AdiBags"
		} else {
			// may be empty, use "label" for a safer "title"
			a.Title = a.TOC.Title
		}
	*/

	// human friendly addon title
	if has_match {
		a.Label = a.CatalogueAddon.Label // "AdiBags"
	} else if has_toc {
		a.Label = a.TOC.Label // "AdiBags", "Group Title *"
	}

	// normalised title
	if has_match {
		a.Name = a.CatalogueAddon.Name // "adibags"
	} else if has_nfo {
		a.Name = a.NFO.Name
	} else if has_toc {
		a.Name = a.TOC.Name
	} else {
		a.Name = primary_addon.Name
	}

	// description
	if has_match {
		a.Description = a.CatalogueAddon.Description
	} else if has_toc {
		a.Description = a.TOC.Notes
	}

	// url
	if has_match {
		a.URL = a.CatalogueAddon.URL
	}

	if has_toc {
		a.DirName = a.TOC.DirName
	}

	// interface-version "100105", "30402"
	if has_toc {
		a.InterfaceVersion = core.IntToString(a.TOC.InterfaceVersion)
	}

	// case "game-version": // "10.1.5", "3.4.2"
	if has_toc {
		v, err := InterfaceVersionToGameVersion(a.TOC.InterfaceVersion)
		if err == nil {
			a.GameVersion = v
		}
	}

	// case "installed-version": // v1.2.3, foo-bar.zip.v1, 10.12.0v1.4.2, 12312312312
	if has_nfo {
		a.InstalledVersion = a.NFO.InstalledVersion
	} else if has_toc {
		a.InstalledVersion = a.TOC.InstalledVersion
	}

	// case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
	if has_update {
		a.AvailableVersion = a.SourceUpdate.Version
	} else if has_toc {
		a.AvailableVersion = a.TOC.InstalledVersion
	}

	// case "source":
	if has_match {
		a.Source = a.CatalogueAddon.Source
	} else if has_nfo {
		a.Source = a.NFO.Source
	} else {
		// can we do anything with a.TOC.SourceMapList if one exists?
		// I guess those are just *potential* sources rather than the *actual* source.
	}

	//case "source-id":
	if has_match {
		a.SourceID = string(a.CatalogueAddon.SourceID)
	} else if has_nfo {
		a.SourceID = string(a.NFO.SourceID)
	}

	// case "updated":
	if has_match || has_update {
		if has_update {
			a.Updated = core.FormatDateTime(a.SourceUpdate.PublishedDate)
		} else if has_match {
			a.Updated = a.CatalogueAddon.UpdatedDate
		}
	}

	return a
}

var _ core.ItemInfo = (*Addon)(nil)

func (a Addon) ItemKeys() []string {
	return []string{
		"source",
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		core.ITEM_FIELD_URL,
		"tags",
		core.ITEM_FIELD_DATE_CREATED,
		core.ITEM_FIELD_DATE_UPDATED,
		"size",
		"installed-version",
		"available-version",
		"game-version",
	}
}

func (a Addon) ItemMap() map[string]string {
	return map[string]string{
		"source":                     a.Source,
		core.ITEM_FIELD_NAME:         a.DirName,
		core.ITEM_FIELD_DESC:         a.Description,
		core.ITEM_FIELD_URL:          a.URL,
		"tags":                       a.Tags,
		core.ITEM_FIELD_DATE_CREATED: a.Created,
		core.ITEM_FIELD_DATE_UPDATED: a.Updated,
		"size":                       a.Size,
		"installed-version":          a.InstalledVersion,
		"available-version":          a.AvailableVersion,
		"game-version":               a.GameVersion,
	}
}

// an Addon may be grouping multiple InstalledAddons.
// if so, they can be loaded immediately.
func (a Addon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (a Addon) ItemChildren(_ *core.App) []core.Result {
	children := []core.Result{}
	for _, installed_addon := range a.InstalledAddonGroup {
		ia_result := core.NewResult(NS_INSTALLED_ADDON, installed_addon, core.UniqueID())
		children = append(children, ia_result)
	}
	return children
}

// clj: addon/updateable?
// an `Addon` may have updates available,
// but other reasons may prevent it from being updated (ignored, pinned, etc).
// returns `true` when given `addon` can be updated to a newer version.
func Updateable(a Addon) bool {
	if a.Ignored {
		return false
	}

	// no updates available to select from
	if len(a.SourceUpdateList) == 0 {
		slog.Info("no updates", "source", a.Source, "id", a.SourceID)
		return false
	}

	// updates available but none selected.
	// this is perfectly normal. the 'retail' game track may be selected but the addon only has 'classic' updates.
	if a.SourceUpdate == nil {
		slog.Info("updates, but none selected")
		return false
	}

	// if addon is pinned ...
	if a.PinnedVersion != "" {
		// ... then it can only be updated if the version installed does not match the pinned version,
		// _and_ the pinned version matches the available version.
		if a.PinnedVersion != a.InstalledVersion {
			return a.PinnedVersion == a.AvailableVersion
		}
	}

	// when versions are equal ...
	// TODO: this whole condition needs a review
	if a.AvailableVersion == a.InstalledVersion {
		// we need to check the available game track vs the installed game track.
		// it is possible there is a '1.0.0' for retail installed but '1.0.0' for classic takes precedence.
		// while it is also possible these differing game tracks belong to the same update,
		// they might also be two separate downloadable files :P
		//if a.SourceUpdate.GameTrackID == a.TOC.GameTrackID {
		if a.SourceUpdate.GameTrackIDSet.Contains(a.TOC.GameTrackID) {
			// game tracks are also the same.
			// so we have a situation where: the available version and installed version are the same,
			// and the currently set game track and installed game track are the same.
			// this is a real edge case, _but_ the game tracks an addon supports may change from underneath it.
			// * the selected addons directory changes it's `AddonsDir.Strict` preference and the `Addon` isn't updated (bad strongbox)
			// * the source of an addon changes (wowi => github) and the same addon with the same version from a different source has been altered (bad addon)
			// * strongbox support for a particular game track is dropped (never happened)
			// * ... ?
			// is it possible this situation isn't possible anymore in 8.0 ?
			//
			// check that this game track is part of the game tracks supported as described in the TOC files.
			//(if (utils/in? game-track supported-game-tracks) false
			if a.Primary.GametrackIDSet.Contains(*a.AddonsDirGameTrackID) {
				// doesn't matter if installed game track doesn't match available game track, the available game track is supported.
				return false
			} else {
				// (not= game-track installed-game-track))
				return a.AddonsDirGameTrackID != &a.NFO.InstalledGameTrackID
			}
		}
	}
	slog.Info("baz")
	return a.AvailableVersion != a.InstalledVersion
}

// ---

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	empty_result := InstalledAddon{}
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		slog.Error("error parsing toc file", "error", err)
		return empty_result, err
	}
	url := "file://" + addon_dir
	nfo_list := ReadNFO(addon_dir)
	return *NewInstalledAddon(url, toc_map, nfo_list), nil
}

// --- public

// addon.clj/load-all-installed-addons
// toc.clj/parse-addon-toc-guard
// reads the toc and nfo data from *all* addons in the given `addon_dir`,
// groups them and returns the result.
func LoadAllInstalledAddons(addons_dir AddonsDir) ([]Addon, error) {
	empty_addon_list := []Addon{}
	dir_list, err := core.DirList(addons_dir.Path)
	if err != nil {
		return empty_addon_list, err
	}

	installed_addon_list := []InstalledAddon{}
	for _, full_path := range dir_list {
		if BlizzardAddon(full_path) {
			continue
		}
		addon, err := load_installed_addon(full_path)
		if err != nil {
			slog.Error("failed to load addon", "error", err)
			continue
		}
		installed_addon_list = append(installed_addon_list, addon)
	}

	// group installed addons // addon.clj/group-addons
	nogroup := ""

	// an installed addon may be part of a bundle.
	// we can only group addons once they've all been loaded and have the group-ids
	installed_addon_groups := core.GroupBy(installed_addon_list, func(installed_addon InstalledAddon) string {
		if len(installed_addon.NFOList) == 0 {
			// no nfo data.
			// it either wasn't found or was bad and ignored.
			return nogroup
		}
		nfo, _ := PickNFO(installed_addon.NFOList)
		return nfo.GroupID
	})

	addon_list := []Addon{}

	// for each group of `InstalledAddon`, create an `Addon` and select the primary
	for group_id, installed_addon_group := range installed_addon_groups {
		installed_addon_group := installed_addon_group

		if group_id == "" {
			// the no-group group.

			// *valid* NFO data requires a GroupID, so treat them all as installed but not strongbox-installed.
			for _, installed_addon := range installed_addon_group {
				installed_addon := installed_addon
				// NFO: not found/bad data/invalid data
				addon := NewAddon(addons_dir, []InstalledAddon{installed_addon}, &installed_addon, nil, nil, nil)
				addon_list = append(addon_list, addon)
			}
			continue
		}

		// regular group

		var final_nfo *NFO
		var final_installed_addon_list []InstalledAddon
		var final_primary *InstalledAddon

		if len(installed_addon_group) == 1 {
			// perfect case, no grouping.
			new_addon_group := []InstalledAddon{installed_addon_group[0]}
			nfo, _ := PickNFO(new_addon_group[0].NFOList)

			final_nfo = &nfo
			final_installed_addon_list = new_addon_group
			final_primary = &new_addon_group[0]
		} else {
			// multiple addons in group
			default_primary := &installed_addon_group[0] // default. todo: sort by NFO for reproducible testing.
			var primary *InstalledAddon

			// read the nfo data to discover the primary
			for _, installed_addon := range installed_addon_group {
				installed_addon := installed_addon
				nfo, _ := PickNFO(installed_addon.NFOList)
				if nfo.Primary {
					if primary != nil {
						slog.Warn("multiple NFO files in addon group are set as the primary. last one wins.")
						// TODO: ensure this isn't propagated
					}
					primary = &installed_addon
				}
			}

			if primary == nil {
				slog.Warn("no NFO files in addon group are set as the primary. first one wins.")
				primary = default_primary
			}

			primary_nfo, _ := PickNFO(primary.NFOList)

			final_nfo = &primary_nfo
			final_primary = primary
			final_installed_addon_list = installed_addon_group
		}

		addon := NewAddon(addons_dir, final_installed_addon_list, final_primary, final_nfo, nil, nil)
		addon_list = append(addon_list, addon)
	}

	return addon_list, nil
}
