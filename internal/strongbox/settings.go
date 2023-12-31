package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
)

const (
	ID_PREFERENCES    = "strongbox preferences"
	ID_CATALOGUE      = "strongbox catalogue"
	ID_USER_CATALOGUE = "strongbox user catalogue"
)

var (
	NS_CATALOGUE_LOC   = core.NS{Major: "strongbox", Minor: "catalogue", Type: "location"}
	NS_CATALOGUE       = core.NS{Major: "strongbox", Minor: "catalogue", Type: "catalogue"}
	NS_CATALOGUE_USER  = core.NS{Major: "strongbox", Minor: "catalogue", Type: "user"}
	NS_ADDON_DIR       = core.NS{Major: "strongbox", Minor: "addon-dir", Type: "dir"}
	NS_ADDON           = core.NS{Major: "strongbox", Minor: "addon", Type: "addon"}
	NS_INSTALLED_ADDON = core.NS{Major: "strongbox", Minor: "addon", Type: "installed-addon"}
	NS_PREFS           = core.NS{Major: "strongbox", Minor: "settings", Type: "preference"}
)

const DEFAULT_INTERFACE_VERSION = 100000

const NFO_FILENAME = ".strongbox.json"

type PathToFile = string        // "/path/to/some/file.ext"
type PathToDir = string         // "/path/to/some/dir/"
type PathToAddon = string       // "/path/to/addon-dir/Addon/"
type PathToDirOfAddons = string // "/path/to/addon-dir/"

type GameTrackID = string

const (
	GAMETRACK_RETAIL        GameTrackID = "retail"
	GAMETRACK_CLASSIC                   = "classic"
	GAMETRACK_CLASSIC_TBC               = "classic-tbc"
	GAMETRACK_CLASSIC_WOTLK             = "classic-wotlk"
)

