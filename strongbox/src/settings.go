package strongbox

import (
	"bw/core"
	"context"
	"encoding/json"
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

// strongbox settings file wrangling.
//
// a settings file is read in three steps:
//  1. decoded leniently into `settings_file`, the superset of every key any strongbox has
//     written, so one bad value is reported rather than failing the whole file;
//  2. converted explicitly into `Settings` by `settings_from_v7` or `settings_from_v8`,
//     each `Settings` field assigned once from named inputs. there is no generic merge;
//  3. validated entry by entry by `validate_settings`, which discards bad entries with a
//     WARN and fills in defaults.
//
// every step returns the problems it found as `Issue`s rather than logging, so each step
// is a pure function of its inputs.

type GUITheme string

const (
	GUI_THEME_LIGHT       GUITheme = "light"
	GUI_THEME_DARK        GUITheme = "dark"
	GUI_THEME_DARK_GREEN  GUITheme = "dark-green"
	GUI_THEME_DARK_ORANGE GUITheme = "dark-orange"
)

// the settings file format version strongbox 8 writes.
// a file declaring a greater version was written by a newer strongbox.
const SETTINGS_VERSION = 1

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

// the default set of catalogue locations.
// order is significant with the first item being the default to use when no catalogue selected.
var DEFAULT_CATALOGUE_LOC_LIST = []CatalogueLocation{
	CAT_SHORT,
	CAT_FULL,
	CAT_WOWI,
	CAT_GITHUB,
}

var DEFAULT_CATALOGUE_LOC = DEFAULT_CATALOGUE_LOC_LIST[0]

// the names of the catalogues strongbox 0.x to 4.x shipped with by default.
// a list with exactly these names is the old default and is replaced by the current one.
var V1_DEFAULT_CATALOGUE_NAMES = mapset.NewSet("short", "full", "tukui", "curseforge", "wowinterface")

// catalogues for hosts that no longer exist.
var DEAD_CATALOGUE_NAMES = mapset.NewSet("curseforge", "tukui")

// all known columns. the order here is the column order.
// clj: `specs.clj/known-column-list`
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

// maps a column name in `ui-selected-columns` to the installed tab column showing it, a key
// in `Addon.ItemMap`. strongbox 7 columns without an equivalent are absent.
var COL_KEY_MAP = map[string]string{
	"source":            "source",
	"source-id":         "source-id",
	"name":              "name",
	"description":       "description",
	"tag-list":          "tags",
	"tags":              "tags", // strongbox 7's default column lists used 'tags'
	"created-date":      "created-date",
	"updated-date":      "updated-date",
	"dirsize":           "size",
	"installed-version": "installed-version",
	"available-version": "available-version",
	"combined-version":  "combined-version",
	"game-version":      "game-version",
}

// returns the installed tab column keys for every column the installed tab offers, in
// display order.
func InstalledColumnKeys() []string {
	key_list := []string{}
	for _, name := range COL_LIST_KNOWN {
		if key, ok := COL_KEY_MAP[name]; ok && !slices.Contains(key_list, key) {
			key_list = append(key_list, key)
		}
	}
	return key_list
}

// returns the set of installed tab column keys to show for the column names
// `selected_columns`. unknown names are ignored.
func SelectedColumnKeys(selected_columns []string) mapset.Set[string] {
	key_set := mapset.NewSet[string]()
	for _, name := range selected_columns {
		if key, ok := COL_KEY_MAP[name]; ok {
			key_set.Add(key)
		}
	}
	return key_set
}

// clj: `specs.clj/default-column-list--v1`
var COL_LIST_DEFAULT_V1 = []string{
	"source",
	"name",
	"description",
	"installed-version",
	"available-version",
	"game-version",
	"uber-button",
}

// differs from strongbox 7's second default list, which lacked 'tags' and 'created-date'.
var COL_LIST_DEFAULT_V2 = []string{
	"source",
	"name",
	"description",
	"tags",
	"created-date",
	"combined-version",
	"game-version",
	"uber-button",
}

// clj: `specs.clj/default-column-list`
var COL_LIST_DEFAULT = COL_LIST_DEFAULT_V2

// ---

type Preferences struct {
	AddonZipsToKeep          *int     `json:"addon-zips-to-keep"`          // nil is 'keep all', 0 is 'keep zero', 1 is 'keep one', etc
	CheckForUpdate           bool     `json:"check-for-update"`            // check for a newer strongbox at startup
	KeepUserCatalogueUpdated bool     `json:"keep-user-catalogue-updated"` // refresh the user catalogue when it is old
	SelectedAddonsDir        string   `json:"selected-addon-dir"`
	SelectedCatalogue        string   `json:"selected-catalogue"`
	SelectedColumns          []string `json:"ui-selected-columns"`
	SelectedGUITheme         GUITheme `json:"selected-gui-theme"` // preserved, has no effect in strongbox 8
}

type SettingsSpec struct {
	Version int `json:"version"`
}

type Settings struct {
	Spec                  SettingsSpec        `json:"spec"`
	AddonsDirList         []AddonsDir         `json:"addon-dir-list"` // note: do not rename 'addons-dir-list'
	CatalogueLocationList []CatalogueLocation `json:"catalogue-location-list"`
	Preferences           Preferences         `json:"preferences"`
}

func NewSettings() Settings {
	return Settings{
		Spec:                  SettingsSpec{Version: SETTINGS_VERSION},
		AddonsDirList:         []AddonsDir{},
		CatalogueLocationList: slices.Clone(DEFAULT_CATALOGUE_LOC_LIST),
		Preferences: Preferences{
			AddonZipsToKeep:          nil, // keep all zips
			CheckForUpdate:           true,
			KeepUserCatalogueUpdated: false,
			SelectedAddonsDir:        "",
			SelectedCatalogue:        DEFAULT_CATALOGUE_LOC.Name,
			SelectedColumns:          slices.Clone(COL_LIST_DEFAULT),
			SelectedGUITheme:         GUI_THEME_LIGHT,
		},
	}
}

// --- issues

// a problem found while reading settings, logged by the caller at `Level`.
type Issue struct {
	Level   slog.Level
	Message string
}

func debug_issue(format string, args ...any) Issue {
	return Issue{Level: slog.LevelDebug, Message: fmt.Sprintf(format, args...)}
}

func warn_issue(format string, args ...any) Issue {
	return Issue{Level: slog.LevelWarn, Message: fmt.Sprintf(format, args...)}
}

// logs each of the given `issue_list`, naming the settings file they came from.
func log_issues(issue_list []Issue, path string) {
	for _, issue := range issue_list {
		slog.Log(context.Background(), issue.Level, issue.Message, "settings-file", path)
	}
}

// --- lenient decoding

// a JSON value that never fails to decode: a value of the wrong type is recorded as bad
// instead of failing the decode of everything around it.
type lenient[T any] struct {
	Val     T
	Present bool // the key was in the file
	Null    bool // the value was `null`
	Bad     bool // the value was present but not a `T`
	Raw     string
}

func (l *lenient[T]) UnmarshalJSON(b []byte) error {
	l.Present = true
	l.Raw = string(b)
	if string(b) == "null" {
		l.Null = true
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		l.Bad = true
		return nil
	}
	l.Val = v
	return nil
}

// returns `true` when the value is present, not null and of the right type.
func (l lenient[T]) ok() bool {
	return l.Present && !l.Null && !l.Bad
}

// the superset of every key any strongbox has written to a settings file.
// strongbox 7 used the top-level selected values, `install-dir`, `selected-catalog`,
// `gui-theme` and per addons dir `strict?`. strongbox 8 uses `spec`, the selected values in
// `preferences` and per addons dir `strict`.
type settings_file struct {
	Spec                  lenient[spec_file]         `json:"spec"`
	AddonDirList          lenient[[]json.RawMessage] `json:"addon-dir-list"`
	CatalogueLocationList lenient[[]json.RawMessage] `json:"catalogue-location-list"`
	Preferences           lenient[preferences_file]  `json:"preferences"`

	// strongbox 7 and earlier
	InstallDir        lenient[string] `json:"install-dir"`
	SelectedCatalog   lenient[string] `json:"selected-catalog"`
	SelectedCatalogue lenient[string] `json:"selected-catalogue"`
	SelectedAddonDir  lenient[string] `json:"selected-addon-dir"`
	GUITheme          lenient[string] `json:"gui-theme"`
}

// the top-level keys `settings_file` knows. any other key is reported and dropped.
var KNOWN_SETTINGS_KEYS = mapset.NewSet(
	"spec", "addon-dir-list", "catalogue-location-list", "preferences",
	"install-dir", "selected-catalog", "selected-catalogue", "selected-addon-dir", "gui-theme",
)

type spec_file struct {
	Version lenient[int] `json:"version"`
}

type preferences_file struct {
	AddonZipsToKeep          lenient[int]      `json:"addon-zips-to-keep"`
	CheckForUpdate           lenient[bool]     `json:"check-for-update"`
	KeepUserCatalogueUpdated lenient[bool]     `json:"keep-user-catalogue-updated"`
	SelectedAddonDir         lenient[string]   `json:"selected-addon-dir"`
	SelectedCatalogue        lenient[string]   `json:"selected-catalogue"`
	SelectedColumns          lenient[[]string] `json:"ui-selected-columns"`
	SelectedGUITheme         lenient[string]   `json:"selected-gui-theme"`
}

type addons_dir_file struct {
	Path      lenient[string] `json:"addon-dir"`
	GameTrack lenient[string] `json:"game-track"`
	Strict    lenient[bool]   `json:"strict"`  // strongbox 8
	StrictQ   lenient[bool]   `json:"strict?"` // strongbox 7
}

type catalogue_location_file struct {
	Name   lenient[string] `json:"name"`
	Label  lenient[string] `json:"label"`
	Source lenient[string] `json:"source"`
}

// the settings file formats strongbox has written.
type settings_format string

const (
	SETTINGS_FORMAT_V7 settings_format = "strongbox 7"
	SETTINGS_FORMAT_V8 settings_format = "strongbox 8"
)

// returns the format of the decoded settings `f`.
// a file declaring a version is strongbox 8. so is a strongbox 8 pre-release file, which
// has no version but keeps the selected addons dir or theme in its preferences, or has
// addons dirs with `strict` rather than `strict?`. anything else is strongbox 7.
func probe_settings_format(f settings_file, addon_dir_list []addons_dir_file) settings_format {
	if f.Spec.Present {
		return SETTINGS_FORMAT_V8
	}
	if f.Preferences.Val.SelectedAddonDir.Present || f.Preferences.Val.SelectedGUITheme.Present {
		return SETTINGS_FORMAT_V8
	}
	for _, ad := range addon_dir_list {
		if ad.Strict.Present && !ad.StrictQ.Present {
			return SETTINGS_FORMAT_V8
		}
	}
	return SETTINGS_FORMAT_V7
}

// returns the addons dir entries in `raw_list`, each decoded on its own so one bad entry
// cannot spoil the others.
func decode_addons_dir_list(raw_list []json.RawMessage) ([]addons_dir_file, []Issue) {
	issue_list := []Issue{}
	ad_list := []addons_dir_file{}
	for i, raw := range raw_list {
		ad := addons_dir_file{}
		if err := json.Unmarshal(raw, &ad); err != nil {
			issue_list = append(issue_list, warn_issue("discarding addons dir %d, it is not an object: %s", i+1, string(raw)))
			continue
		}
		ad_list = append(ad_list, ad)
	}
	return ad_list, issue_list
}

// returns the catalogue locations in `raw_list`, each decoded on its own.
// entries that are not objects, or lack a name or source, are discarded with an issue.
func decode_catalogue_location_list(raw_list []json.RawMessage) ([]CatalogueLocation, []Issue) {
	issue_list := []Issue{}
	cl_list := []CatalogueLocation{}
	for i, raw := range raw_list {
		clf := catalogue_location_file{}
		if err := json.Unmarshal(raw, &clf); err != nil || !clf.Name.ok() || !clf.Source.ok() {
			issue_list = append(issue_list, warn_issue("discarding catalogue location %d, it needs a name and a source: %s", i+1, string(raw)))
			continue
		}
		cl_list = append(cl_list, CatalogueLocation{Name: clf.Name.Val, Label: clf.Label.Val, Source: clf.Source.Val})
	}
	return cl_list, issue_list
}

// returns the top-level keys in `b` that `settings_file` does not know, sorted.
func unknown_settings_keys(b []byte) []string {
	key_map := map[string]json.RawMessage{}
	_ = json.Unmarshal(b, &key_map) // only the names are wanted, the caller has validated `b`
	unknown := []string{}
	for key := range key_map {
		if !KNOWN_SETTINGS_KEYS.Contains(key) {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	return unknown
}

// --- conversion

// returns `true` when `gt` is one of the dead 'compound' game tracks of strongbox 3.x.
func is_compound_game_track(gt GameTrackID) bool {
	return gt == GAMETRACK_RETAIL_CLASSIC || gt == GAMETRACK_CLASSIC_RETAIL
}

// returns the live game track a dead compound game track meant: its first part.
// 'retail-classic' => retail, 'classic-retail' => classic.
func convert_compound_game_track(gt GameTrackID) GameTrackID {
	switch gt {
	case GAMETRACK_RETAIL_CLASSIC:
		return GAMETRACK_RETAIL
	case GAMETRACK_CLASSIC_RETAIL:
		return GAMETRACK_CLASSIC
	}
	return gt
}

// returns the addons dir described by `ad`, with strictness `strict` when
// `strict_present`. otherwise a compound game track is not strict and any other is.
// a compound game track becomes its first part.
func addons_dir_from_file(ad addons_dir_file, strict_present bool, strict bool) AddonsDir {
	game_track := GameTrackID(ad.GameTrack.Val)
	if !strict_present {
		strict = !is_compound_game_track(game_track)
	}
	return AddonsDir{
		Path:        ad.Path.Val,
		GameTrackID: convert_compound_game_track(game_track),
		Strict:      strict,
	}
}

// returns the catalogue location list from `f`: the default list when absent, the
// current default when it is the 0.x–4.x default, and without dead hosts.
func catalogue_locations_from_file(f settings_file) ([]CatalogueLocation, []Issue) {
	if !f.CatalogueLocationList.Present {
		return slices.Clone(DEFAULT_CATALOGUE_LOC_LIST), nil
	}
	if !f.CatalogueLocationList.ok() {
		return []CatalogueLocation{}, []Issue{warn_issue("'catalogue-location-list' is not a list, no catalogues will be used: %s", f.CatalogueLocationList.Raw)}
	}
	cl_list, issue_list := decode_catalogue_location_list(f.CatalogueLocationList.Val)

	name_set := mapset.NewSet[string]()
	for _, cl := range cl_list {
		name_set.Add(cl.Name)
	}
	if len(cl_list) > 0 && name_set.Equal(V1_DEFAULT_CATALOGUE_NAMES) {
		issue_list = append(issue_list, debug_issue("replacing the old default catalogue list with the current default"))
		return slices.Clone(DEFAULT_CATALOGUE_LOC_LIST), issue_list
	}

	live := []CatalogueLocation{}
	for _, cl := range cl_list {
		if DEAD_CATALOGUE_NAMES.Contains(cl.Name) {
			issue_list = append(issue_list, debug_issue("removing catalogue for a host that no longer exists: %s", cl.Name))
			continue
		}
		live = append(live, cl)
	}
	return live, issue_list
}

// returns the preferences in `pf`, each with its default when absent or of the wrong type.
// the selected addons dir, catalogue and theme are left to the caller, they live in
// different places in strongbox 7 and 8 files.
func preferences_from_file(pf preferences_file) (Preferences, []Issue) {
	defaults := NewSettings().Preferences
	issue_list := []Issue{}
	bad := func(key string, raw string) {
		issue_list = append(issue_list, warn_issue("preference '%s' has a bad value and takes its default: %s", key, raw))
	}

	prefs := defaults

	switch {
	case pf.AddonZipsToKeep.Bad || (pf.AddonZipsToKeep.ok() && pf.AddonZipsToKeep.Val < 0):
		bad("addon-zips-to-keep", pf.AddonZipsToKeep.Raw)
	case pf.AddonZipsToKeep.ok():
		prefs.AddonZipsToKeep = new(pf.AddonZipsToKeep.Val)
	}

	if pf.CheckForUpdate.Bad {
		bad("check-for-update", pf.CheckForUpdate.Raw)
	} else if pf.CheckForUpdate.ok() {
		prefs.CheckForUpdate = pf.CheckForUpdate.Val
	}

	if pf.KeepUserCatalogueUpdated.Bad {
		bad("keep-user-catalogue-updated", pf.KeepUserCatalogueUpdated.Raw)
	} else if pf.KeepUserCatalogueUpdated.ok() {
		prefs.KeepUserCatalogueUpdated = pf.KeepUserCatalogueUpdated.Val
	}

	if pf.SelectedColumns.Bad {
		bad("ui-selected-columns", pf.SelectedColumns.Raw)
	} else if pf.SelectedColumns.ok() && len(pf.SelectedColumns.Val) > 0 {
		prefs.SelectedColumns = pf.SelectedColumns.Val
		if mapset.NewSet(prefs.SelectedColumns...).Equal(mapset.NewSet(COL_LIST_DEFAULT_V1...)) {
			issue_list = append(issue_list, debug_issue("replacing the old default column list with the current default"))
			prefs.SelectedColumns = slices.Clone(COL_LIST_DEFAULT)
		}
	}

	return prefs, issue_list
}

// returns the first of `val_list` that is present, a string and not empty, or "".
func first_string(val_list ...lenient[string]) string {
	for _, v := range val_list {
		if v.ok() && strings.TrimSpace(v.Val) != "" {
			return v.Val
		}
	}
	return ""
}

// returns `Settings` from a strongbox 7 (0.9 to 7.x) settings file `f`.
// the single place strongbox 7 values map to strongbox 8 fields.
func settings_from_v7(f settings_file, ad_list []addons_dir_file) (Settings, []Issue) {
	issue_list := []Issue{}
	settings := NewSettings()

	settings.AddonsDirList = []AddonsDir{}
	for _, ad := range ad_list {
		settings.AddonsDirList = append(settings.AddonsDirList, addons_dir_from_file(ad, ad.StrictQ.ok(), ad.StrictQ.Val))
	}

	// a strongbox 0.x '--install-dir' is an extra retail addons dir
	if f.InstallDir.ok() && f.InstallDir.Val != "" {
		listed := slices.ContainsFunc(settings.AddonsDirList, func(ad AddonsDir) bool { return ad.Path == f.InstallDir.Val })
		if !listed {
			settings.AddonsDirList = append(settings.AddonsDirList, AddonsDir{Path: f.InstallDir.Val, GameTrackID: GAMETRACK_RETAIL, Strict: true})
		}
	}

	cl_list, cl_issues := catalogue_locations_from_file(f)
	settings.CatalogueLocationList = cl_list
	issue_list = append(issue_list, cl_issues...)

	prefs, pref_issues := preferences_from_file(f.Preferences.Val)
	issue_list = append(issue_list, pref_issues...)
	prefs.SelectedAddonsDir = first_string(f.SelectedAddonDir)
	prefs.SelectedCatalogue = first_string(f.SelectedCatalogue, f.SelectedCatalog)
	if f.SelectedCatalog.ok() && f.SelectedCatalogue.ok() {
		issue_list = append(issue_list, debug_issue("'selected-catalog' is set and will be ignored"))
	}
	if theme := first_string(f.GUITheme); theme != "" {
		prefs.SelectedGUITheme = GUITheme(theme)
	}
	settings.Preferences = prefs

	return settings, issue_list
}

// returns `Settings` from a strongbox 8 settings file `f`, upgrading a pre-release file
// (version 0) to the current version.
func settings_from_v8(f settings_file, ad_list []addons_dir_file) (Settings, []Issue) {
	issue_list := []Issue{}
	settings := NewSettings()

	settings.AddonsDirList = []AddonsDir{}
	for _, ad := range ad_list {
		strict_present := ad.Strict.ok() || ad.StrictQ.ok()
		strict := ad.Strict.Val
		if !ad.Strict.ok() {
			strict = ad.StrictQ.Val
		}
		if !strict_present {
			strict = true
		}
		settings.AddonsDirList = append(settings.AddonsDirList, addons_dir_from_file(ad, true, strict))
	}

	cl_list, cl_issues := catalogue_locations_from_file(f)
	settings.CatalogueLocationList = cl_list
	issue_list = append(issue_list, cl_issues...)

	prefs, pref_issues := preferences_from_file(f.Preferences.Val)
	issue_list = append(issue_list, pref_issues...)
	prefs.SelectedAddonsDir = first_string(f.Preferences.Val.SelectedAddonDir, f.SelectedAddonDir)
	prefs.SelectedCatalogue = first_string(f.Preferences.Val.SelectedCatalogue, f.SelectedCatalogue)
	// pre-releases also wrote a top-level 'gui-theme'
	if theme := first_string(f.Preferences.Val.SelectedGUITheme, f.GUITheme); theme != "" {
		prefs.SelectedGUITheme = GUITheme(theme)
	}
	settings.Preferences = prefs

	settings.Spec.Version = SETTINGS_VERSION
	return settings, issue_list
}

// --- validation

// returns `settings` with invalid entries discarded and defaults filled in.
// `available` reports whether an addons dir's directory exists. a missing directory is
// kept, it may be on a drive that isn't mounted, but is never chosen as the selected
// addons dir in place of an available one.
func validate_settings(settings Settings, available func(path string) bool) (Settings, []Issue) {
	issue_list := []Issue{}

	// addons dirs
	ad_list := []AddonsDir{}
	seen_paths := mapset.NewSet[string]()
	for _, ad := range settings.AddonsDirList {
		switch {
		case strings.TrimSpace(ad.Path) == "":
			issue_list = append(issue_list, warn_issue("discarding addons dir with no path"))
		case !filepath.IsAbs(ad.Path):
			issue_list = append(issue_list, warn_issue("discarding addons dir, its path is not absolute: %s", ad.Path))
		case !SUPPORTED_GAME_TRACKS.Contains(ad.GameTrackID):
			issue_list = append(issue_list, warn_issue("discarding addons dir, its game track '%s' is not supported: %s", ad.GameTrackID, ad.Path))
		case seen_paths.Contains(filepath.Clean(ad.Path)):
			issue_list = append(issue_list, warn_issue("discarding duplicate addons dir: %s", ad.Path))
		default:
			seen_paths.Add(filepath.Clean(ad.Path))
			ad_list = append(ad_list, ad)
		}
	}
	settings.AddonsDirList = ad_list

	// catalogue locations
	cl_list := []CatalogueLocation{}
	seen_names := mapset.NewSet[string]()
	for _, cl := range settings.CatalogueLocationList {
		switch {
		case strings.TrimSpace(cl.Name) == "":
			issue_list = append(issue_list, warn_issue("discarding catalogue location with no name"))
		case !strings.HasPrefix(cl.Source, "https://"):
			issue_list = append(issue_list, warn_issue("discarding catalogue location '%s', its source is not an https URL: %s", cl.Name, cl.Source))
		case seen_names.Contains(cl.Name):
			issue_list = append(issue_list, warn_issue("discarding duplicate catalogue location: %s", cl.Name))
		default:
			seen_names.Add(cl.Name)
			if cl.Label == "" {
				cl.Label = cl.Name
			}
			cl_list = append(cl_list, cl)
		}
	}
	settings.CatalogueLocationList = cl_list

	// the selected catalogue must be one of the catalogue locations
	if !seen_names.Contains(settings.Preferences.SelectedCatalogue) {
		fallback := ""
		if len(cl_list) > 0 {
			fallback = cl_list[0].Name
		}
		if settings.Preferences.SelectedCatalogue != "" {
			issue_list = append(issue_list, debug_issue("selected catalogue '%s' is not a known catalogue, using '%s'", settings.Preferences.SelectedCatalogue, fallback))
		}
		settings.Preferences.SelectedCatalogue = fallback
	}

	// the selected addons dir must be an available addons dir, when there is one
	selected := settings.Preferences.SelectedAddonsDir
	selected_ok := slices.ContainsFunc(ad_list, func(ad AddonsDir) bool { return ad.Path == selected && available(ad.Path) })
	if !selected_ok {
		fallback := ""
		for _, ad := range ad_list {
			if available(ad.Path) {
				fallback = ad.Path
				break
			}
		}
		settings.Preferences.SelectedAddonsDir = fallback
	}

	if len(settings.Preferences.SelectedColumns) == 0 {
		settings.Preferences.SelectedColumns = slices.Clone(COL_LIST_DEFAULT)
	}
	if settings.Preferences.SelectedGUITheme == "" {
		settings.Preferences.SelectedGUITheme = GUI_THEME_LIGHT
	}

	return settings, issue_list
}

// --- reading

// the outcome of parsing a settings file.
type parsed_settings struct {
	Settings Settings
	Issues   []Issue
	Format   settings_format
	ReadOnly bool // written by a newer strongbox, do not write it back
}

// returns the settings in the settings file contents `b`, migrated and validated.
// returns an error only when `b` is not a JSON object: any other problem is an issue.
func parse_settings(b []byte, available func(path string) bool) (parsed_settings, error) {
	f := settings_file{}
	if err := json.Unmarshal(b, &f); err != nil {
		return parsed_settings{}, fmt.Errorf("settings are not a JSON object: %w", err)
	}

	issue_list := []Issue{}
	if f.Preferences.Bad {
		issue_list = append(issue_list, warn_issue("'preferences' is not an object, default preferences are used: %s", f.Preferences.Raw))
	}
	for _, key := range unknown_settings_keys(b) {
		issue_list = append(issue_list, debug_issue("removing unknown setting: %s", key))
	}

	raw_ad_list := []json.RawMessage{}
	switch {
	case f.AddonDirList.ok():
		raw_ad_list = f.AddonDirList.Val
	case f.AddonDirList.Bad:
		issue_list = append(issue_list, warn_issue("'addon-dir-list' is not a list, no addons dirs loaded: %s", f.AddonDirList.Raw))
	}
	ad_list, ad_issues := decode_addons_dir_list(raw_ad_list)
	issue_list = append(issue_list, ad_issues...)

	result := parsed_settings{Format: probe_settings_format(f, ad_list)}

	var settings Settings
	var conv_issues []Issue
	if result.Format == SETTINGS_FORMAT_V8 {
		settings, conv_issues = settings_from_v8(f, ad_list)
		if f.Spec.Val.Version.Val > SETTINGS_VERSION {
			result.ReadOnly = true
			issue_list = append(issue_list, warn_issue("settings were written by a newer strongbox (version %d), they will not be saved", f.Spec.Val.Version.Val))
		}
	} else {
		settings, conv_issues = settings_from_v7(f, ad_list)
	}
	issue_list = append(issue_list, conv_issues...)

	settings, valid_issues := validate_settings(settings, available)
	issue_list = append(issue_list, valid_issues...)

	result.Settings = settings
	result.Issues = issue_list
	return result, nil
}

// returns a copy of `path` named `<path>.<UTC timestamp>.invalid`, for preserving a file
// that cannot be read before anything is written over it.
func invalid_copy_path(path string, now time.Time) string {
	return fmt.Sprintf("%s.%s.invalid", path, now.UTC().Format("20060102T150405Z"))
}

// the result of loading settings from disk.
type loaded_settings struct {
	Settings Settings
	ReadOnly bool
	Source   string // the file the settings came from, "" for defaults
}

// reads strongbox 8's settings from `cfg_file`, or imports strongbox 7's from
// `v7_cfg_file` when strongbox 8 has none.
// a strongbox 8 settings file that cannot be read is copied aside, never lost, and the
// defaults are used. strongbox 7's files are only ever read.
// a missing user catalogue is copied from strongbox 7's when that exists.
func load_settings(cfg_file string, v7 V7Paths, user_catalogue_file string, available func(string) bool, now time.Time) loaded_settings {
	defaults := func() loaded_settings {
		settings, _ := validate_settings(NewSettings(), available)
		return loaded_settings{Settings: settings}
	}

	if core.FileExists(cfg_file) {
		b, err := os.ReadFile(cfg_file)
		if err != nil {
			slog.Error("failed to read settings file, using default settings", "settings-file", cfg_file, "error", err)
			return loaded_settings{Settings: defaults().Settings, ReadOnly: true} // do not write over what could not be read
		}
		parsed, err := parse_settings(b, available)
		if err != nil {
			copy_path := invalid_copy_path(cfg_file, now)
			if cerr := os.WriteFile(copy_path, b, 0o600); cerr != nil {
				slog.Error("settings file is invalid and could not be preserved, using default settings without saving", "settings-file", cfg_file, "error", err, "copy-error", cerr)
				return loaded_settings{Settings: defaults().Settings, ReadOnly: true}
			}
			slog.Error("settings file is invalid, it has been preserved and default settings are used", "settings-file", cfg_file, "preserved-as", copy_path, "error", err)
			return defaults()
		}
		log_issues(parsed.Issues, cfg_file)
		return loaded_settings{Settings: parsed.Settings, ReadOnly: parsed.ReadOnly, Source: cfg_file}
	}

	result := defaults()
	if core.FileExists(v7.CfgFile) {
		b, err := os.ReadFile(v7.CfgFile)
		if err == nil {
			parsed, perr := parse_settings(b, available)
			if perr == nil {
				log_issues(parsed.Issues, v7.CfgFile)
				slog.Info("imported strongbox 7 settings", "from", v7.CfgFile, "to", cfg_file)
				result = loaded_settings{Settings: parsed.Settings, Source: v7.CfgFile}
			} else {
				err = perr
			}
		}
		if err != nil {
			slog.Warn("failed to import strongbox 7 settings, using default settings", "settings-file", v7.CfgFile, "error", err)
		}
	}

	if !core.FileExists(user_catalogue_file) && core.FileExists(v7.UserCatalogueFile) {
		b, err := os.ReadFile(v7.UserCatalogueFile)
		if err == nil {
			err = write_atomic(user_catalogue_file, b)
		}
		if err != nil {
			slog.Warn("failed to import strongbox 7 user catalogue", "from", v7.UserCatalogueFile, "error", err)
		} else {
			slog.Info("imported strongbox 7 user catalogue", "from", v7.UserCatalogueFile, "to", user_catalogue_file)
		}
	}

	return result
}

// keyval set when the settings must not be written back.
const KV_SETTINGS_READ_ONLY = "strongbox.settings.read-only"

// reads the settings from disk, upgrades them and stores them in app state, along with a
// result per catalogue location and per addons dir.
// a settings file that cannot be read is not an error: the defaults are used instead.
// the selected addons dir is tagged so the GUI expands its children.
func LoadSettings(app *core.App) {
	slog.Info("loading settings")
	paths := get_paths(app)
	v7 := V7Paths{
		CfgFile:           paths["strongbox.paths.v7-cfg-file"],
		UserCatalogueFile: paths["strongbox.paths.v7-user-catalogue-file"],
	}
	loaded := load_settings(paths["strongbox.paths.cfg-file"], v7, paths["strongbox.paths.user-catalogue-file"], core.DirExists, time.Now())
	app.State().SetKeyAnyVal(KV_SETTINGS_READ_ONLY, loaded.ReadOnly)
	settings_to_state(app, loaded.Settings)
}

// replaces the settings, catalogue locations and addons dirs in app state with those in
// `settings`. addons dir availability is derived here and never stored.
// the selected addons dir is tagged so the GUI expands its children.
func settings_to_state(app *core.App, settings Settings) {
	result_list := []core.Result{core.MakeResult(NS_SETTINGS, settings, ID_SETTINGS)}

	for _, catalogue_loc := range settings.CatalogueLocationList {
		result_list = append(result_list, core.MakeResult(NS_CATALOGUE_LOC, catalogue_loc, "catalogue-location:"+catalogue_loc.Name))
	}

	for _, addons_dir := range settings.AddonsDirList {
		addons_dir = derive_addons_dir(addons_dir, settings, core.DirExists)
		result_list = append(result_list, tag_addons_dir_result(MakeAddonsDirResult(addons_dir), addons_dir))
	}

	app.AddReplaceResults(result_list...).Wait()
}

// ---

// returns the settings stored in the given `state`,
// or an error when they are not present.
func find_settings(state *core.State) (Settings, error) {
	empty_result := Settings{}
	result, err := state.GetResult(ID_SETTINGS)
	if err != nil {
		return empty_result, errors.New("strongbox settings not found in app state")
	}
	settings := result.Item.(Settings)
	return settings, nil
}

// returns the settings stored in app state.
// panics if the settings are missing: they are loaded during startup, so their absence
// is a wiring defect rather than a runtime condition.
func FindSettings(app *core.App) Settings {
	s, e := find_settings(app.State())
	if e != nil {
		slog.Error("failed to find settings. they should be available by now", "error", e)
		panic("programming error")
	}
	return s
}

// ---

// writes `settings` to `cfg_file` as indented JSON, atomically, creating parent
// directories.
func save_settings_file(settings Settings, cfg_file PathToFile) error {
	settings.Spec.Version = SETTINGS_VERSION
	b, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}
	return write_atomic(cfg_file, b)
}

// writes the settings held in app state to disk, unless they were written by a newer
// strongbox.
// panics if the settings path has not been set in state.
// clj: `core.clj/save-settings!`
func SaveSettings(app *core.App) error {
	cfg_file := get_paths(app)["strongbox.paths.cfg-file"]
	if cfg_file == "" {
		slog.Error("failed to save settings, output path in app state is empty", "strongbox.paths.cfg-file", cfg_file)
		panic("programming error")
	}

	if read_only, _ := app.State().GetKeyAnyVal(KV_SETTINGS_READ_ONLY).(bool); read_only {
		slog.Debug("not saving settings, they are read-only this session", "cfg-file", cfg_file)
		return nil
	}

	slog.Debug("saving settings", "cfg-file", cfg_file)
	settings := FindSettings(app)
	err := save_settings_file(settings, cfg_file)
	if err != nil {
		slog.Error("failed to save settings to file", "cfg-file", cfg_file, "error", err)
		return err
	}
	return nil
}
