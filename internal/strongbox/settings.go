package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
)

const DEFAULT_INTERFACE_VERSION = 100000

const NFO_FILENAME = ".strongbox.json"

type PathToAddon = string       // "/path/to/addon-dir/Addon/"
type PathToDirOfAddons = string // "/path/to/addon-dir/"

type GameTrackID = string

const (
	GAMETRACK_RETAIL        GameTrackID = "retail"
	GAMETRACK_CLASSIC                   = "classic"
	GAMETRACK_CLASSIC_TBC               = "classic-tbc"
	GAMETRACK_CLASSIC_WOTLK             = "classic-wotlk"
)

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
)

type SourceMap struct {
	Source   Source
	SourceID string
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
	Ignore                      bool // indicates addon should be ignored
	SourceMapList               []SourceMap
}

type NFO struct {
	InstalledVersion     string      `json:"installed-version"`
	ID                   string      `json:"name"`
	GroupID              string      `json:"group-id"`
	Primary              bool        `json:"primary?"`
	Source               Source      `json:"source"`
	InstalledGameTrackID GameTrackID `json:"installed-game-track"`
	SourceID             string      `json:"source-id"`
	SouceMapList         []SourceMap `json:"source-map-list"`
	Ignored              bool        `json:"ignore?"`
	PinnedVersion        string      `json:"pinned-version"`
}

// previously 'summary' or 'addon summary'
type CatalogueAddon struct {
	URL             string        `json:"url"`
	ID              string        `json:"name"`
	Label           string        `json:"label"`
	Description     string        `json:"description"`
	TagList         []string      `json:"tag-list"`
	UpdatedDate     string        `json:"updated-date"`
	DownloadCount   int           `json:"download-count"`
	Source          Source        `json:"source"`
	SourceID        string        `json:"source-id"`
	GameTrackIDList []GameTrackID `json:"game-track-list"`
}

type InstalledAddon struct {
	// an addon may have many .toc files, keyed by game track.
	// the toc data eventually used is determined by the selected addon dir's game track.
	TOC map[GameTrackID]TOC

	// an addon has a single `strongbox.json` 'nfo' file,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	NFO    []NFO
	Source string

	CatalogueAddon *CatalogueAddon // the catalogue match, if any
}

// an 'addon' represents one or many installed addons.
// the group has a representative 'primary' addon,
// representative TOC data according to the selected game track of the addon dir the addon lives in,
// representative NFO data according to whether the addon is overriding or being overridden by other addons.
type Addon struct {
	GroupedAddons []InstalledAddon
	PrimaryAddon  *InstalledAddon

	// derived data.
	TOC TOC
	NFO NFO
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
	Path      string      `json:"addon-dir"`
	GameTrack GameTrackID `json:"game-track"`
	Strict    bool        `json:"strict?"`
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

type Config struct {
	AddonDirList          []AddonsDir         `json:"addon-dir-list"`
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	Preferences           Preferences         `json:"preferences"`

	// deprecated
	GUITheme          GUITheme `json:"gui-theme,omitempty"`
	SelectedCatalogue string   `json:"selected-catalogue,omitempty"`
	SelectedAddonDir  *string  `json:"selected-addon-dir,omitempty"`
}

// ---

func default_settings() Config {
	c := Config{}
	c.AddonDirList = []AddonsDir{}
	c.GUITheme = LIGHT
	return c
}

func load_settings_file(path string) (Config, error) {
	var settings Config
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
