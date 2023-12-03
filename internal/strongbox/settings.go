package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
)

type GameTrack string

const (
	Retail       GameTrack = "retail"
	Classic                = "classic"
	ClassicTBC             = "classic-tbc"
	ClassicWOTLK           = "classic-wotlk"
)

type AddonDir struct {
	AddonDir  string    `json:"addon-dir"`
	GameTrack GameTrack `json:"game-track"`
	Strict    bool      `json:"strict?"`
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
}

type CatalogueLocation struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Source string `json:"source"`
}

type Config struct {
	AddonDirList          []AddonDir          `json:"addon-dir-list"`
	SelectedAddonDir      *string             `json:"selected-addon-dir"`
	GUITheme              GUITheme            `json:"gui-theme"`
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	SelectedCatalogue     string              `json:"selected-catalogue"`
	Preferences           Preferences         `json:"preferences"`
}

// ---

func default_settings() Config {
	c := Config{}
	c.AddonDirList = []AddonDir{}
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

	return settings, nil
}
