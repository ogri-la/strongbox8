package strongbox

import (
	"bw/core"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

/*
   strongbox settings file wrangling.
   see models.go for spec/constants values.
*/

type GUITheme string

const (
	GUI_THEME_LIGHT       GUITheme = "light"
	GUI_THEME_DARK        GUITheme = "dark"
	GUI_THEME_DARK_GREEN  GUITheme = "dark-green"
	GUI_THEME_DARK_ORANGE GUITheme = "dark-orange"
)

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

	// dead
	CAT_TUKUI = CatalogueLocation{
		Name: "tukui",
	}
	CAT_CURSEFORGE = CatalogueLocation{
		Name: "curseforge",
	}
)

// the default set of catalogue locations.
// order is significant with the first item being the default to use when no catalogue selected.
var DEFAULT_CATALOGUE_LOC_LIST = []CatalogueLocation{
	CAT_SHORT,
	CAT_FULL,
	CAT_WOWI,
	CAT_GITHUB,
}

var DEFAULT_CATALOGUE_LOC = DEFAULT_CATALOGUE_LOC_LIST[0]

// specs.clj/known-column-list
// all known columns. also constitutes the column order.
var COL_LIST_KNOWN = []string{
	"starred",
	"browse-local",
	"source",
	"source-id",
	"source-map-list",
	"name",
	"description",
	"tag-list",
	"created-date",
	"updated-date",
	"dirsize",
	"installed-version",
	"available-version",
	"combined-version",
	"game-version",
}

// specs.clj/default-column-list--v1
var COL_LIST_DEFAULT_V1 = []string{
	"source",
	"name",
	"description",
	"installed-version",
	"available-version",
	"game-version",
	"uber-button",
}

// specs.clj/default-column-list--v2
var COL_LIST_DEFAULT_V2 = []string{
	"source",
	"name",
	"description",
	"combined-version",
	"game-version",
	"uber-button",
}

// specs.clj/default-column-list
var COL_LIST_DEFAULT = COL_LIST_DEFAULT_V2

var COL_LIST_SKINNY = []string{
	"name",
	"version",
	"combined-version",
	"game-version",
	"uber-button",
}

var COL_LIST_FAT = []string{
	"starred",
	"browse-local",
	"source",
	"source-id",
	"name",
	"description",
	"tag-list",
	"created-date",
	"updated-date",
	"dirsize",
	"installed-version",
	"available-version",
	"game-version",
	"uber-version",
}

var COL_PRESET_LIST = map[string][]string{
	"default": COL_LIST_DEFAULT,
	"skinny":  COL_LIST_SKINNY,
	"fat":     COL_LIST_FAT,
}

// ---

type Preferences struct {
	AddonZipsToKeep          *uint8   `json:"addon-zips-to-keep,omitempty"`          // a nil value is important here
	CheckForUpdate           *bool    `json:"check-for-update,omitempty"`            // future: false
	KeepUserCatalogueUpdated *bool    `json:"keep-user-catalogue-updated,omitempty"` // todo: "keep-user-catalogue-updated?"
	SelectedAddonsDir        string   `json:"selected-addon-dir"`
	SelectedCatalogue        string   `json:"selected-catalogue"` // todo: enum
	SelectedColumns          []string `json:"ui-selected-columns"`
	SelectedGUITheme         GUITheme `json:"selected-gui-theme"`
}

// ---

type Settings struct {
	AddonsDirList         []AddonsDir         `json:"addon-dir-list"` // note: do not rename 'addons-dir-list'
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	Preferences           Preferences         `json:"preferences"`

	// deprecated.
	// do not preserve these values when writing settings files

	DeprecatedGUITheme          GUITheme `json:"gui-theme,omitempty"`          // moved to Preferences.SelectedGUITheme in 8.0
	DeprecatedSelectedCatalog   string   `json:"selected-catalog,omitempty"`   // moved to Preferences.SelectedCatalogue circa 1.0
	DeprecatedSelectedCatalogue string   `json:"selected-catalogue,omitempty"` // moved to Preferences.SelectedCatalogue in 8.0
	DeprecatedSelectedAddonDir  string   `json:"selected-addon-dir,omitempty"` // moved to Preferences.SelectedAddonsDir. note: do not rename 'SelectedAddonsDir'
}

