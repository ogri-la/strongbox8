package strongbox

import (
	"bw/core"
	"encoding/json"
	"log/slog"
)

/*
   strongbox settings file wrangling.
   see models.go for spec/constants values.
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
	SelectedAddonsDir        string   `json:"selected-addon-dir"`
	SelectedGUITheme         GUITheme `json:"selected-gui-theme"`
}

// ---

type Settings struct {
	AddonsDirList         []AddonsDir         `json:"addon-dir-list"`
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	Preferences           Preferences         `json:"preferences"`

	// deprecated
	GUITheme          GUITheme `json:"gui-theme,omitempty"`
	SelectedCatalogue string   `json:"selected-catalogue,omitempty"`
	SelectedAddonDir  *string  `json:"selected-addon-dir,omitempty"` // note: do not rename 'SelectedAddonsDir'
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
	c.Preferences = Preferences{}
	c.Preferences.SelectedCatalogue = "short"
	c.AddonsDirList = []AddonsDir{}
	c.GUITheme = LIGHT

	return c
}

func load_settings_file(path string) (Settings, error) {
	var settings Settings
	default_settings := default_settings()
	if !core.FileExists(path) {
		// settings do not exist, write default settings
		data, err := json.Marshal(default_settings)
		if err != nil {
			return settings, err
		}
		core.Spit(path, string(data))
	}

	data, err := core.SlurpBytes(path)
	if err != nil {
		slog.Error("failed to load settings file", "error", err)
		slog.Warn("using default settings")
		settings = default_settings
	}
	err = json.Unmarshal(data, &settings)
	if err != nil {
		slog.Error("failed to parse JSON in settings file", "error", err)
		slog.Warn("using default settings")
		settings = default_settings
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
	if settings.SelectedAddonDir == nil {
		settings.Preferences.SelectedAddonsDir = ""
	} else {
		settings.Preferences.SelectedAddonsDir = *settings.SelectedAddonDir
	}
	settings.Preferences.SelectedCatalogue = settings.SelectedCatalogue
	settings.Preferences.SelectedGUITheme = settings.GUITheme
	settings.SelectedCatalogue = ""
	settings.SelectedAddonDir = nil
	settings.GUITheme = ""

	// sigh, we need some *proper* validation in Go.
	if settings.Preferences.SelectedCatalogue == "" {
		settings.Preferences.SelectedCatalogue = default_settings.SelectedCatalogue
	}

	return settings, nil
}
