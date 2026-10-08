package strongbox

import (
	"bw/core"
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

// addon data loading, merging, wrangling.

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
	URL     string // "file:///path/to/addons-dir/EveryAddon"
	DirName string // "EveryAddon"

	// an addon may have many .toc files, keyed by file name
	TOCMap map[FileName]TOC // required, >= 1

	// the directory's `.strongbox.json` data, empty when it has none or it is invalid.
	// several addons may share a directory, so it may hold a stack of nfo data.
	NFOFile NFOFile

	// the directory holds a `.git`, `.hg` or `.svn` directory: a developer's checkout
	VersionControlled bool

	result_id string // set when shown as the child of an addon, for scoping its own children

	// --- derived fields

	Name           string                  // the directory name. todo: rename, `DirName` is the same value
	Description    string                  // the .toc notes when there is a single .toc file
	GametrackIDSet mapset.Set[GameTrackID] // the superset of game tracks in each .toc file
}

var _ core.ItemInfo = (*InstalledAddon)(nil)

func NewInstalledAddon() InstalledAddon {
	return InstalledAddon{
		TOCMap:         map[FileName]TOC{},
		GametrackIDSet: mapset.NewSet[GameTrackID](),
	}
}

// returns an `InstalledAddon` for the directory `dir_name` with its game track set
// derived from `toc_map`.
// the .toc data is preferred over the nfo data for display: the nfo data may be missing,
// and is not written with a UI in mind.
// with several .toc files the correct one is unknown without a game track, so the
// description is left empty.
func MakeInstalledAddon(url string, dir_name string, toc_map map[FileName]TOC, nfo_file NFOFile, version_controlled bool) InstalledAddon {
	ia := NewInstalledAddon()
	ia.URL = url
	ia.DirName = dir_name
	ia.Name = dir_name
	ia.TOCMap = toc_map
	ia.NFOFile = nfo_file
	ia.VersionControlled = version_controlled
	for _, toc := range toc_map {
		ia.GametrackIDSet = ia.GametrackIDSet.Union(toc.GameTrackIDSet)
	}
	if len(toc_map) == 1 {
		for _, toc := range toc_map {
			ia.Description = toc.Notes
		}
	}
	return ia
}

func (ia *InstalledAddon) IsEmpty() bool {
	return len(ia.TOCMap) == 0
}

// returns `true` when the directory would be ignored even without being told to: it is a
// version controlled checkout, or a .toc file has an unrendered version.
func (ia InstalledAddon) ImplicitlyIgnored() bool {
	if ia.VersionControlled {
		return true
	}
	for _, toc := range ia.TOCMap {
		if toc.Ignored {
			return true
		}
	}
	return false
}

// returns `true` when the directory is ignored: its explicit flag when it has one,
// otherwise whether it is implicitly ignored.
func (ia InstalledAddon) IsIgnored() bool {
	if flag := ia.NFOFile.Ignored(); flag != nil {
		return *flag
	}
	return ia.ImplicitlyIgnored()
}

// returns the .toc file names in a stable order.
func (ia InstalledAddon) toc_file_names() []FileName {
	name_list := slices.Collect(maps.Keys(ia.TOCMap))
	slices.Sort(name_list)
	return name_list
}

// returns the addon's first .toc file by file name, or an error when it has none.
// use it only for data common to every .toc file.
func (ia InstalledAddon) SomeTOC() (TOC, error) {
	name_list := ia.toc_file_names()
	if len(name_list) == 0 {
		return TOC{}, errors.New("InstalledAddon has an empty tocmap")
	}
	return ia.TOCMap[name_list[0]], nil
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
		core.ITEM_FIELD_NAME: ia.DirName,
		core.ITEM_FIELD_DESC: ia.Description,
		core.ITEM_FIELD_URL:  ia.URL,
	}
	return row
}