func NewSettings() Settings {
	c := Settings{
		AddonsDirList:         []AddonsDir{},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil, // keep no zips
			KeepUserCatalogueUpdated: Ptr(false),
			CheckForUpdate:           Ptr(true),
			SelectedAddonsDir:        "",
			SelectedCatalogue:        DEFAULT_CATALOGUE_LOC.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_LIGHT,
		},

		DeprecatedGUITheme: GUI_THEME_LIGHT, // deprecated
	}
	return c
}

func read_settings_file(path PathToFile) (Settings, error) {
	empty_result := Settings{}
	path = strings.TrimSpace(path)
	if path == "" {
		return empty_result, fmt.Errorf(`path to settings file cannot be empty`)
	}

	if !filepath.IsAbs(path) {
		return empty_result, fmt.Errorf("path to settings file must be absolute")
	}

	if filepath.Ext(path) == "json" {
		return empty_result, fmt.Errorf("path to settings file must have a .json extension: %v", path)
	}

	if !core.FileExists(path) {
		return empty_result, fmt.Errorf("settings file not found: %v", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return empty_result, err
	}

	var settings Settings
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return empty_result, err
	}

	return settings, nil
}

// configures/parses/validates settings data
func configure_settings(settings Settings) Settings {
	default_settings := NewSettings()

	// load etag-db

	// 'handle install dir'
	// - going to remove this in 8.0, it doesn't fit neatly anymore

	// 'remove invalid catalogue location entries'
	// 'handle column preferences'

	// - gui theme can only be certain values otherwise data fails to load

	// new in 8.0
	// selected addon dir, catalogue, gui-theme moved to preferences and removed from output settings

	if settings.DeprecatedSelectedAddonDir != "" {
		settings.Preferences.SelectedAddonsDir = settings.DeprecatedSelectedAddonDir
	}

	if settings.DeprecatedSelectedCatalog != "" {
		// very old setting is in use and the new location is empty.
		if settings.Preferences.SelectedCatalogue == "" {
			settings.Preferences.SelectedCatalogue = settings.DeprecatedSelectedCatalog
		} else {
			slog.Warn("'settings.selected-catalog' is set and will be ignored")
		}
	}

	if settings.DeprecatedSelectedCatalogue != "" {
		settings.Preferences.SelectedCatalogue = settings.DeprecatedSelectedCatalogue
	}

	if settings.DeprecatedGUITheme != "" {
		settings.Preferences.SelectedGUITheme = settings.DeprecatedGUITheme
	}

	// empty whatever values we have stored here to prevent them being propagated forwards.
	settings.DeprecatedSelectedCatalog = ""
	settings.DeprecatedSelectedCatalogue = ""
	settings.DeprecatedSelectedAddonDir = ""
	settings.DeprecatedGUITheme = ""

	// --- set defaults if empty

	if len(settings.CatalogueLocationList) == 0 {
		settings.CatalogueLocationList = DEFAULT_CATALOGUE_LOC_LIST
	}

	if settings.Preferences.CheckForUpdate == nil {
		settings.Preferences.CheckForUpdate = default_settings.Preferences.CheckForUpdate
	}

	if settings.Preferences.KeepUserCatalogueUpdated == nil {
		settings.Preferences.KeepUserCatalogueUpdated = default_settings.Preferences.KeepUserCatalogueUpdated
	}

	if settings.Preferences.SelectedCatalogue == "" {
		settings.Preferences.SelectedCatalogue = default_settings.Preferences.SelectedCatalogue
	}

	if len(settings.Preferences.SelectedColumns) == 0 {
		settings.Preferences.SelectedColumns = default_settings.Preferences.SelectedColumns
	}

	if settings.Preferences.SelectedGUITheme == "" {
		settings.Preferences.SelectedGUITheme = default_settings.Preferences.SelectedGUITheme
	}

	// --- fix up catalogues

	new_cat_locs := []CatalogueLocation{}
	has_github := false
	for _, cl := range settings.CatalogueLocationList {
		// 'remove curseforge catalogue'
		if cl.Name == CAT_CURSEFORGE.Name {
			continue
		}

		// 'remove tukui catalogue'
		if cl.Name == CAT_TUKUI.Name {
			continue
		}

		if cl.Name == CAT_GITHUB.Name {
			has_github = true
		}

		new_cat_locs = append(new_cat_locs, cl)
	}
	// 'add github catalogue'
	if !has_github {
		new_cat_locs = append(new_cat_locs, CAT_GITHUB)
	}
	settings.CatalogueLocationList = new_cat_locs

	// --- handle addon dirs

	// remove any invalid addon dirs (DNE, ...)
	new_addon_dirs := []AddonsDir{}
	for _, ad := range settings.AddonsDirList {
		// `/tmp` prefix check is for testing fixtures
		if !core.DirExists(ad.Path) && !strings.HasPrefix(ad.Path, "/tmp/") {
			continue
		}

		// handle 'strict?' potentially missing and defaulting to 'false'
		if ad.StrictPtr == nil || *ad.StrictPtr {
			ad.Strict = true
		}

		ad.StrictPtr = nil

		new_addon_dirs = append(new_addon_dirs, ad)
	}
	settings.AddonsDirList = new_addon_dirs

	// 'handle selected addon dir'
	// - selected addon dir must exist in list of addon dirs and also exist on fs
	present := false
	for _, ad := range settings.AddonsDirList {
		present = present || ad.Path == settings.Preferences.SelectedAddonsDir
	}
	if !present && len(settings.AddonsDirList) > 0 {
		settings.Preferences.SelectedAddonsDir = settings.AddonsDirList[0].Path
	}

	// --- handle compound game tracks
	for i, ad := range settings.AddonsDirList {
		if is_compound_game_track(ad.GameTrackID) {
			settings.AddonsDirList[i] = convert_compound_game_track(ad)
		}
	}

	return settings
}

