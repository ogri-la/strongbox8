package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
	"log/slog"
)

/*
   strongbox settings file wrangling
*/

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

// ---

type CatalogueLocation struct {
	Name   string `json:"name"`   // "short"
	Label  string `json:"label"`  // "Short"
	Source string `json:"source"` // "https://someurl.org/path/to/catalogue.json"
}

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

	result_list := []core.Result{}
	i := 0
	for _, addon := range catalogue.AddonSummaryList {
		if i > 200 {
			break
		}
		//id := fmt.Sprintf("%v/%v/%v", cl.Name, addon.Source, addon.SourceID)
		id := core.UniqueID()
		result_list = append(result_list, core.NewResult(NS_CATALOGUE_ADDON, addon, id))
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
			return settings, fmt.Errorf("failed to load settings file: %w", err)
		}
		err = json.Unmarshal(data, &settings)
		if err != nil {
			return settings, fmt.Errorf("failed to parse JSON in settings file: %w", err)
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