var GT_PREF_MAP map[GameTrackID][]GameTrackID = map[GameTrackID][]GameTrackID{
	GAMETRACK_RETAIL:        []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK},
	GAMETRACK_CLASSIC:       []GameTrackID{GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_TBC:   []GameTrackID{GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_WOTLK: []GameTrackID{GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
}

type GameTrack struct {
	ID    GameTrackID
	Label string
	// description ?
	// original release date ?
	// classic release date ?
}

var (
	GT_RETAIL        = GameTrack{ID: GAMETRACK_RETAIL, Label: "Retail"}
	GT_CLASSIC       = GameTrack{GAMETRACK_CLASSIC, "Classic"}
	GT_CLASSIC_TBC   = GameTrack{GAMETRACK_CLASSIC_TBC, "Classic (TBC)"}
	GT_CLASSIC_WOTLK = GameTrack{GAMETRACK_CLASSIC_WOTLK, "Classic (WotLK)"}
)

type Source = string

const (
	SOURCE_GITHUB Source = "github"
	SOURCE_GITLAB        = "gitlab"
	SOURCE_WOWI          = "wowinterface"

	// dead
	SOURCE_CURSEFORGE          = "curseforge"
	SOURCE_TUKUI               = "tukui"
	SOURCE_TUKUI_CLASSIC       = "tukui-classic"
	SOURCE_TUKUI_CLASSIC_TBC   = "tukui-classic-tbc"
	SOURCE_TUKUI_CLASSIC_WOTLK = "tukui-classic-wotlk"
)

var DISABLED_HOSTS = map[Source]bool{
	SOURCE_CURSEFORGE:          true,
	SOURCE_TUKUI:               true,
	SOURCE_TUKUI_CLASSIC:       true,
	SOURCE_TUKUI_CLASSIC_TBC:   true,
	SOURCE_TUKUI_CLASSIC_WOTLK: true,
}

type SourceMap struct {
	Source   Source     `json:"source"`
	SourceID FlexString `json:"source-id"`
}

type TOC struct {
	Title                       string      // the unmodified 'title' value. new in 8.0
	Label                       string      // a modified 'title' value and even a replacement in some cases
	Name                        string      // a slugified 'label'
	Notes                       string      // 'description' in v7. some addons may use 'description' the majority use 'notes'
	DirName                     string      // "AdiBags" in "/path/to/addon-dir/AdiBags/AdiBags.toc"
	FileName                    string      // "AdiBags.toc" in "/path/to/addon-dir/AdiBags/AdiBags.toc". new in 8.0.
	FileNameGameTrackID         GameTrackID // game track guessed from filename
	InterfaceVersionGameTrackID GameTrackID // game track derived from the interface version. the interface version may not be present.
	GameTrackID                 GameTrackID // game track decided upon
	InterfaceVersion            int
	InstalledVersion            string
	Ignored                     bool // indicates addon should be ignored
	SourceMapList               []SourceMap
}

// for converting fields that are ints or strings to just strings.
// remove in v10.
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

// raw data of an installed addon
type InstalledAddon struct {
	// an addon may have many .toc files, keyed by game track.
	// the toc data eventually used is determined by the selected addon dir's game track.
	TOCMap map[GameTrackID]TOC

	// an installed addon has zero or one `strongbox.json` 'nfo' files,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	NFOList []NFO // all nfo data is now a list, new in v8
}

// todo: rename 'release' or similar? release.type: 'lib', 'nolib'. release.stability: 'stable', 'beta', 'alpha', etc.
type SourceUpdate struct {
	//Type string // lib, nolib
	//Stability string // beta, alpha, etc
	Version          string `json:"version"`
	DownloadURL      string
	GameTrackID      GameTrackID
	InterfaceVersion int
}

// an 'addon' represents one or many installed addons.
// the group has a representative 'primary' addon,
// representative TOC data according to the selected game track of the addon dir the addon lives in,
// representative NFO data according to whether the addon is overriding or being overridden by other addons.
type Addon struct {
	AddonGroup []InstalledAddon
	Primary    *InstalledAddon

	TOC            *TOC            // Addon.Primary.TOC[$gametrack]
	NFO            *NFO            // Addon.Primary.NFO[-1]
	CatalogueAddon *CatalogueAddon // the catalogue match, if any

	Ignored bool // Addon.Primary.NFO[-1].Ignored or Addon.Primary.TOC[$gametrack].Ignored

	SourceUpdateList []SourceUpdate
	SourceUpdate     *SourceUpdate // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
}

func (a Addon) RowKeys() []string {
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

func (a Addon) RowMap() map[string]string {
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

// wraps each of the grouped addons in an `Addon` and then in a `core.Result`.
func (a Addon) RowChildren() []core.Result {
	children := []core.Result{}
	if len(a.AddonGroup) > 1 {
		for _, installed_addon := range a.AddonGroup {
			installed_addon := installed_addon
			synthetic_addon := InstalledAddonToAddon(installed_addon)
			children = append(children, core.NewResult(NS_ADDON, synthetic_addon, AddonID(synthetic_addon)))
		}
	}
	return children
}

// attribute picked for an addon.
// order of precedence is: source_updates (tbd), catalogue_addon, nfo, toc
func (a Addon) Attr(field string) string {
	has_toc := a.TOC != nil
	has_nfo := a.NFO != nil
	has_match := a.CatalogueAddon != nil
	has_updates := false
	switch field {
	case "title": // "AdiBags" => "AdiBags"
		if has_match {
			return a.CatalogueAddon.Label
		}
		if has_toc {
			return a.CatalogueAddon.Label
		}

	case "label": // "AdiBags" => "AdiBags", "Group Title *"
		if has_match {
			return a.CatalogueAddon.Label
		}
		if has_toc {
			return a.TOC.Label
		}

	case "name": // "AdiBags" => "adibags"
		if has_match {
			return a.CatalogueAddon.Name
		}
		if has_nfo {
			return a.NFO.Name
		}
		if has_toc {
			return a.TOC.Name
		}

	case "description":
		if has_match {
			return a.CatalogueAddon.Description
		}
		if has_toc {
			return a.TOC.Notes
		}

	case "dirname":
		if has_toc {
			return a.TOC.DirName
		}

	case "interface-version": // 100105, 30402
		if has_toc {
			return core.IntToString(a.TOC.InterfaceVersion)
		}

	case "game-version": // "10.1.5", "3.4.2"
		if has_toc {
			v, err := InterfaceVersionToGameVersion(a.TOC.InterfaceVersion)
			if err == nil {
				return v
			}
		}

	case "installed-version": // v1.2.3, foo-bar.zip.v1, 10.12.0v1.4.2, 12312312312
		if has_nfo {
			return a.NFO.InstalledVersion
		}
		if has_toc {
			return a.TOC.InstalledVersion
		}

	case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
		if has_updates {
			return a.SourceUpdate.Version
		}
		if has_toc {
			return a.TOC.InstalledVersion
		}

	case "source":
		if has_match {
			return a.CatalogueAddon.Source
		}
		if has_nfo {
			return a.NFO.Source
		}

	case "source-id":
		if has_match {
			return string(a.CatalogueAddon.SourceID)
		}
		if has_nfo {
			return string(a.NFO.SourceID)
		}

	case "updated":
		if has_match {
			return a.CatalogueAddon.UpdatedDate
		}

	default:
		panic(fmt.Sprintf("programming error, unknown field: %s", field))
	}

	return ""

}

type CatalogueSpec struct {
	Version int `json:"version"`
}

type Catalogue struct {
	Spec             CatalogueSpec    `json:"spec"`
	Datestamp        string           `json:"datestamp"`
	Total            int              `json:"total"`
	AddonSummaryList []CatalogueAddon `json:"addon-summary-list"`
}

// a directory containing addons.
// a typical WoW installation will have multiple of these, one for retail, classic, etc.
// a user may have multiple WoW installations.
type AddonsDir struct {
	Path        string      `json:"addon-dir"`
	GameTrackID GameTrackID `json:"game-track"`
	Strict      bool        `json:"strict?"`
}

type GUITheme string

const (
	LIGHT       GUITheme = "Light"
	DARK                 = "Dark"
	DARK_GREEN           = "DarkGreen"
	DARK_ORANGE          = "DarkOrange"
)

type Preferences struct {
	AddonZipsToKeep          *uint8   `json:"addon-zips-to-keep"`
	SelectedColumns          []string `json:"ui-selected-columns"`
	KeepUserCatalogueUpdated bool     `json:"keep-user-catalogue-updated"` // todo: "keep-user-catalogue-updated?"
	CheckForUpdate           bool     `json:"check-for-update"`            // todo: "check-for-update?"
	SelectedCatalogue        string   `json:"selected-catalogue"`          // todo: enum
	SelectedAddonDir         *string  `json:"selected-addon-dir"`
	SelectedGUITheme         GUITheme `json:"selected-gui-theme"`
}

type CatalogueLocation struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Source string `json:"source"`
}

type Settings struct {
	AddonDirList          []AddonsDir         `json:"addon-dir-list"`
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	Preferences           Preferences         `json:"preferences"`

	// deprecated
	GUITheme          GUITheme `json:"gui-theme,omitempty"`
	SelectedCatalogue string   `json:"selected-catalogue,omitempty"`
	SelectedAddonDir  *string  `json:"selected-addon-dir,omitempty"`
}

// if the user provides their own catalogue list in their config file, it will override these defaults entirely.
// if the `catalogue-location-list` entry is *missing* in the user config file, these will be used instead.
// to use strongbox with no catalogues at all, use `catalogue-location-list []` (empty list) in the user config.
var (
	CAT_SHORT = CatalogueLocation{
		Name:   "short",
		Label:  "Short (default)",
		Source: "https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/short-catalogue.json",
	}
	CAT_FULL = CatalogueLocation{
		Name:   "full",
		Label:  "Full",
		Source: "https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/full-catalogue.json",
	}
	CAT_WOWI = CatalogueLocation{
		Name:   "wowinterface",
		Label:  "WoWInterface",
		Source: "https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/wowinterface-catalogue.json",
	}
	CAT_GITHUB = CatalogueLocation{
		Name:   "github",
		Label:  "GitHub",
		Source: "https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/github-catalogue.json",
	}
)

func default_settings() Settings {
	c := Settings{}
	c.AddonDirList = []AddonsDir{}
	c.GUITheme = LIGHT
	return c
}

// --- public

func LoadSettingsFile(path string) (Settings, error) {
	var settings Settings
	if core.FileExists(path) {
		data, err := core.SlurpBytes(path)
		if err != nil {
			return settings, fmt.Errorf("loading settings file: %w", err)
		}
		err = json.Unmarshal(data, &settings)
		if err != nil {
			return settings, fmt.Errorf("parsing json in settings file: %w", err)
		}
	} else {
		// app does not start if `path` does not exist or is not writable. see `init-dirs`.
		data, err := json.Marshal(default_settings())
		if err != nil {
			return settings, err
		}
		core.Spit(path, string(data))
	}

	// see https://pkg.go.dev/github.com/go-playground/validator
	// see mapstructure

	// rename 'selected-catalog' to 'selected-catalogue'
	// load etag-db
	// 'handle install dir'
	// 'handle compound game tracks'
	// 'remove invalid addon dirs'
	// 'handle selected addon dir'
	// 'remove invalid catalogue location entries'
	// 'add github catalogue'
	// 'remove curseforge catalogue'
	// 'remove tukui catalogue'
	// 'handle column preferences'
	// 'strip unspecced keys'

	// - gui theme can only be certain values otherwise data fails to load

	// new in 8.0
	// selected addon dir, catalogue, gui-theme moved to preferences,
	// and removed from output settings
	settings.Preferences.SelectedAddonDir = settings.SelectedAddonDir
	settings.Preferences.SelectedCatalogue = settings.SelectedCatalogue
	settings.Preferences.SelectedGUITheme = settings.GUITheme
	settings.SelectedCatalogue = ""
	settings.SelectedAddonDir = nil
	settings.GUITheme = ""

	return settings, nil
}

// addon.clj/host-disabled?
// returns `true` if the addon host has been disabled
func HostDisabled(source Source) bool {
	_, present := DISABLED_HOSTS[source]
	return present
}
