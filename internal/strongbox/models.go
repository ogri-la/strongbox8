package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"errors"
	"fmt"
)

// for converting fields that are either ints or strings to just strings.
// deprecated.
// inspiration from here:
// - https://docs.bitnami.com/tutorials/dealing-with-json-with-non-homogeneous-types-in-go
type FlexString string

func (fi *FlexString) UnmarshalJSON(b []byte) error {
	if b[0] != '"' {
		// we have an int (hopefully)
		var i int
		err := json.Unmarshal(b, &i)
		if err != nil {
			return err
		}
		*fi = FlexString(core.IntToString(i))
		return nil
	}
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*fi = FlexString(s)
	return nil
}

// --- Source Map
// used to know where an addon came from and other locations it may live.

type SourceMap struct {
	Source   Source     `json:"source"`
	SourceID FlexString `json:"source-id"`
}

// --- TOC
// subset of data parsed from .toc files

// todo: should a distinction be made between 'raw' and 'processed' values?
// for example, 'Title' and 'Notes' are raw, 'Label' is processed
type TOC struct {
	Title                       string      // the unmodified 'title' value. new in 8.0
	Label                       string      // a modified 'title' value and even a replacement in some cases
	Name                        string      // a slugified 'label'
	Notes                       string      // 'description' in v7. some addons may use 'description' the majority use 'notes'
	DirName                     string      // "AdiBags" in "/path/to/addon-dir/AdiBags/AdiBags.toc"
	FileName                    string      // "AdiBags.toc" in "/path/to/addon-dir/AdiBags/AdiBags.toc". new in 8.0.
	FileNameGameTrackID         GameTrackID // game track guessed from filename
	InterfaceVersionGameTrackID GameTrackID // game track derived from the interface version. the interface version may not be present.
	GameTrackID                 GameTrackID // game track decided upon from file name and file contents
	InterfaceVersion            int         // WoW version 101001
	InstalledVersion            string      // Addon version "v1.200-beta-alpha-extreme"
	Ignored                     bool        // indicates addon should be ignored
	SourceMapList               []SourceMap
}

func (t TOC) Attr(field string) string {
	switch field {
	case "id":
		return fmt.Sprintf("%v/%v", t.DirName, t.FileName)
	default:
		panic("programming error, TOC file has no such field: " + field)
	}

}

func (t TOC) RowHasChildren() bool {
	// a toc file doesn't have any semantically significant children.
	// I imagine if I implement an explode() function in the future then a .toc file could give rise to:
	// access and modification dates, file size integer, text blob, comments, etc
	return false
}

func (t TOC) RowChildren() []core.Result {
	return []core.Result{}
}

func (t TOC) RowKeys() []string {
	return []string{
		"name",
		"description",
		"installed",
		"WoW",
		"ignored",
	}
}

func (t TOC) RowMap() map[string]string {
	game_version, _ := InterfaceVersionToGameVersion(t.InterfaceVersion)
	return map[string]string{
		"name":        t.FileName,
		"description": t.Notes,
		"installed":   t.InstalledVersion,
		"WoW":         game_version,
		"ignored":     fmt.Sprintf("%v", t.Ignored),
	}
}

// --- NFO
// strongbox curated data about an addon or group of addons.
// created when an addon is installed through strongbox.
// derived from toc, catalogue, per-addon user preferences, etc.
// lives in .strongbox.json files in the addon's root.

// we *could* create these upon first detecting an addon so that nfo data is *always* available,
// but first time users would be left with .strongbox files hanging around.
// a solution might be to not store these per-directory and instead keep a central database.
// should that happen we still may not have enough data to create a valid nfo file as we need
// a catalogue match.

type NFO struct {
	InstalledVersion     string      `json:"installed-version"`
	Name                 string      `json:"name"`
	GroupID              string      `json:"group-id"`
	Primary              bool        `json:"primary?"`
	Source               Source      `json:"source"`
	InstalledGameTrackID GameTrackID `json:"installed-game-track"`
	SourceID             FlexString  `json:"source-id"` // ints become strings, new in v8
	SourceMapList        []SourceMap `json:"source-map-list"`
	Ignored              *bool       `json:"ignore?"` // null means the user hasn't explicitly ignored or explicitly un-ignored it
	PinnedVersion        string      `json:"pinned-version"`
}