// the natural children for an InstalledAddon,
// from the pov of an addon manager,
// are .toc files.
// each child's ID is scoped to this directory's, so reloading gives the same IDs.
func (ia InstalledAddon) ItemChildren(_ *core.App) []core.Result {
	toc_result_list := []core.Result{}
	for _, name := range ia.toc_file_names() {
		toc := ia.TOCMap[name]
		toc_result_list = append(toc_result_list, core.MakeResult(NS_TOC, toc, ia.result_id+"/"+name))
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
	TOC          *TOC           // optional, Addon.Primary.TOC[$gametrack], nil when no .toc supports the addons dir's game track
	SourceUpdate *SourceUpdate  // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
	Ignored      *bool          // the primary's explicit ignore flag, nil when neither ignored nor un-ignored. preserved when nfo data is derived
	IsIgnored    bool           // any member of the group is ignored, explicitly or implicitly
	IsPinned     bool           // Addon.Primary.NFO[-1].PinnedVersion
	IsGrouped    bool           // the addon spans several directories

	// --- formerly only accessible for Addon.Attr.
	// for now these values are just the stringified versions of the original values. may change!

	ID               string
	Source           Source
	SourceID         string
	DirName          string // "EveryAddon" in "/path/to/addons/dir/EveryAddon"
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
	SourceMapList    []SourceMap // every source the addon is known by, its current source first
}

var _ core.ItemInfo = (*Addon)(nil)

// returns the .toc in `toc_map` that supports `game_track_id`, or nil when none does.
// among several, the .toc supporting the fewest game tracks wins, then the lowest path.
// a suffixed .toc such as `EveryAddon_Cata.toc` exists to serve one game track, so it is
// preferred over a general .toc that lists many. The path breaks ties so the result never
// depends on map order.
func best_toc(toc_map map[PathToFile]TOC, game_track_id GameTrackID) *TOC {
	var best *TOC
	var best_path PathToFile
	for path, toc := range toc_map {
		if !toc.GameTrackIDSet.Contains(game_track_id) {
			continue
		}
		if best != nil {
			n, best_n := toc.GameTrackIDSet.Cardinality(), best.GameTrackIDSet.Cardinality()
			if n > best_n || (n == best_n && path > best_path) {
				continue
			}
		}
		best, best_path = &toc, path
	}
	return best
}

// returns the .toc data to use for the given `game_track_id`, or nil when none matches.
// when `strict` is false, the game track preference map is consulted and a .toc file for
// a nearby game track is accepted, the most preferred game track first.
func _make_addon__find_toc(game_track_id GameTrackID, primary_addon InstalledAddon, strict bool) *TOC {
	if strict {
		return best_toc(primary_addon.TOCMap, game_track_id)
	}
	for _, gt := range GAMETRACK_PREF_MAP[game_track_id] {
		toc := best_toc(primary_addon.TOCMap, gt)
		if toc != nil {
			return toc
		}
	}
	return nil
}

// returns the updates in `source_update_list` supporting the game track to use for
// `game_track_id`: that game track when `strict`, otherwise the first game track in its
// preference order that any update supports. the order of `source_update_list` is kept.
func _make_addon__filter_source_updates(source_update_list []SourceUpdate, game_track_id GameTrackID, strict bool) []SourceUpdate {
	pref_list := []GameTrackID{game_track_id}
	if !strict {
		pref_list = GAMETRACK_PREF_MAP[game_track_id]
	}
	for _, gt := range pref_list {
		matching := []SourceUpdate{}
		for _, su := range source_update_list {
			if su.GameTrackIDSet != nil && su.GameTrackIDSet.Contains(gt) {
				matching = append(matching, su)
			}
		}
		if len(matching) > 0 {
			return matching
		}
	}
	return []SourceUpdate{}
}

// returns the best update in `source_update_list` for the given `game_track_id`,
// or nil when none matches.
// when `strict` is false, the game track preference map is consulted and an update for a
// nearby game track is accepted.
// when `pinned_version` is set and a qualifying update has that version, it is chosen
// over newer ones.
// assumes `source_update_list` is sorted newest to oldest.
// clj: `catalogue.clj/expand-summary`
func _make_addon__pick_source_update(source_update_list []SourceUpdate, game_track_id GameTrackID, strict bool, pinned_version string) *SourceUpdate {
	matching := _make_addon__filter_source_updates(source_update_list, game_track_id, strict)
	if len(matching) == 0 {
		return nil
	}
	if pinned_version != "" {
		for _, su := range matching {
			if su.Version == pinned_version {
				return &su
			}
		}
	}
	return &matching[0]
}

// returns an `Addon`: a flattened view of an installed addon group, its catalogue match
// and the updates available for it.
// most fields are derived here, preferring catalogue data, then nfo data, then .toc data.
// the .toc file and source update chosen depend on the game track and strictness of the
// given `addons_dir`.
// panics if `addons_dir` is empty, or if a primary addon is given without a group.
// the complex parts are farmed out to testable functions rather than done inline.
func MakeAddon(addons_dir AddonsDir, installed_addon_list []InstalledAddon, primary_addon InstalledAddon, nfo *NFO, catalogue_addon *CatalogueAddon, source_update_list []SourceUpdate) Addon {
	a := Addon{
		InstalledAddonGroup: installed_addon_list,
		Primary:             primary_addon,
		CatalogueAddon:      catalogue_addon,
		SourceUpdateList:    source_update_list,
		AddonsDir:           &addons_dir,

		// --- fields we can derive immediately

		NFO: nfo,
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
	// the explicit flag captures three states: ignored (true), un-ignored (false) and
	// neither (nil). it is kept so nfo data derived from this addon preserves it.
	if has_nfo {
		a.Ignored = nfo.Ignored
	}
	// one ignored member ignores the whole group: touching any of it would touch the
	// ignored directory.
	for _, ia := range installed_addon_list {
		a.IsIgnored = a.IsIgnored || ia.IsIgnored()
	}
	if len(installed_addon_list) == 0 && has_nfo && nfo.Ignored != nil {
		// an addon not installed yet, carrying a flag to write
		a.IsIgnored = *nfo.Ignored
	}

	// 'pinned'
	if has_nfo {
		a.IsPinned = nfo_pinned(*nfo)
		a.PinnedVersion = nfo.PinnedVersion
	}

	a.IsGrouped = len(installed_addon_list) > 1

	// pick a `SourceUpdate` from a list of updates.
	if has_updates_available && has_game_track {
		// choose a specific update from a list of updates.
		// assumes `source_update_list` is sorted newest to oldest.
		a.SourceUpdate = _make_addon__pick_source_update(source_update_list, a.AddonsDir.GameTrackID, a.AddonsDir.Strict, a.PinnedVersion)
	}

	has_update := a.SourceUpdate != nil

	// the primary's .toc, for details common to every .toc file when none suits the game track
	some_toc, some_toc_err := primary_addon.SomeTOC()
	has_some_toc := some_toc_err == nil

	// human friendly addon title
	switch {
	case has_match:
		a.Label = a.CatalogueAddon.Label // "AdiBags"
	case a.IsGrouped && has_nfo && !nfo.Primary:
		// no directory claims to be the primary, the group is named for its group ID
		a.Label = nfo.GroupID + " (group)"
	case has_toc:
		a.Label = a.TOC.Label
	case has_some_toc:
		a.Label = some_toc.Label
	default:
		a.Label = primary_addon.DirName
	}
	a.Label = RemoveEscapeSequences(a.Label)

	a.DirName = primary_addon.DirName

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
	switch {
	case has_nfo && a.NFO.InstalledVersion != "":
		a.InstalledVersion = a.NFO.InstalledVersion
	case has_toc:
		a.InstalledVersion = a.TOC.InstalledVersion
	case has_some_toc:
		a.InstalledVersion = some_toc.InstalledVersion
	}

	// case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
	if has_update {
		a.AvailableVersion = a.SourceUpdate.Version
	} else if has_toc {
		// new in 8.0 - if no version available, don't show an available version!
		//a.AvailableVersion = a.TOC.InstalledVersion
	}

	// case "source", "source-id":
	// where the addon was installed from wins over where the catalogue says it lives: the
	// user may have switched source, and updates must come from where it was installed.
	// a .toc's sources are only *potential* sources, not the actual one.
	switch {
	case has_nfo && a.NFO.Source != "" && a.NFO.SourceID != "":
		a.Source = a.NFO.Source
		a.SourceID = string(a.NFO.SourceID)
	case has_match:
		a.Source = a.CatalogueAddon.Source
		a.SourceID = string(a.CatalogueAddon.SourceID)
	}

	// every source the addon is known by: the nfo's, then the .toc files'.
	// curseforge and tukui are gone, switching to them is pointless.
	sml := []SourceMap{}
	add_source := func(sm SourceMap) {
		if sm.Source != "" && sm.SourceID != "" && SUPPORTED_HOSTS.Contains(sm.Source) && !slices.Contains(sml, sm) {
			sml = append(sml, sm)
		}
	}
	add_source(SourceMap{Source: a.Source, SourceID: FlexString(a.SourceID)})
	if has_nfo {
		for _, sm := range nfo.SourceMapList {
			add_source(sm)
		}
	}
	for _, name := range primary_addon.toc_file_names() {
		for _, sm := range primary_addon.TOCMap[name].SourceMapList {
			add_source(sm)
		}
	}
	if len(sml) > 0 {
		a.SourceMapList = sml
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

// returns a friendly unique ID for a zipfile based on the file name.
// the zipfile need not exist.
// clj: `cli.clj/unique-group-id-from-zip-file`
func unique_group_id_from_zip_file(zipfile string) string {
	basename := filepath.Base(zipfile)        // "/foo/bar/baz--1-2-3.zip" => "baz--1-2-3.zip"
	ext := filepath.Ext(basename)             // "baz--1-2-3.zip" => ".zip"
	name := strings.TrimSuffix(basename, ext) // "baz--1-2-3.zip" => "baz--1-2-3"
	// random zip files are unlikely to be double-hyphenated,
	// this is something strongbox does for easier tokenisation,
	// but if a strongbox-downloaded .zip is being used, this will strip some noise.
	first_bit, _, _ := strings.Cut(name, "--") // "baz--1-2-3" => "baz"
	suffix := make([]byte, 4)
	rand.Read(suffix)
	return fmt.Sprintf("%s-%s", strings.ToLower(first_bit), hex.EncodeToString(suffix)) // "baz-928e42d2"
}

// returns an `Addon` for a .zip file that has not been installed yet.
// the addon has no installed addons, no catalogue match and no updates: only a group ID
// derived from the file name.
// returns an error when `zipfile` does not exist.
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

// returns an `Addon` for a catalogue addon that has not been installed yet.
// the addon has no installed addons, and its group ID is the catalogue addon's URL.
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
		"version",
		"combined-version",
		"source-id",
		"game-version",
	}
}

// returns the version to display for `a`: the available version when there is an update,
// otherwise the installed version, prefixed with why it won't change.
// clj: `cli.clj/combined-version`
func combined_version(a Addon) string {
	version := a.InstalledVersion
	if Updateable(a) {
		version = a.AvailableVersion
	}
	switch {
	case a.IsIgnored:
		return strings.TrimSpace("(ignored) " + version)
	case a.IsPinned:
		return strings.TrimSpace("(pinned) " + version)
	}
	return version
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

	version := a.InstalledVersion
	if a.AvailableVersion != "" {
		version = a.AvailableVersion
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
		"version":                    version,
		"combined-version":           combined_version(a),
		"source-id":                  a.SourceID,
		"game-version":               a.GameVersion,
	}
}

// an Addon may be grouping multiple InstalledAddons.
// if so, they can be loaded immediately.
func (a Addon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

// the directories making up the addon, each with an ID scoped to the addon: several
// addons may share a directory.
func (a Addon) ItemChildren(_ *core.App) []core.Result {
	if a.AddonsDir == nil {
		return []core.Result{}
	}
	parent_id := addon_result_id(a.AddonsDir.Path, a)
	children := []core.Result{}
	for _, ia := range a.InstalledAddonGroup {
		ia.result_id = parent_id + "/" + ia.DirName
		children = append(children, core.MakeResult(NS_INSTALLED_ADDON, ia, ia.result_id))
	}
	return children
}

// returns `true` when the given addon `a` can be updated to a newer version.
// an addon with updates available may still not be updateable: an ignored addon never
// is, and a pinned addon only is when the pinned version is both uninstalled and
// available.
// when the installed and available versions are equal, the addon is still updateable if
// neither its .toc data nor its nfo data supports any game track the update offers.
// clj: `addon/updateable?`
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

	// a pinned addon can only be updated to its pinned version, and only when that is not
	// already installed.
	if a.IsPinned {
		return a.PinnedVersion != a.InstalledVersion && a.PinnedVersion == a.AvailableVersion
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

// reads the .toc and nfo data in the given `addon_dir` as a single `InstalledAddon`.
// loads everything it can about the addon regardless of game track, strictness, pinned
// status or ignore status: filtering is the caller's job.
// an nfo file that cannot be read is logged at WARN and treated as absent, it is never
// deleted.
// returns an error when the .toc files cannot be read.
// clj: `addon.clj/-load-installed-addon`
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		return InstalledAddon{}, fmt.Errorf("failed to load addon: %w", err)
	}
	nfo_file, err := read_nfo_file(addon_dir)
	if err != nil && !errors.Is(err, ErrNFODNE) {
		slog.Warn("ignoring nfo data that cannot be read", "error", err)
		nfo_file = NFOFile{}
	}
	url := "file://" + addon_dir
	return MakeInstalledAddon(url, filepath.Base(addon_dir), toc_map, nfo_file, version_controlled(addon_dir)), nil
}

// returns the 'main' directory out of `toplevel_dirs`, or an error when it cannot be
// determined.
//
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
//  4. if case doesn't hold, do nothing and accept we have no 'main' addon
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

// returns the group ID of `ia`, or "" when no addon owns its directory.
func installed_addon_group_id(ia InstalledAddon) string {
	top, ok := ia.NFOFile.Top()
	if !ok {
		return ""
	}
	return top.GroupID
}

// returns the member of `group` that represents it: the one marked primary, or the one
// with the lowest directory name when none or several are, so the choice never depends on
// the order directories were read.
func pick_primary(group []InstalledAddon) InstalledAddon {
	sorted := slices.Clone(group)
	slices.SortFunc(sorted, func(a, b InstalledAddon) int { return cmp.Compare(a.DirName, b.DirName) })
	for _, ia := range sorted {
		if top, _ := ia.NFOFile.Top(); top.Primary {
			return ia
		}
	}
	return sorted[0]
}

// returns the installed addons in `installed_addon_list` as `Addon`s: one per group ID,
// and one per directory no addon owns.
// a map from group ID to members, so related directories are found in one pass.
// clj: `addon.clj/group-addons`
func group_installed_addons(addons_dir AddonsDir, installed_addon_list []InstalledAddon) []Addon {
	addon_list := []Addon{}
	grouped := map[string][]InstalledAddon{}
	for _, ia := range installed_addon_list {
		group_id := installed_addon_group_id(ia)
		if group_id == "" {
			// no nfo data, or only an ignore flag: an addon of its own
			var nfo *NFO
			addon_list = append(addon_list, MakeAddon(addons_dir, []InstalledAddon{ia}, ia, nfo, nil, nil))
			continue
		}
		grouped[group_id] = append(grouped[group_id], ia)
	}

	for _, group := range grouped {
		slices.SortFunc(group, func(a, b InstalledAddon) int { return cmp.Compare(a.DirName, b.DirName) })
		primary := pick_primary(group)
		primary_nfo, _ := primary.NFOFile.Top()
		addon_list = append(addon_list, MakeAddon(addons_dir, group, primary, &primary_nfo, nil, nil))
	}

	sort_addons(addon_list)
	return addon_list
}

// sorts `addon_list` in place by case-insensitive label, then directory name.
func sort_addons(addon_list []Addon) {
	slices.SortStableFunc(addon_list, func(a Addon, b Addon) int {
		if c := cmp.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)); c != 0 {
			return c
		}
		return cmp.Compare(a.DirName, b.DirName)
	})
}

