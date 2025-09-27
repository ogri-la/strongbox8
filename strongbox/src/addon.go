package strongbox

import (
	"bw/core"
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func NewSourceUpdate() SourceUpdate {
	return SourceUpdate{
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
}

// --- InstalledAddon

// an InstalledAddon captures the data of a single addon directory in an AddonsDir.
// the collection of .toc data and .strongbox.json data for an addon directory.
type InstalledAddon struct {
	URL string

	// an addon may have many .toc files, keyed by filename
	// todo: this will lead to non-deterministic results with many toc files supporting overlapping game tracks
	TOCMap map[PathToFile]TOC // required, >= 1

	// an installed addon has zero or one `strongbox.json` 'nfo' files,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	// new in v8, all nfo data is now a list
	NFOList []NFO // optional

	// --- derived fields

	Name           string // derived, see `NewInstalledAddon`. TODO: rename 'DirName'
	Description    string
	GametrackIDSet mapset.Set[GameTrackID] // the superset of gametracks in each .toc file
}

var _ core.ItemInfo = (*InstalledAddon)(nil)

func NewInstalledAddon() InstalledAddon {
	return InstalledAddon{
		TOCMap:         map[PathToFile]TOC{},
		NFOList:        []NFO{},
		GametrackIDSet: mapset.NewSet[GameTrackID](),
	}
}

func MakeInstalledAddon(url string, toc_map map[GameTrackID]TOC, nfo_list []NFO) *InstalledAddon {
	ia := NewInstalledAddon()
	ia.URL = url
	ia.TOCMap = toc_map
	ia.NFOList = nfo_list
	ia.GametrackIDSet = mapset.NewSetFromMapKeys(toc_map)

	// try to populate derived fields.

	// the data in the NFOList is not great for displaying in a GUI,
	// also, it may not be present,
	// so just grep the toc map instead.

	// without knowing the gametrack of the selected addon dir we can't know which .toc file in the tocmap is best!
	// if just one .toc file exists, it's easy.
	if len(toc_map) == 1 {
		for _, toc := range toc_map {
			if ia.Name == "" { // todo: why not ia.Name = toc.Name? and we need to remove colour formatting here
				ia.Name = toc.DirName // "EveryAddon" derived from "EveryAddon.toc"
			}
			ia.Description = toc.Notes
			break
		}
	} else {
		// some values are identical between toc files
		for _, val := range toc_map {
			ia.Name = val.DirName
			break
		}
	}

	return &ia
}

func (ia *InstalledAddon) IsEmpty() bool {
	return len(ia.TOCMap) == 0
}

func (ia InstalledAddon) SomeTOC() (TOC, error) {
	var some_toc TOC
	if len(ia.TOCMap) < 1 {
		return some_toc, errors.New("InstalledAddon has an empty tocmap")
	}
	for _, toc := range ia.TOCMap {
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
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		core.ITEM_FIELD_URL,
	}
}

func (ia InstalledAddon) ItemMap() map[string]string {
	row := map[string]string{
		core.ITEM_FIELD_NAME: ia.Name,
		core.ITEM_FIELD_DESC: ia.Description,
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
		toc_result := core.MakeResult(NS_TOC, toc, core.UniqueID())
		toc_result_list = append(toc_result_list, toc_result)
	}
	return toc_result_list
}

// --- Addon

// an 'addon' represents one or a group of installed addons.
// the group has a representative 'primary' addon.
// the majority of it's fields are derived from it's constituents. See `MakeAddon`.
type Addon struct {
	InstalledAddonGroup []InstalledAddon // required >= 1
	CatalogueAddon      *CatalogueAddon  // optional, the catalogue match
	SourceUpdateList    []SourceUpdate

	// an addon may support many game tracks.
	// an update to an addon (SourceUpdate) may support many game tracks.
	// many SourceUpdates may support many game tracks between them.
	// these can be collapsed to a filtered set of source updates when given the context of an AddonsDir and it's selected GameTrack and strictness
	AddonsDir *AddonsDir // where the `InstalledAddonGroup` came from

	// --- fields derived from the above.

	Primary      InstalledAddon // required, one of Addon.AddonGroup
	NFO          *NFO           // optional, Addon.Primary.NFO[-1] // todo: make this a list of NFO
	TOC          *TOC           // required, Addon.Primary.TOC[$gametrack]
	SourceUpdate *SourceUpdate  // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
	Ignored      *bool          // required for implicit/explicit ignore. `Addon.Primary.NFO[-1].Ignored` or `Addon.Primary.TOC[$gametrack].Ignored`
	IsIgnored    bool           // resolved from bool ptr
	IsPinned     bool           // Addon.Primary.NFO[-1].PinnedVersion

	// --- formerly only accessible for Addon.Attr.
	// for now these values are just the stringified versions of the original values. may change!

	ID       string
	Source   Source
	SourceID string
	DirName  string // "EveryAddon" in "/path/to/addons/dir/EveryAddon"
	//Title            string // ???
	Name             string // normalised label. todo: rename 'slug' or 'normalised-name' or something.
	Label            string // preferred label
	Description      string
	URL              string
	Tags             []string
	Created          time.Time
	Updated          time.Time
	Size             string
	PinnedVersion    string
	InstalledVersion string
	AvailableVersion string // Addon.SourceUpdate.Version, previously just 'Version'
	GameVersion      string
	InterfaceVersion string
}

var _ core.ItemInfo = (*Addon)(nil)

// `MakeAddon` helper. Find the correct `TOC` data file given a bunch of conditions.
// returns nil if addon has no .toc files matching given `game_track_id`.
func _make_addon__find_toc(game_track_id GameTrackID, primary_addon InstalledAddon, strict bool) *TOC {
	var final_toc *TOC

	if strict {
		// return the first set of toc data that supports the given game track
		for _, toc := range primary_addon.TOCMap {
			toc := toc
			if toc.GameTrackIDSet.Contains(game_track_id) {
				final_toc = &toc
				break
			}
		}
	} else {
		// in relaxed mode, if there is *any* toc data it will be used.
		// use the preference map to decide the best one to use.
		gt_pref_list := GAMETRACK_PREF_MAP[game_track_id]
		for _, gt := range gt_pref_list {
			// return the first set of toc data that supports the given game track
			for _, toc := range primary_addon.TOCMap {
				toc := toc
				if toc.GameTrackIDSet.Contains(gt) {
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
func _make_addon__pick_source_update(source_update_list []SourceUpdate, game_track_id GameTrackID, strict bool) *SourceUpdate {
	var empty_result *SourceUpdate

	if len(source_update_list) == 0 {
		return empty_result
	}

	if strict {
		for _, source_update := range source_update_list {
			if source_update.GameTrackIDSet.Contains(game_track_id) {
				return &source_update
			}
		}
	} else {
		game_track_pref_list := GAMETRACK_PREF_MAP[game_track_id]
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
func MakeAddon(addons_dir AddonsDir, installed_addon_list []InstalledAddon, primary_addon InstalledAddon, nfo *NFO, catalogue_addon *CatalogueAddon, source_update_list []SourceUpdate) Addon {
	a := Addon{
		InstalledAddonGroup: installed_addon_list,
		Primary:             primary_addon,
		CatalogueAddon:      catalogue_addon,
		SourceUpdateList:    source_update_list,
		AddonsDir:           &addons_dir,

		// --- fields we can derive immediately

		NFO: nfo, // assumed to be the NFO of the primary? TODO! shift primary addon selection here
	}

	// sanity checks
	if addons_dir == (AddonsDir{}) {
		slog.Error("an `Addon` is tied to a specific `AddonsDir` and cannot be empty")
		panic("programming error")
	}

	if len(installed_addon_list) == 0 && !primary_addon.IsEmpty() {
		slog.Error("no list of installed addons given, yet a non-empty primary addon was specified")
		panic("programming error")
	}

	a.TOC = _make_addon__find_toc(a.AddonsDir.GameTrackID, primary_addon, a.AddonsDir.Strict)

	// ---

	has_toc := a.TOC != nil
	has_nfo := a.NFO != nil
	has_match := a.CatalogueAddon != nil
	has_updates_available := len(source_update_list) > 0
	has_game_track := a.AddonsDir.GameTrackID != ""

	// 'ignored'
	if has_nfo {
		// the nilable bool field in the nfo data is actually capturing three states:
		// explicitly ignored (true), explicitly unignored (false) and not-ignored (nil)
		// the three states can be collapsed to two for practical reasons here ...
		a.IsIgnored = nfo_ignored(*nfo)
		// but because we use an Addon to derive new NFO, we also need to capture the three states
		a.Ignored = nfo.Ignored
	}

	// 'pinned'
	if has_nfo {
		a.IsPinned = nfo_pinned(*nfo)
	}

	// pick a `SourceUpdate` from a list of updates.
	if has_updates_available && has_game_track {
		// choose a specific update from a list of updates.
		// assumes `source_update_list` is sorted newest to oldest.
		a.SourceUpdate = _make_addon__pick_source_update(source_update_list, a.AddonsDir.GameTrackID, a.AddonsDir.Strict)
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
	a.Label = a.Primary.Name
	if has_match {
		a.Label = a.CatalogueAddon.Label // "AdiBags"
	}
	a.Label = RemoveEscapeSequences(a.Label)

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
	if has_match && a.CatalogueAddon.Description != "" {
		a.Description = a.CatalogueAddon.Description
	} else if has_toc {
		a.Description = a.TOC.Notes
	}

	// url
	if has_match {
		a.URL = a.CatalogueAddon.URL
	}

	// tags
	if has_match {
		a.Tags = a.CatalogueAddon.TagList
	}

	// created date
	if has_match {
		a.Created = a.CatalogueAddon.CreatedDate
	}

	if has_toc {
		a.DirName = a.TOC.DirName
	}

	// "interface-version": [100105, 30402] => "100105, 30402"
	if has_toc {
		ivl := a.TOC.InterfaceVersionSet.ToSlice()
		slices.Sort(ivl)
		ivsl := []string{}
		for _, iv := range ivl {
			ivsl = append(ivsl, core.IntToString(iv))
		}
		a.InterfaceVersion = strings.Join(ivsl, ", ")
	}

	// case "game-version": [100105, 30402] => "10.1.5, 3.4.2"
	if has_toc {
		ivl := a.TOC.InterfaceVersionSet.ToSlice()
		slices.Sort(ivl)

		gvl := []string{}
		for _, iv := range ivl {
			gv, err := InterfaceVersionToGameVersion(iv)
			if err == nil {
				gvl = append(gvl, gv)
			}
		}
		a.GameVersion = strings.Join(gvl, ", ")
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
		// new in 8.0 - if no version available, don't show an available version!
		//a.AvailableVersion = a.TOC.InstalledVersion
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
			a.Updated = a.SourceUpdate.PublishedDate
		} else if has_match {
			a.Updated = a.CatalogueAddon.UpdatedDate
		}
	}

	return a
}

// cli.clj/unique-group-id-from-zip-file
// returns a friendly unique ID for a zipfile based on the file name.
// zipfile need not exist.
func unique_group_id_from_zip_file(zipfile string) string {
	basename := filepath.Base(zipfile)        // "/foo/bar/baz--1-2-3.zip" => "baz--1-2-3.zip"
	ext := filepath.Ext(basename)             // "baz--1-2-3.zip" => ".zip"
	name := strings.TrimSuffix(basename, ext) // "baz--1-2-3.zip" => "baz--1-2-3"
	// random zip files are unlikely to be double-hyphenated,
	// this is something strongbox does for easier tokenisation,
	// but if a strongbox-downloaded .zip is being used, this will strip some noise.
	first_bit := strings.Split(name, "--")[0]                 // "baz--1-2-3" => "baz"
	return fmt.Sprintf("%s-%s", first_bit, core.UniqueIDN(8)) // "baz-928e42d2
}

// `MakeAddon` takes a lot of existing addon data and creates a denormalised/flattened view of it.
// but what if all you have is a .zip file and a directory to install it?
func MakeAddonFromZipfile(addons_dir AddonsDir, zipfile PathToFile) (Addon, error) {
	if !core.FileExists(zipfile) {
		return Addon{}, fmt.Errorf("failed to create Addon from .zip file: file does not exist: %s", zipfile)
	}

	ial := []InstalledAddon{}
	pa := InstalledAddon{}
	nfo := NFO{
		GroupID: unique_group_id_from_zip_file(zipfile),
	}
	sul := []SourceUpdate{}
	a := MakeAddon(addons_dir, ial, pa, &nfo, nil, sul)

	return a, nil
}

func MakeAddonFromCatalogueAddon(addons_dir AddonsDir, ca CatalogueAddon, sul []SourceUpdate) Addon {
	ial := []InstalledAddon{}
	pa := InstalledAddon{}
	nfo := NFO{
		GroupID: ca.URL,
	}
	a := MakeAddon(addons_dir, ial, pa, &nfo, &ca, sul)
	return a
}

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
	var created_str, updated_str string

	if a.Created.IsZero() {
		created_str = ""
	} else if created_formatted, err := core.FormatTimeHumanOffset(a.Created); err != nil {
		slog.Error("failed to format created date", "addon", a.Name, "created", a.Created, "error", err)
		created_str = ""
	} else {
		created_str = created_formatted
	}

	if a.Updated.IsZero() {
		updated_str = ""
	} else if updated_formatted, err := core.FormatTimeHumanOffset(a.Updated); err != nil {
		slog.Error("failed to format updated date", "addon", a.Name, "updated", a.Updated, "error", err)
		updated_str = ""
	} else {
		updated_str = updated_formatted
	}

	return map[string]string{
		"source":                     a.Source,
		core.ITEM_FIELD_NAME:         a.Label,
		core.ITEM_FIELD_DESC:         a.Description,
		core.ITEM_FIELD_URL:          a.URL,
		"tags":                       strings.Join(a.Tags, ", "),
		core.ITEM_FIELD_DATE_CREATED: created_str,
		core.ITEM_FIELD_DATE_UPDATED: updated_str,
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
		ia_result := core.MakeResult(NS_INSTALLED_ADDON, installed_addon, core.UniqueID())
		children = append(children, ia_result)
	}

	// todo: doesn't work with gui. ItemChildren is evaluated once, when the row is added.
	// if the updates don't exist then they won't be shown.
	/*
		for _, source_update := range a.SourceUpdateList {
			su_result := core.MakeResult(NS_SOURCE_UPDATE, source_update, core.UniqueID())
			children = append(children, su_result)
		}
	*/
	return children
}

// clj: addon/updateable?
// an `Addon` may have updates available,
// but other reasons may prevent it from being updated (ignored, pinned, etc).
// returns `true` when given `addon` can be updated to a newer version.
func Updateable(a Addon) bool {
	if a.IsIgnored {
		return false
	}

	// no updates available to select from
	if len(a.SourceUpdateList) == 0 {
		return false
	}

	// updates available but none selected.
	// this is perfectly normal. the 'retail' game track may be selected but the addon only has 'classic' updates.
	if a.SourceUpdate == nil {
		return false
	}

	// if addon is pinned ...
	if a.IsPinned {
		// ... then it can only be updated if the version installed does not match the pinned version,
		// _and_ the pinned version matches the available version.
		if a.PinnedVersion != a.InstalledVersion {
			return a.PinnedVersion == a.AvailableVersion
		}
	}

	// when versions are equal but the gametracks are wonky ...
	// (and (= version installed-version) (and game-track installed-game-track))
	// `game-track` condition captured above with `a.SourceUpdate == nil`
	if (a.SourceUpdate.Version == a.InstalledVersion) && (a.NFO != nil) {
		// (utils/in? game-track supported-game-tracks)
		if a.Primary.GametrackIDSet.Intersect(a.SourceUpdate.GameTrackIDSet).Cardinality() > 0 {
			// covered.
			// the currently installed addon supports one or more of the game tracks supported by the update.
			return false
		}

		// versions equal but the installed addon does not support the gametracks available in the update.
		// consult the nfo data.

		// (not= game-track installed-game-track))
		if a.SourceUpdate.GameTrackIDSet.Contains(a.NFO.InstalledGameTrackID) {
			// there is a disjoint between the .toc data and the .nfo data.
			// the current set of .toc data doesn't support any of the gametracks supported by the update,
			// but the addon was installed under a gametrack supported by the update.
			// bad data? missing data? most likely the AddonsDir or it's strictness level was changed.
			// this would allow a classic-only addon to be installed under a retail gametrack.
			// either way, the versions are the same and the game tracks match, no update needed.
			return false
		}

		// versions equal, installed toc data doesn't support game tracks in update
		// and the game track recorded when the addon was installed isn't covered either.
		// addon is really out of place and needs replacement.
		return true
	}
	return a.SourceUpdate.Version != a.InstalledVersion
}

// ---

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	empty_result := InstalledAddon{}
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		return empty_result, fmt.Errorf("failed to load addon: %w", err)
	}
	url := "file://" + addon_dir
	nfo_list, err := read_nfo_file(addon_dir)
	if err != nil {
		return empty_result, err
	}
	return *MakeInstalledAddon(url, toc_map, nfo_list), nil
}

// if an addon unpacks to multiple directories, which is the 'main' addon?
// a common convention looks like 'Addon[seperator]Subname', for example:
//
//	'Healbot' and 'Healbot_de' or
//	'MogIt' and 'MogIt_Artifact'
//
// DBM is one exception to this as the 'main' addon is 'DBM-Core' (I think, it's definitely the largest)
// 'MasterPlan' and 'MasterPlanA' is another exception
// these exceptions to the rule are easily handled. the rule is:
//  1. if multiple directories,
//  2. assume dir with shortest name is the main addon
//  3. but only if it's a prefix of all other directories
//  4. if case doesn't hold, do nothing and accept we have no 'main' addon"
func determine_primary_subdir(toplevel_dirs mapset.Set[string]) (string, error) {
	// empty set, return an error
	if toplevel_dirs.Cardinality() == 0 {
		return "", fmt.Errorf("empty set")
	}

	// single dir, perfect case
	if toplevel_dirs.Cardinality() == 1 {
		val := toplevel_dirs.ToSlice()[0] // urgh
		return val, nil
	}

	srtd := toplevel_dirs.ToSlice()
	slices.SortStableFunc(srtd, func(a, b string) int {
		return cmp.Compare(len(a), len(b))
	})

	// multiple dirs and one is shorter than all others
	if len(srtd[0]) != len(srtd[1]) {
		// ... and all dirs are prefixed with the entirety of the first toplevel dir toplevel dir
		prefix := srtd[0]
		all_prefixed := true
		for _, toplevel_dir := range srtd[1:] {
			all_prefixed = all_prefixed && strings.HasPrefix(toplevel_dir, prefix)
		}
		if all_prefixed {
			return prefix, nil
		}
	}

	// couldn't reasonably determine the primary directory
	return "", fmt.Errorf("no common directory prefix")
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
		ia, err := load_installed_addon(full_path)
		if err != nil {
			slog.Warn("failed to load addon", "error", err)
			continue
		}
		installed_addon_list = append(installed_addon_list, ia)
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
		nfo, _ := pick_nfo(installed_addon.NFOList)
		return nfo.GroupID
	})

	addon_list := []Addon{}

	// for each group of `InstalledAddon`, create an `Addon` and select the primary
	for group_id, installed_addon_group := range installed_addon_groups {
		installed_addon_group := installed_addon_group

		// TODO: how much of this picking the primary etc can be pushed into MakeAddon?

		if group_id == nogroup {
			// NFO not found/bad or invalid data.
			// valid NFO data requires a GroupID, so treat them all as installed but not strongbox-installed.
			for _, installed_addon := range installed_addon_group {
				installed_addon := installed_addon
				addon := MakeAddon(addons_dir, []InstalledAddon{installed_addon}, installed_addon, nil, nil, nil)
				addon_list = append(addon_list, addon)
			}
		} else {
			// regular group

			var final_nfo *NFO
			var final_installed_addon_list []InstalledAddon
			var final_primary *InstalledAddon

			if len(installed_addon_group) == 1 {
				// perfect case, no grouping.
				nfo, _ := pick_nfo(installed_addon_group[0].NFOList)

				final_nfo = &nfo
				final_installed_addon_list = installed_addon_group
				final_primary = &installed_addon_group[0]
			} else {
				// multiple addons in group
				default_primary := &installed_addon_group[0] // default. todo: sort by NFO for reproducible testing.
				var primary *InstalledAddon

				// read the nfo data to discover the primary
				for _, installed_addon := range installed_addon_group {
					installed_addon := installed_addon
					nfo, _ := pick_nfo(installed_addon.NFOList)
					if nfo.Primary {
						if primary != nil {
							slog.Debug("multiple NFO files in addon group are set as the primary. last one wins.")
							// TODO: ensure this isn't propagated
						}
						primary = &installed_addon
					}
				}

				if primary == nil {
					slog.Debug("no NFO files in addon group are set as the primary. first one wins.")
					primary = default_primary
				}

				primary_nfo, _ := pick_nfo(primary.NFOList)

				final_nfo = &primary_nfo
				final_primary = primary
				final_installed_addon_list = installed_addon_group
			}

			addon := MakeAddon(addons_dir, final_installed_addon_list, *final_primary, final_nfo, nil, nil)
			addon_list = append(addon_list, addon)
		}
	}

	// deterministic order.
	slices.SortStableFunc(addon_list, func(a Addon, b Addon) int {
		return cmp.Compare(a.Label, b.Label)
	})

	return addon_list, nil
}

// safely removes the given `addon-dirname` from `install-dir`.
// if the given `addon-dirname` is a mutual dependency with another addon, just remove it's entry from
// the nfo file instead of deleting the whole directory."
func _remove_addon(ia InstalledAddon, addons_dir AddonsDir, grpid string) error {
	addon_dirname := ia.Name                                          // "EveryAddon"
	final_addon_path := filepath.Join(addons_dir.Path, addon_dirname) // "/path/to/addons/dir/EveryAddon"
	if !filepath.IsAbs(final_addon_path) {
		slog.Error("final path to addon to be removed must be absolute by this point", "addons-dir", addons_dir.Path, "addon-dir", addon_dirname)
		panic("programming error")
	}

	// directory to remove is not a directory!
	// how could this happen? between reading the addon data and removing the addon
	// the addon directory was removed,
	// or replaced by a file or a symlink.
	if !core.IsDir(final_addon_path) {
		return fmt.Errorf("addon not removed, path is not a directory: %s", final_addon_path)
	}

	//  directory to remove is outside of addon directory (or exactly equal to it)!
	if !strings.HasPrefix(final_addon_path, addons_dir.Path) || final_addon_path == addons_dir.Path {
		return fmt.Errorf("addon directory is outside of the addons directory: %s", final_addon_path)
	}

	if is_mutual_dependency(ia.NFOList) {
		// other addons depend on this addon, just remove the nfo file entry

		// logic in this section can be improved here.
		// * do we really need to re-read nfo data from disk? it's safer, I guess ...
		// * can we recover from failure to read or write nfo data rather than returning an error and aborting removal/installation?
		// * atomic deletions?
		updated_nfo_data, err := rm_nfo(final_addon_path, grpid)
		if err != nil {
			return fmt.Errorf("failed to remove nfo data during removal of mutual dependency addon: %w", err)
		}
		err = write_nfo(final_addon_path, updated_nfo_data)
		if err != nil {
			return fmt.Errorf("failed to write nfo data during removal of mutual dependency addon: %w", err)
		}
		slog.Debug("removed addon as mutual dependency", "addon", final_addon_path)
	} else {
		// all good, remove addon
		err := os.RemoveAll(final_addon_path)
		if err != nil {
			return fmt.Errorf("failed to remove addon directory during uninstallation: %w", err)
		}
		if core.DirExists(final_addon_path) {
			slog.Error("addon dir still exists", "final-addon-path", final_addon_path)
			panic("programming error")
		}
		slog.Debug("removed addon directory", "addon", final_addon_path)
	}

	return nil
}

// removes the given `addon` from within the `addons_dir`.
// if addon is part of a group, all addons in group are removed.
func remove_addon(addon Addon, addons_dir AddonsDir) error {
	// if addon is being ignored, refuse to remove addon.
	// note: `group-addons` will add a top level `:ignore?` flag if any addon in a bundle is being ignored.
	// 2024-08-25: behaviour changed. this is not the place to prevent ignored addons from being removed.
	// see `core/install-addon`, `core/remove-many-addons`
	if addon.IsIgnored {
		// we don't know which addon is ignored
		slog.Warn("deleting ignored addon", "addon", addon.Label, "addons-dir", addons_dir.Path)
	}

	for _, ia := range addon.InstalledAddonGroup {
		err := _remove_addon(ia, addons_dir, addon.NFO.GroupID)
		if err != nil {
			// bail early with a partial removal?
			// or continue attempting to remove installed addons and risk more partial removals?
			// either way we're going to have a broken installation,
			// so it depends on the magnitude of the breakage.
			// for now, err on the side of a small breakage and bail early.
			return err
		}
	}

	return nil
}