// --- Catalogue Addon
// data parsed from strongbox catalogues

// previously 'summary' or 'addon summary'
type CatalogueAddon struct {
	URL             string        `json:"url"`
	Name            string        `json:"name"`
	Label           string        `json:"label"`
	Description     string        `json:"description"`
	TagList         []string      `json:"tag-list"`
	UpdatedDate     string        `json:"updated-date"`
	DownloadCount   int           `json:"download-count"`
	Source          Source        `json:"source"`
	SourceID        FlexString    `json:"source-id"`
	GameTrackIDList []GameTrackID `json:"game-track-list"`
}

// --- InstalledAddon
// the collection of .toc data and .strongbox.json data for an addon directory.

type InstalledAddon struct {
	// an addon may have many .toc files, keyed by game track.
	// the toc data eventually used is determined by the selected addon dir's game track.
	TOCMap map[GameTrackID]TOC // required, >= 1

	// an installed addon has zero or one `strongbox.json` 'nfo' files,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	// new in v8, all nfo data is now a list
	NFOList []NFO // optional
}

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

// priority: parent, toc
func (a InstalledAddon) Attr(field string) string {
	if len(a.TOCMap) < 1 {
		panic("programming error. empty `InstalledAddon` accessed `.Attr`")
	}

	some_toc := TOC{}
	for _, val := range a.TOCMap {
		some_toc = val
		break
	}

	//has_nfo := len(a.NFOList) > 0

	switch field {
	case "id":
		return some_toc.DirName // "AdiBags_Config"

	case "name":
		return some_toc.DirName

	}
	panic(fmt.Sprintf("programming error, unknown field: %s", field))
}

func (ia InstalledAddon) ItemHasChildren() bool {
	return true // an installed addon has 1+ .toc files
}

func (ia InstalledAddon) ItemKeys() []string {
	return []string{
		"browse",
		"source",
		"name",
	}
}

func (ia InstalledAddon) ItemMap() map[string]string {
	row := map[string]string{
		"browse": "browse", // a.AnyTOC().DirName,
		"source": "",
		"name":   ia.Attr("name"),
	}
	return row
}

func (ia InstalledAddon) ItemChildren() []core.Result {
	// todo: what would be natural children for an installed addon? the .toc files? the file listing?
	foo := []core.Result{}
	for _, toc := range ia.TOCMap {
		foo = append(foo, core.Result{ID: toc.Attr("id"), NS: NS_TOC, Item: toc})
	}
	return foo
}

// --- Source Updates
// extra data a source (wowinterface, github, etc) provides about an addon.

// todo: rename 'release' or similar? release.type: 'lib', 'nolib'. release.stability: 'stable', 'beta', 'alpha', etc.
type SourceUpdate struct {
	//Type string // lib, nolib
	//Stability string // beta, alpha, etc
	Version          string `json:"version"`
	DownloadURL      string
	GameTrackID      GameTrackID
	InterfaceVersion int
}

// --- Addon

// an 'addon' represents one or a group of installed addons.
// the group has a representative 'primary' addon,
// representative TOC data according to the selected game track of the addon dir the addon lives in,
// representative NFO data according to whether the addon is overriding or being overridden by other addons.
type Addon struct {
	AddonGroup []InstalledAddon // required >= 1
	Primary    *InstalledAddon  // required

	TOC            *TOC            // required, Addon.Primary.TOC[$gametrack]
	NFO            *NFO            // optional, Addon.Primary.NFO[-1]
	CatalogueAddon *CatalogueAddon // optional, the catalogue match

	Ignored bool // required, `Addon.Primary.NFO[-1].Ignored` or `Addon.Primary.TOC[$gametrack].Ignored`

	SourceUpdateList []SourceUpdate
	SourceUpdate     *SourceUpdate // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
}

func (a Addon) ItemKeys() []string {
	return []string{
		"browse",
		"source",
		"name",
		"description",
		"tags",
		"created",
		"updated",
		"size",
		"installed",
		"available",
		"WoW",
	}
}