// reads the .toc and nfo data of every addon in the given `addons_dir`, groups them by
// nfo group ID and returns one `Addon` per group, sorted by label.
// addons that fail to load are logged and skipped, they do not fail the whole read.
// Blizzard's own addons are skipped.
// clj: `addon.clj/load-all-installed-addons`, `toc.clj/parse-addon-toc-guard`
func LoadAllInstalledAddons(addons_dir AddonsDir) ([]Addon, error) {
	dir_list, err := core.DirList(addons_dir.Path)
	if err != nil {
		return []Addon{}, err
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

	return group_installed_addons(addons_dir, installed_addon_list), nil
}

// returns an error when `dir_name` is not a directory strictly inside `addons_dir`: a
// symbolic link, a path that escapes it, or the addons dir itself.
// a deletion is not recoverable, so anything that looks wrong is refused.
func check_removable(addons_dir AddonsDir, dir_name string) (string, error) {
	final_addon_path := filepath.Join(addons_dir.Path, dir_name) // "/path/to/addons/dir/EveryAddon"
	rel, err := filepath.Rel(addons_dir.Path, final_addon_path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(dir_name) || strings.Contains(rel, string(filepath.Separator)) {
		return "", fmt.Errorf("directory is outside the addons directory: %s", final_addon_path)
	}
	stat, err := os.Lstat(final_addon_path)
	if err != nil || !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		// between reading the addon and removing it the directory was removed, or replaced
		// by a file or a symlink.
		return "", fmt.Errorf("addon not removed, path is not a directory: %s", final_addon_path)
	}
	return final_addon_path, nil
}

// removes the directory of the given installed addon `ia` from `addons_dir`.
// when the directory is shared with another addon, only the nfo entry for `group_id` is
// removed and the directory is left in place.
// refuses to remove, returning an error, anything `check_removable` refuses.
func _remove_addon(ia InstalledAddon, addons_dir AddonsDir, group_id string) error {
	final_addon_path, err := check_removable(addons_dir, ia.DirName)
	if err != nil {
		return err
	}

	// re-read rather than trust the loaded data: another addon may have claimed the
	// directory since.
	nfo_file, err := read_nfo_file(final_addon_path)
	if err != nil && !errors.Is(err, ErrNFODNE) {
		return fmt.Errorf("failed to read nfo data during removal: %w", err)
	}

	if group_id != "" && nfo_file.IsMutualDependency() {
		// other addons use this directory, just remove this addon's nfo entry
		err = write_nfo_file(final_addon_path, nfo_file_rm(nfo_file, group_id))
		if err != nil {
			return fmt.Errorf("failed to write nfo data during removal of mutual dependency addon: %w", err)
		}
		slog.Debug("removed addon as mutual dependency", "addon", final_addon_path)
		return nil
	}

	err = os.RemoveAll(final_addon_path)
	if err != nil {
		return fmt.Errorf("failed to remove addon directory during uninstallation: %w", err)
	}
	slog.Debug("removed addon directory", "addon", final_addon_path)
	return nil
}

// removes the given `addon` from within the `addons_dir`.
// every installed addon in the group is removed.
// stops at the first failure, which may leave the group partly removed: a small
// breakage is preferred to continuing and risking a larger one.
// does not refuse to remove an ignored addon, callers check that first.
func remove_addon(addon Addon, addons_dir AddonsDir) error {
	if addon.IsIgnored {
		slog.Warn("deleting ignored addon", "addon", addon.Label, "addons-dir", addons_dir.Path)
	}

	group_id := ""
	if addon.NFO != nil {
		group_id = addon.NFO.GroupID
	}

	for i, ia := range addon.InstalledAddonGroup {
		err := _remove_addon(ia, addons_dir, group_id)
		if err != nil {
			if i > 0 {
				return fmt.Errorf("addon partly removed: %w", err)
			}
			return err
		}
	}

	// another addon may have taken over one of this addon's directories, which then
	// belongs to that addon's group but still lists this one beneath it
	if group_id != "" {
		return forget_group(addons_dir, group_id)
	}
	return nil
}

// removes the entries for `group_id` from the nfo data of every directory in `addons_dir`.
func forget_group(addons_dir AddonsDir, group_id string) error {
	dir_list, err := core.DirList(addons_dir.Path)
	if err != nil {
		return err
	}
	for _, dir := range dir_list {
		f, err := read_nfo_file(dir)
		if err != nil {
			continue
		}
		if !slices.ContainsFunc(f.Stack, func(n NFO) bool { return n.GroupID == group_id }) {
			continue
		}
		if err := write_nfo_file(dir, nfo_file_rm(f, group_id)); err != nil {
			return fmt.Errorf("failed to remove addon from shared directory: %w", err)
		}
	}
	return nil
}
