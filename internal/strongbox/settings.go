package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
	"log/slog"
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
	NS_TOC             = core.NS{Major: "strongbox", Minor: "addon", Type: "toc"}
	NS_PREFS           = core.NS{Major: "strongbox", Minor: "settings", Type: "preference"}
)

const DEFAULT_INTERFACE_VERSION = 100000

const NFO_FILENAME = ".strongbox.json"

var GT_PREF_MAP map[GameTrackID][]GameTrackID = map[GameTrackID][]GameTrackID{
	GAMETRACK_RETAIL:        {GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK},
	GAMETRACK_CLASSIC:       {GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_TBC:   {GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_WOTLK: {GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
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
	SOURCE_GITLAB Source = "gitlab"
	SOURCE_WOWI   Source = "wowinterface"

	// dead
	SOURCE_CURSEFORGE          Source = "curseforge"
	SOURCE_TUKUI               Source = "tukui"
	SOURCE_TUKUI_CLASSIC       Source = "tukui-classic"
	SOURCE_TUKUI_CLASSIC_TBC   Source = "tukui-classic-tbc"
	SOURCE_TUKUI_CLASSIC_WOTLK Source = "tukui-classic-wotlk"
)

var DISABLED_HOSTS = map[Source]bool{
	SOURCE_CURSEFORGE:          true,
	SOURCE_TUKUI:               true,
	SOURCE_TUKUI_CLASSIC:       true,
	SOURCE_TUKUI_CLASSIC_TBC:   true,
	SOURCE_TUKUI_CLASSIC_WOTLK: true,
}

type GUITheme string

const (
	LIGHT       GUITheme = "Light"
	DARK        GUITheme = "Dark"
	DARK_GREEN  GUITheme = "DarkGreen"
	DARK_ORANGE GUITheme = "DarkOrange"
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
	Name   string `json:"name"`   // "short"
	Label  string `json:"label"`  // "Short"
	Source string `json:"source"` // "https://someurl.org/path/to/catalogue.json"
}

// ---

func (cl CatalogueLocation) ItemKeys() []string {
	return []string{
		"name",
		"url",
	}
}

func (cl CatalogueLocation) ItemMap() map[string]string {
	return map[string]string{
		"name": cl.Label,
		"url":  cl.Source,
	}
}

func (cl CatalogueLocation) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (cl CatalogueLocation) ItemChildren(app *core.App) []core.Result {
	path_to_catalogue := CataloguePath(app, cl.Name)
	catalogue, err := ReadCatalogue(path_to_catalogue)
	if err != nil {
		slog.Error("failed to read catalogue", "catalogue", cl)
		return nil
	}

	// eh, technically a CatalogueLocation's children would be a single Catalogue

	ns := core.NewNS("strongbox", "addon", "catalogue-addon")
	result_list := []core.Result{}
	i := 0
	for _, addon := range catalogue.AddonSummaryList {
		if i > 200 {
			break
		}
		result_list = append(result_list, core.NewResult(ns, addon, core.UniqueID()))
		i++
	}
	return result_list
}

var _ core.ItemInfo = (*CatalogueLocation)(nil)

// ---

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