func (a Addon) ItemMap() map[string]string {
	return map[string]string{
		"browse":      "[link]",
		"source":      a.Attr("source"),
		"name":        a.Attr("dirname"),
		"description": a.Attr("description"),
		"tags":        "foo,bar,baz",
		"created":     "[todo]",
		"updated":     a.Attr("updated"),
		"size":        "0",
		"installed":   a.Attr("installed-version"),
		"available":   a.Attr("available-version"),
		"WoW":         a.Attr("game-version"),
	}
}

func (a Addon) ItemHasChildren() bool {
	return len(a.AddonGroup) > 1
}

func (a Addon) ItemChildren() []core.Result {
	children := []core.Result{}
	if len(a.AddonGroup) > 1 {
		for _, installed_addon := range a.AddonGroup {
			children = append(children, core.NewResult(NS_INSTALLED_ADDON, installed_addon, installed_addon.Attr("id")))
		}
	}
	return children
}

// attribute picked for an addon.
// order of precedence (typically) is: source_updates (tbd), catalogue_addon, nfo, toc
func (a Addon) Attr(field string) string {
	// has_toc := a.TOC != nil // always true, must be strict.
	has_nfo := a.NFO != nil //
	has_match := a.CatalogueAddon != nil
	has_updates := false
	//has_parent := false // is this an addon within a group?
	switch field {

	// unique-ish identifier for this addon
	case "id":
		if has_nfo {
			return fmt.Sprintf("%s/%s", a.NFO.Source, a.NFO.SourceID) // "github/AdiBags/AdiBags"
		}
		return fmt.Sprintf("%s/%s", a.TOC.DirName, a.TOC.FileName) // "AdiBags_Config/AdiBags_Config.toc"

	// raw title. does anything use this?
	case "title":
		if has_match {
			return a.CatalogueAddon.Label // "AdiBags"
		}
		return a.TOC.Title // may be empty, use "label" for a safer "title"

	// human friendly addon title
	case "label":
		if has_match {
			return a.CatalogueAddon.Label // "AdiBags"
		}
		return a.TOC.Label // "AdiBags", "Group Title *"

	// normalised title
	case "name":
		if has_match {
			return a.CatalogueAddon.Name // "adibags"
		}
		if has_nfo {
			return a.NFO.Name
		}

		return a.TOC.Name

	case "description":
		if has_match {
			return a.CatalogueAddon.Description
		}
		return a.TOC.Notes

	case "dirname":
		return a.TOC.DirName

	case "interface-version": // 100105, 30402
		return core.IntToString(a.TOC.InterfaceVersion)

	case "game-version": // "10.1.5", "3.4.2"
		v, err := InterfaceVersionToGameVersion(a.TOC.InterfaceVersion)
		if err == nil {
			return v
		}

	case "installed-version": // v1.2.3, foo-bar.zip.v1, 10.12.0v1.4.2, 12312312312
		if has_nfo {
			return a.NFO.InstalledVersion
		}
		return a.TOC.InstalledVersion

	case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
		if has_updates {
			return a.SourceUpdate.Version
		}
		return a.TOC.InstalledVersion

	case "source":
		if has_match {
			return a.CatalogueAddon.Source
		}
		if has_nfo {
			return a.NFO.Source
		}

		// can we do anything with a.TOC.SourceMapList if one exists?
		// I guess those are just *potential* sources rather than the *actual* source.

	case "source-id":
		if has_match {
			return string(a.CatalogueAddon.SourceID)
		}
		if has_nfo {
			return string(a.NFO.SourceID)
		}

	case "updated":
		/* todo: use the latest update value we can find?
		if has_updates {
			// ...
		}
		*/

		if has_match {
			return a.CatalogueAddon.UpdatedDate
		}

	default:
		panic(fmt.Sprintf("programming error, unknown field: %s", field))
	}

	return ""
}

// --- Catalogue

type CatalogueSpec struct {
	Version int `json:"version"`
}

type Catalogue struct {
	Spec             CatalogueSpec    `json:"spec"`
	Datestamp        string           `json:"datestamp"`
	Total            int              `json:"total"`
	AddonSummaryList []CatalogueAddon `json:"addon-summary-list"`
}