// reads settings from disk using the path stored in app state,
// using default settings if necessary.
// configures/parses/validates the unmarshaled data and then stores it in app state.
func LoadSettings(app *core.App) {
	slog.Info("loading settings")
	settings, err := read_settings_file(app.State.KeyVal("strongbox.paths.cfg-file"))
	if err != nil {
		slog.Warn("failed to load settings, using default settings", "error", err)
		settings = NewSettings()
	}

	settings = configure_settings(settings)

	result_list := []core.Result{}

	result := core.MakeResult(NS_SETTINGS, settings, ID_SETTINGS)
	//app.SetResults(result).Wait()
	result_list = append(result_list, result)

	// add each of the catalogue locations to app state.
	//result_list := []core.Result{}
	for _, catalogue_loc := range settings.CatalogueLocationList {
		result_list = append(result_list, core.MakeResult(NS_CATALOGUE_LOC, catalogue_loc, core.UniqueID()))
	}
	//app.SetResults(result_list...).Wait()

	// add each of the addon directories

	for _, addons_dir := range settings.AddonsDirList {
		res := core.MakeResult(NS_ADDONS_DIR, addons_dir, addons_dir.Path)

		// selected addons dirs should show their children (addons) by default.
		// in a gui, this means expand the row contents.
		if settings.Preferences.SelectedAddonsDir == addons_dir.Path {
			res.Tags.Add(core.TAG_SHOW_CHILDREN)
		}

		result_list = append(result_list, res)
	}

	app.SetResults(result_list...).Wait()
}

// ---

// fetch the preferences stored in state
func find_settings(app *core.App) (Settings, error) {
	empty_result := Settings{}
	result_ptr := app.GetResult(ID_SETTINGS)
	if result_ptr == nil {
		return empty_result, errors.New("strongbox settings not found in app state")
	}
	settings, is_settings := result_ptr.Item.(Settings)
	if !is_settings {
		return empty_result, fmt.Errorf("unexpected type: %v", reflect.TypeOf(result_ptr.Item))
	}
	return settings, nil
}

func FindSettings(app *core.App) Settings {
	s, e := find_settings(app)
	if e != nil {
		slog.Error("failed to find settings. they should be available by now", "error", e)
		panic("programming error")
	}
	return s
}

// ---

func _save_settings(s Settings, cfg_file PathToFile) error {
	prefix := ""
	indent := "    "
	bs, err := json.MarshalIndent(s, prefix, indent)
	if err != nil {
		return err
	}
	return core.Spit(cfg_file, bs)
}

// core.clj/save-settings!
// fetches the current settings in app state and writes it to the filesystem.
func save_settings_file(app *core.App) error {
	cfg_file := get_paths(app)["strongbox.paths.cfg-file"]
	if cfg_file == "" {
		return fmt.Errorf("output path in app state is empty: strongbox.paths.cfg-file")
	}
	settings, err := find_settings(app)
	if err != nil {
		return fmt.Errorf("failed to find settings: %v", err)
	}
	return _save_settings(settings, cfg_file)
}

func SaveSettings(app *core.App) {
	slog.Info("saving settings")
	err := save_settings_file(app)
	if err != nil {
		slog.Error("failed to save settings", "error", err)
	}
}
