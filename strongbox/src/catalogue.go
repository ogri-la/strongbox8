package strongbox

import (
	"bw/core"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// --- Catalogue Addon

// an addon as described by a catalogue, before it is matched against anything installed.
// previously 'summary' or 'addon summary'.
type CatalogueAddon struct {
	URL             string        `json:"url"`
	Name            string        `json:"name"` // normalised name
	Label           string        `json:"label"`
	Description     string        `json:"description"`
	TagList         []string      `json:"tag-list"`
	UpdatedDate     time.Time     `json:"updated-date"`
	CreatedDate     time.Time     `json:"created-date"`
	DownloadCount   int           `json:"download-count"`
	Source          Source        `json:"source"`
	SourceID        FlexString    `json:"source-id"`
	GameTrackIDList []GameTrackID `json:"game-track-list"`

	// derived, never stored
	starred   bool // the addon is in the user catalogue
	installed bool // an addon in the selected addons dir is matched to it
}

var _ core.ItemInfo = (*CatalogueAddon)(nil)

// returns the key identifying a catalogue addon across catalogues: its source and source
// ID. "github/ogri-la/everyaddon"
func (ca CatalogueAddon) Key() string {
	return ca.Source + "/" + string(ca.SourceID)
}

// returns `true` when the addon is in the user catalogue.
func (ca CatalogueAddon) Starred() bool {
	return ca.starred
}

// returns `true` when an addon in the selected addons dir is matched to this one.
func (ca CatalogueAddon) Installed() bool {
	return ca.installed
}

// reads a catalogue addon, accepting timestamps in several formats.
// an unparseable timestamp becomes the zero time rather than an error: a bad date should
// not discard an otherwise usable addon.
func (ca *CatalogueAddon) UnmarshalJSON(data []byte) error {
	// dates are read as strings first so they can be parsed leniently below
	type TempCatalogueAddon struct {
		URL             string        `json:"url"`
		Name            string        `json:"name"`
		Label           string        `json:"label"`
		Description     string        `json:"description"`
		TagList         []string      `json:"tag-list"`
		UpdatedDate     string        `json:"updated-date"`
		CreatedDate     string        `json:"created-date"`
		DownloadCount   int           `json:"download-count"`
		Source          Source        `json:"source"`
		SourceID        FlexString    `json:"source-id"`
		GameTrackIDList []GameTrackID `json:"game-track-list"`
	}

	var temp TempCatalogueAddon
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	ca.URL = temp.URL
	ca.Name = temp.Name
	ca.Label = temp.Label
	ca.Description = temp.Description
	ca.TagList = temp.TagList
	ca.DownloadCount = temp.DownloadCount
	ca.Source = temp.Source
	ca.SourceID = temp.SourceID
	ca.GameTrackIDList = temp.GameTrackIDList

	ca.CreatedDate = parseFlexibleTimestamp(temp.CreatedDate)
	ca.UpdatedDate = parseFlexibleTimestamp(temp.UpdatedDate)

	return nil
}

// returns the given `dateStr` as a time, trying several formats.
// returns the zero time when the string is empty or matches no known format.
func parseFlexibleTimestamp(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	formats := []string{
		time.RFC3339,             // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04Z07:00", // "2024-04-23T16:09Z" (missing seconds)
		"2006-01-02T15:04:05Z",   // "2006-01-02T15:04:05Z"
		"2006-01-02T15:04Z",      // "2006-01-02T15:04Z"
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			return parsed
		}
	}

	slog.Warn("failed to parse timestamp, using zero time", "timestamp", dateStr)
	return time.Time{}
}

func (ca CatalogueAddon) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_URL,
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		"source",
		core.ITEM_FIELD_DATE_UPDATED,
		core.ITEM_FIELD_DATE_CREATED,
		"downloads",
		"tags",
		"starred",
		"installed",
	}
}

func (ca CatalogueAddon) ItemMap() map[string]string {
	var created_str, updated_str string

	if ca.CreatedDate.IsZero() {
		created_str = ""
	} else if created_formatted, err := core.FormatTimeHumanOffset(ca.CreatedDate); err != nil {
		slog.Error("failed to format created date", "catalogue-addon", ca.Name, "created", ca.CreatedDate, "error", err)
		created_str = ""
	} else {
		created_str = created_formatted
	}

	if ca.UpdatedDate.IsZero() {
		updated_str = ""
	} else if updated_formatted, err := core.FormatTimeHumanOffset(ca.UpdatedDate); err != nil {
		slog.Error("failed to format updated date", "catalogue-addon", ca.Name, "updated", ca.UpdatedDate, "error", err)
		updated_str = ""
	} else {
		updated_str = updated_formatted
	}

	return map[string]string{
		core.ITEM_FIELD_URL:          ca.URL,
		core.ITEM_FIELD_NAME:         ca.Label,
		core.ITEM_FIELD_DESC:         ca.Description,
		"source":                     ca.Source,
		core.ITEM_FIELD_DATE_UPDATED: updated_str,
		core.ITEM_FIELD_DATE_CREATED: created_str,
		"downloads":                  strconv.Itoa(ca.DownloadCount),
		"tags":                       strings.Join(ca.TagList, ", "),
		"starred":                    map[bool]string{true: "★", false: ""}[ca.starred],
		"installed":                  map[bool]string{true: "installed", false: ""}[ca.installed],
		"normalised-name":            ca.Name,
	}
}

func (ca CatalogueAddon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (ca CatalogueAddon) ItemChildren(app *core.App) []core.Result {
	return nil
}

// --- Catalogue Location

type CatalogueLocation struct {
	Name   string `json:"name"`   // "short"
	Label  string `json:"label"`  // "Short"
	Source string `json:"source"` // "https://someurl.org/path/to/catalogue.json"
}

var _ core.ItemInfo = (*CatalogueLocation)(nil)

func (cl CatalogueLocation) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
	}
}

func (cl CatalogueLocation) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_NAME: cl.Label,
		core.ITEM_FIELD_URL:  cl.Source,
	}
}

// a CatalogueLocation doesn't have children,
// but a Catalogue that extends a CatalogueLocation *does*.
func (cl CatalogueLocation) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (cl CatalogueLocation) ItemChildren(app *core.App) []core.Result {
	return nil
}

// --- Catalogue

type CatalogueSpec struct {
	Version int `json:"version"`
}

type Catalogue struct {
	CatalogueLocation
	Spec             CatalogueSpec    `json:"spec"`
	Datestamp        string           `json:"datestamp"` // "2026-10-03"
	Total            int              `json:"total"`
	AddonSummaryList []CatalogueAddon `json:"addon-summary-list"`
}

func (c Catalogue) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
		core.ITEM_FIELD_VERSION,
		core.ITEM_FIELD_DATE_UPDATED,
		"total",
	}
}

func (c Catalogue) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_NAME:         c.Label,
		core.ITEM_FIELD_URL:          c.Source,
		core.ITEM_FIELD_VERSION:      strconv.Itoa(c.Total),
		core.ITEM_FIELD_DATE_UPDATED: c.Datestamp,
		"total":                      strconv.Itoa(c.Total),
	}
}

// eager: the search tab lists the addons without the catalogue row being expanded.
func (c Catalogue) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

// returns the catalogue's addons, one result per addon, each identified by its key so
// reloading the catalogue gives the same IDs.
func (c Catalogue) ItemChildren(_ *core.App) []core.Result {
	result_list := make([]core.Result, 0, len(c.AddonSummaryList))
	for _, addon := range c.AddonSummaryList {
		result_list = append(result_list, core.MakeResult(NS_CATALOGUE_ADDON, addon, "catalogue-addon:"+addon.Key()))
	}
	return result_list
}

var _ core.ItemInfo = (*Catalogue)(nil)

// the user's own catalogue. kept in state for reference, its addons are listed through the
// combined catalogue.
type UserCatalogue struct {
	Catalogue
}

func (uc UserCatalogue) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (uc UserCatalogue) ItemChildren(_ *core.App) []core.Result {
	return nil
}

// the selected catalogue as downloaded, before the user catalogue is combined with it.
// kept so the two can be combined again when the user catalogue changes.
type SelectedCatalogue struct {
	Catalogue
}

func (sc SelectedCatalogue) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (sc SelectedCatalogue) ItemChildren(_ *core.App) []core.Result {
	return nil
}

// ---

// how long a downloaded catalogue is used before it is downloaded again.
const CATALOGUE_MAX_AGE = time.Hour

// the longest catalogue addon description kept.
const CATALOGUE_DESCRIPTION_MAX = 255

// the catalogue format version strongbox reads.
const CATALOGUE_VERSION = 2

func catalogue_local_path(data_dir string, filename string) string {
	return filepath.Join(data_dir, filename+"-catalogue.json")
}

// returns the local path of the catalogue with the given `catalogue_name`.
// panics if the catalogue directory has not been set in state yet.
func CataloguePath(app *core.App, catalogue_name string) string {
	val := app.State().GetKeyAnyVal("strongbox.paths.catalogue-dir")
	if val == nil {
		panic("attempted to access strongbox.paths.catalogue-dir before it was present")
	}
	return catalogue_local_path(val.(string), catalogue_name)
}

// the fields every catalogue entry must have, checked before the entry is used.
type catalogue_addon_required struct {
	URL             lenient[string]        `json:"url"`
	Name            lenient[string]        `json:"name"`
	Label           lenient[string]        `json:"label"`
	UpdatedDate     lenient[string]        `json:"updated-date"`
	Source          lenient[string]        `json:"source"`
	SourceID        lenient[FlexString]    `json:"source-id"`
	GameTrackIDList lenient[[]GameTrackID] `json:"game-track-list"`
}

// returns the problem with a catalogue entry missing a required field, or "".
func missing_catalogue_field(req catalogue_addon_required) string {
	for field, ok := range map[string]bool{
		"url":             req.URL.ok() && req.URL.Val != "",
		"name":            req.Name.ok() && req.Name.Val != "",
		"label":           req.Label.ok() && req.Label.Val != "",
		"updated-date":    req.UpdatedDate.ok() && req.UpdatedDate.Val != "",
		"source":          req.Source.ok() && req.Source.Val != "",
		"source-id":       req.SourceID.ok() && req.SourceID.Val != "",
		"game-track-list": req.GameTrackIDList.ok(),
	} {
		if !ok {
			return field
		}
	}
	return ""
}

// returns the catalogue in the file contents `b`, tagged with `cat_loc`.
// the catalogue must be a v2 catalogue whose total matches its entries, otherwise an
// error is returned. a bad entry is discarded, with an issue, leaving the rest. entries
// from hosts that no longer exist are discarded, unknown game tracks are removed from an
// entry, and descriptions are truncated.
// the fields the strongbox catalogue builder may omit (`description`, `created-date`,
// `download-count`, `tag-list`) are optional.
// clj: `catalogue.clj/read-catalogue`, `validate`
func parse_catalogue(b []byte, cat_loc CatalogueLocation) (Catalogue, []Issue, error) {
	raw := struct {
		Spec             CatalogueSpec     `json:"spec"`
		Datestamp        string            `json:"datestamp"`
		Total            *int              `json:"total"`
		AddonSummaryList []json.RawMessage `json:"addon-summary-list"`
	}{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Catalogue{}, nil, fmt.Errorf("catalogue is not valid: %w", err)
	}
	if raw.Spec.Version != CATALOGUE_VERSION {
		return Catalogue{}, nil, fmt.Errorf("catalogue version %d is not supported", raw.Spec.Version)
	}
	if raw.Total == nil || *raw.Total != len(raw.AddonSummaryList) {
		return Catalogue{}, nil, fmt.Errorf("catalogue total does not match its %d entries", len(raw.AddonSummaryList))
	}
	if _, err := time.Parse("2006-01-02", raw.Datestamp); err != nil {
		return Catalogue{}, nil, fmt.Errorf("catalogue datestamp is not a date: %q", raw.Datestamp)
	}

	issue_list := []Issue{}
	seen := map[string]bool{}
	addon_list := make([]CatalogueAddon, 0, len(raw.AddonSummaryList))
	for i, entry := range raw.AddonSummaryList {
		req := catalogue_addon_required{}
		if err := json.Unmarshal(entry, &req); err != nil {
			issue_list = append(issue_list, debug_issue("discarding catalogue entry %d, it is not an object", i+1))
			continue
		}
		if field := missing_catalogue_field(req); field != "" {
			issue_list = append(issue_list, debug_issue("discarding catalogue entry %d, it has no '%s'", i+1, field))
			continue
		}
		ca := CatalogueAddon{}
		if err := json.Unmarshal(entry, &ca); err != nil {
			issue_list = append(issue_list, debug_issue("discarding catalogue entry %d: %s", i+1, err))
			continue
		}
		if DISABLED_HOSTS.Contains(ca.Source) {
			continue
		}
		if seen[ca.Key()] {
			issue_list = append(issue_list, debug_issue("discarding duplicate catalogue entry: %s", ca.Key()))
			continue
		}
		seen[ca.Key()] = true

		game_track_list := []GameTrackID{}
		for _, gt := range ca.GameTrackIDList {
			if SUPPORTED_GAME_TRACKS.Contains(gt) {
				game_track_list = append(game_track_list, gt)
			} else {
				issue_list = append(issue_list, debug_issue("removing unknown game track '%s' from catalogue entry: %s", gt, ca.Key()))
			}
		}
		ca.GameTrackIDList = game_track_list
		if ca.TagList == nil {
			ca.TagList = []string{}
		}
		if utf8.RuneCountInString(ca.Description) > CATALOGUE_DESCRIPTION_MAX {
			ca.Description = string([]rune(ca.Description)[:CATALOGUE_DESCRIPTION_MAX])
		}
		addon_list = append(addon_list, ca)
	}

	return Catalogue{
		CatalogueLocation: cat_loc,
		Spec:              raw.Spec,
		Datestamp:         raw.Datestamp,
		Total:             len(addon_list),
		AddonSummaryList:  addon_list,
	}, issue_list, nil
}

// reads the catalogue at the given `catalogue_path`, tagged with `cat_loc`.
// returns an error when the file is missing, unreadable or not a valid catalogue.
// clj: `catalogue.clj/read-catalogue`
func read_catalogue_file(cat_loc CatalogueLocation, catalogue_path PathToFile) (Catalogue, error) {
	b, err := os.ReadFile(catalogue_path)
	if err != nil {
		return Catalogue{}, fmt.Errorf("failed to read catalogue: %w", err)
	}
	cat, issue_list, err := parse_catalogue(b, cat_loc)
	if err != nil {
		return Catalogue{}, err
	}
	log_issues(issue_list, catalogue_path)
	return cat, nil
}

// returns the catalogue that is listed and matched against: every entry of `selected`,
// plus the entries of `user` that `selected` lacks.
// an entry in both comes from `selected` whole. no entry ever mixes fields from both.
// entries in `user` are starred.
// a map keyed by source and source ID, so the combination is one pass over each list.
func combine_catalogues(selected Catalogue, user Catalogue) Catalogue {
	user_keys := map[string]bool{}
	for _, ca := range user.AddonSummaryList {
		user_keys[ca.Key()] = true
	}
	combined := selected
	seen := map[string]bool{}
	combined.AddonSummaryList = make([]CatalogueAddon, 0, len(selected.AddonSummaryList)+len(user.AddonSummaryList))
	for _, ca := range selected.AddonSummaryList {
		ca.starred = user_keys[ca.Key()]
		seen[ca.Key()] = true
		combined.AddonSummaryList = append(combined.AddonSummaryList, ca)
	}
	for _, ca := range user.AddonSummaryList {
		if seen[ca.Key()] {
			continue
		}
		ca.starred = true
		combined.AddonSummaryList = append(combined.AddonSummaryList, ca)
	}
	combined.Total = len(combined.AddonSummaryList)
	return combined
}

// returns all `CatalogueLocation` items in app state as a map keyed by catalogue name.
func catalogue_loc_map(app *core.App) map[string]CatalogueLocation {
	idx := map[string]CatalogueLocation{}
	for _, result := range app.GetResultList() {
		if result.NS == NS_CATALOGUE_LOC {
			idx[result.Item.(CatalogueLocation).Name] = result.Item.(CatalogueLocation)
		}
	}
	return idx
}

// returns the selected catalogue location, falling back to the first catalogue location
// when the selection is unknown.
// returns an error only when there are no catalogue locations at all.
// clj: `core.clj/current-catalogue`
func current_catalogue_location(app *core.App) (CatalogueLocation, error) {
	settings := FindSettings(app)
	for _, cl := range settings.CatalogueLocationList {
		if cl.Name == settings.Preferences.SelectedCatalogue {
			return cl, nil
		}
	}
	if len(settings.CatalogueLocationList) > 0 {
		return settings.CatalogueLocationList[0], nil
	}
	return CatalogueLocation{}, errors.New("no catalogues available")
}

// returns `true` when the file at `path` is missing, or was written `max_age` or more
// before `now`.
func file_stale(path string, max_age time.Duration, now time.Time) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return true
	}
	return now.Sub(stat.ModTime()) >= max_age
}

// downloads the catalogue `cat_loc` to `local_path` when the local copy is missing or
// stale, or always when `force`.
// a failed download leaves any local copy as it was and is logged at WARN.
// returns an error when the download failed.
// clj: `core.clj/download-catalogue`
func download_catalogue(app *core.App, cat_loc CatalogueLocation, local_path PathToFile, force bool, now time.Time) error {
	if !force && !file_stale(local_path, CATALOGUE_MAX_AGE, now) {
		slog.Debug("catalogue is fresh, not downloading", "catalogue", local_path)
		return nil
	}
	if err := core.MakeParents(local_path); err != nil {
		return err
	}
	slog.Info("downloading catalogue", "catalogue", cat_loc.Name)
	err := app.DownloadFile(cat_loc.Source, local_path)
	if err != nil {
		slog.Warn("failed to download catalogue", "catalogue", cat_loc.Name, "url", cat_loc.Source, "error", err)
		return err
	}
	return nil
}

// returns the catalogue `cat_loc`, downloading it when the local copy is missing or stale.
// a local copy that cannot be read is deleted and downloaded once more.
// returns an error when no usable catalogue can be had.
// clj: `core.clj/load-current-catalogue`
func fetch_catalogue(app *core.App, cat_loc CatalogueLocation, now time.Time) (Catalogue, error) {
	local_path := CataloguePath(app, cat_loc.Name)
	download_catalogue(app, cat_loc, local_path, false, now) // a failure keeps the old copy, if any

	cat, err := read_catalogue_file(cat_loc, local_path)
	if err == nil {
		return cat, nil
	}
	if !core.FileExists(local_path) {
		return Catalogue{}, fmt.Errorf("no catalogue available: %w", err)
	}

	slog.Warn("local catalogue cannot be read, downloading it again", "catalogue", local_path, "error", err)
	os.Remove(local_path)
	if derr := download_catalogue(app, cat_loc, local_path, true, now); derr != nil {
		return Catalogue{}, fmt.Errorf("failed to download catalogue again: %w", derr)
	}
	cat, err = read_catalogue_file(cat_loc, local_path)
	if err != nil {
		slog.Error("downloaded catalogue cannot be read either, no catalogue is loaded", "catalogue", local_path, "error", err)
		return Catalogue{}, err
	}
	return cat, nil
}

// returns `state` with the result `root` and its descendents replacing any existing
// result with the same ID and that result's descendents.
func replace_subtree(old_state core.State, root core.Result) core.State {
	old_list := old_state.GetResults()
	stale := descendent_ids(old_list, root.ID)
	stale.Add(root.ID)
	new_list := make([]core.Result, 0, len(old_list))
	for _, r := range old_list {
		if !stale.Contains(r.ID) {
			new_list = append(new_list, r)
		}
	}
	old_state.Root.Item = append(new_list, root)
	return old_state
}

// returns `true` when a catalogue has been loaded into state.
// clj: `core.clj/db-catalogue-loaded?`
func db_catalogue_loaded(app *core.App) bool {
	return app.HasResult(ID_CATALOGUE)
}

// returns `true` when a catalogue is loaded but holds no addons.
// returns an error when no catalogue is loaded: an unloaded catalogue has no length to
// report, which is distinct from a loaded one holding zero addons.
func db_catalogue_empty(app *core.App) (bool, error) {
	res := app.GetResult(ID_CATALOGUE)
	if res == nil {
		return false, errors.New("catalogue not loaded")
	}
	cat, is_cat := res.Item.(Catalogue)
	if !is_cat {
		return false, fmt.Errorf("result with the catalogue ID is not a catalogue: %T", res.Item)
	}
	return len(cat.AddonSummaryList) == 0, nil
}

// returns the catalogue in state, combined with the user catalogue, or `false` when none
// is loaded.
func loaded_catalogue(app *core.App) (Catalogue, bool) {
	r := app.GetResult(ID_CATALOGUE)
	if r == nil {
		return Catalogue{}, false
	}
	cat, ok := r.Item.(Catalogue)
	return cat, ok
}

// returns the user catalogue in state, or an empty one.
func loaded_user_catalogue(app *core.App) Catalogue {
	r := app.GetResult(ID_USER_CATALOGUE)
	if r == nil {
		return Catalogue{}
	}
	uc, ok := r.Item.(UserCatalogue)
	if !ok {
		return Catalogue{}
	}
	return uc.Catalogue
}

// returns the selected catalogue in state, before it was combined, or an empty one.
func loaded_selected_catalogue(app *core.App) Catalogue {
	r := app.GetResult(ID_SELECTED_CATALOGUE)
	if r == nil {
		return Catalogue{}
	}
	sc, ok := r.Item.(SelectedCatalogue)
	if !ok {
		return Catalogue{}
	}
	return sc.Catalogue
}

// puts `selected` combined with `user` into state as the catalogue, plus each of them on
// its own, replacing what was there in one state update.
func catalogue_to_state(app *core.App, selected Catalogue, user Catalogue) {
	combined := combine_catalogues(selected, user)
	app.UpdateState(func(old_state core.State) core.State {
		old_state = replace_subtree(old_state, core.MakeResult(NS_CATALOGUE_USER, UserCatalogue{user}, ID_USER_CATALOGUE))
		old_state = replace_subtree(old_state, core.MakeResult(NS_CATALOGUE_SELECTED, SelectedCatalogue{selected}, ID_SELECTED_CATALOGUE))
		return replace_subtree(old_state, core.MakeResult(NS_CATALOGUE, combined, ID_CATALOGUE))
	}).Wait()
}

// loads the selected catalogue, downloading it when stale, and the user catalogue, into
// state.
// the user catalogue is loaded even when the selected catalogue cannot be, so its addons
// can still be found and matched.
// returns an error when the selected catalogue could not be loaded.
// clj: `core.clj/db-load-catalogue`
func LoadCatalogue(app *core.App) error {
	user := read_user_catalogue(app.State().GetKeyVal("strongbox.paths.user-catalogue-file"))
	cat_loc, err := current_catalogue_location(app)
	if err != nil {
		catalogue_to_state(app, Catalogue{}, user)
		return err
	}
	selected, err := fetch_catalogue(app, cat_loc, time.Now())
	if err != nil {
		catalogue_to_state(app, Catalogue{CatalogueLocation: cat_loc}, user)
		return err
	}
	catalogue_to_state(app, selected, user)
	return nil
}

// selects the catalogue named `name`, saves the settings and loads it.
// returns an error when there is no such catalogue location.
func SwitchCatalogue(app *core.App, name string) error {
	settings := FindSettings(app)
	known := false
	for _, cl := range settings.CatalogueLocationList {
		known = known || cl.Name == name
	}
	if !known {
		return fmt.Errorf("unknown catalogue: %s", name)
	}
	apply_settings(app, func(s Settings) Settings {
		s.Preferences.SelectedCatalogue = name
		return s
	})
	return LoadCatalogue(app)
}

// --- user catalogue

// how old the user catalogue may get before a scheduled refresh updates it.
const USER_CATALOGUE_MAX_AGE = 28 * 24 * time.Hour

// returns the user catalogue at `path`, or an empty one when it is missing.
// an unreadable user catalogue is logged at WARN and read as empty. it is preserved
// before anything is written over it, see `write_user_catalogue`.
// clj: `core.clj/get-user-catalogue`
func read_user_catalogue(path PathToFile) Catalogue {
	empty := Catalogue{Spec: CatalogueSpec{Version: CATALOGUE_VERSION}, AddonSummaryList: []CatalogueAddon{}}
	if path == "" || !core.FileExists(path) {
		return empty
	}
	cat, err := read_catalogue_file(CatalogueLocation{Name: "user", Label: "User catalogue"}, path)
	if err != nil {
		slog.Warn("user catalogue cannot be read, it will be preserved when next changed", "user-catalogue", path, "error", err)
		return empty
	}
	return cat
}

// writes `cat` to `path` as the user catalogue, dated `now`, atomically.
// an existing file that cannot be read is first copied aside so it is never lost.
// clj: `core.clj/write-user-catalogue!`
func write_user_catalogue(path PathToFile, cat Catalogue, now time.Time) error {
	if core.FileExists(path) {
		if b, err := os.ReadFile(path); err == nil {
			if _, _, perr := parse_catalogue(b, CatalogueLocation{}); perr != nil {
				copy_path := invalid_copy_path(path, now)
				if werr := os.WriteFile(copy_path, b, 0o600); werr != nil {
					return fmt.Errorf("refusing to overwrite unreadable user catalogue that could not be preserved: %w", werr)
				}
				slog.Warn("preserved unreadable user catalogue", "user-catalogue", path, "preserved-as", copy_path)
			}
		}
	}
	out := struct {
		Spec             CatalogueSpec    `json:"spec"`
		Datestamp        string           `json:"datestamp"`
		Total            int              `json:"total"`
		AddonSummaryList []CatalogueAddon `json:"addon-summary-list"`
	}{
		Spec:             CatalogueSpec{Version: CATALOGUE_VERSION},
		Datestamp:        now.UTC().Format("2006-01-02"),
		Total:            len(cat.AddonSummaryList),
		AddonSummaryList: cat.AddonSummaryList,
	}
	if out.AddonSummaryList == nil {
		out.AddonSummaryList = []CatalogueAddon{}
	}
	b, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	return write_atomic(path, b)
}

// returns `cat` with `ca` added, replacing any entry with the same key.
// clj: `core.clj/add-user-addon!`
func user_catalogue_add(cat Catalogue, ca CatalogueAddon) Catalogue {
	ca.starred = false
	list := slices.DeleteFunc(slices.Clone(cat.AddonSummaryList), func(x CatalogueAddon) bool { return x.Key() == ca.Key() })
	cat.AddonSummaryList = append(list, ca)
	cat.Total = len(cat.AddonSummaryList)
	return cat
}

// returns `cat` without the entry with key `key`.
// clj: `core.clj/remove-user-addon!`
func user_catalogue_remove(cat Catalogue, key string) Catalogue {
	cat.AddonSummaryList = slices.DeleteFunc(slices.Clone(cat.AddonSummaryList), func(x CatalogueAddon) bool { return x.Key() == key })
	cat.Total = len(cat.AddonSummaryList)
	return cat
}

// changes the user catalogue with `fn`, writes it and puts it into state alongside the
// selected catalogue.
func update_user_catalogue(app *core.App, fn func(Catalogue) Catalogue) error {
	path := app.State().GetKeyVal("strongbox.paths.user-catalogue-file")
	user := fn(read_user_catalogue(path))
	if err := write_user_catalogue(path, user, time.Now()); err != nil {
		return fmt.Errorf("failed to write user catalogue: %w", err)
	}
	catalogue_to_state(app, loaded_selected_catalogue(app), user)
	return nil
}

// adds `ca` to the user catalogue.
// clj: `cli.clj/add-summary-to-user-catalogue`
func StarCatalogueAddon(app *core.App, ca CatalogueAddon) error {
	return update_user_catalogue(app, func(cat Catalogue) Catalogue { return user_catalogue_add(cat, ca) })
}

// removes `ca` from the user catalogue.
// clj: `cli.clj/remove-summary-from-user-catalogue`
func UnstarCatalogueAddon(app *core.App, ca CatalogueAddon) error {
	return update_user_catalogue(app, func(cat Catalogue) Catalogue { return user_catalogue_remove(cat, ca.Key()) })
}

// returns the user catalogue with every entry refreshed: replaced by the same addon from
// `full`, by source and source ID, or failing that found again with `find_addon`.
// an entry that cannot be found is kept as it is. a failure for one entry does not stop
// the others.
// clj: `core.clj/refresh-user-catalogue`, `refresh-user-catalogue-item`
func refresh_user_catalogue(user Catalogue, full Catalogue, find_addon func(CatalogueAddon) (CatalogueAddon, error)) Catalogue {
	full_idx := map[string]CatalogueAddon{} // key => catalogue addon
	for _, ca := range full.AddonSummaryList {
		full_idx[ca.Key()] = ca
	}
	refreshed := user
	refreshed.AddonSummaryList = make([]CatalogueAddon, 0, len(user.AddonSummaryList))
	for _, ca := range user.AddonSummaryList {
		if newer, present := full_idx[ca.Key()]; present {
			refreshed.AddonSummaryList = append(refreshed.AddonSummaryList, newer)
			continue
		}
		found, err := find_addon(ca)
		if err != nil {
			slog.Warn("failed to refresh user catalogue entry, keeping it", "addon", ca.Label, "error", err)
			refreshed.AddonSummaryList = append(refreshed.AddonSummaryList, ca)
			continue
		}
		refreshed.AddonSummaryList = append(refreshed.AddonSummaryList, found)
	}
	refreshed.Total = len(refreshed.AddonSummaryList)
	return refreshed
}

// returns `true` when the user catalogue is due a scheduled refresh: the preference is on
// and it was last written `USER_CATALOGUE_MAX_AGE` or more before `now`.
// a user catalogue with no addons is never due.
// clj: `core.clj/scheduled-user-catalogue-refresh`
func user_catalogue_refresh_due(user Catalogue, keep_updated bool, now time.Time) bool {
	if !keep_updated || len(user.AddonSummaryList) == 0 {
		return false
	}
	dt, err := time.Parse("2006-01-02", user.Datestamp)
	if err != nil {
		return true
	}
	return now.Sub(dt) > USER_CATALOGUE_MAX_AGE
}

// refreshes every addon in the user catalogue from the full catalogue, or failing that
// from its host, then writes it once and puts it into state.
// a full catalogue that cannot be had is not an error: every entry is then looked up at
// its host.
// clj: `core.clj/refresh-user-catalogue`
func RefreshUserCatalogue(app *core.App) error {
	path := app.State().GetKeyVal("strongbox.paths.user-catalogue-file")
	user := read_user_catalogue(path)
	if len(user.AddonSummaryList) == 0 {
		return nil
	}

	full_loc := CAT_FULL
	for _, cl := range FindSettings(app).CatalogueLocationList {
		if cl.Name == CAT_FULL.Name {
			full_loc = cl
		}
	}
	full, err := fetch_catalogue(app, full_loc, time.Now())
	if err != nil {
		slog.Warn("full catalogue unavailable, refreshing the user catalogue from each addon's host", "error", err)
	}

	find_addon := func(ca CatalogueAddon) (CatalogueAddon, error) {
		host, err := addon_source(ca.Source)
		if err != nil {
			return CatalogueAddon{}, err
		}
		return host.FindAddon(app, string(ca.SourceID))
	}

	job := app.StartJob("refreshing user catalogue", 0)
	defer job.Finish()
	refreshed := refresh_user_catalogue(user, full, find_addon)
	if err := write_user_catalogue(path, refreshed, time.Now()); err != nil {
		return fmt.Errorf("failed to write user catalogue: %w", err)
	}
	catalogue_to_state(app, loaded_selected_catalogue(app), refreshed)
	slog.Info("user catalogue refreshed", "num-addons", len(refreshed.AddonSummaryList))
	return nil
}

// returns `true` when the search tab row `row` matches the search text `input`: its label,
// name or description contains it, compared case-insensitively. empty search text matches
// every row.
func CatalogueSearchFilter(input string, row map[string]string) bool {
	needle := strings.ToLower(strings.TrimSpace(input))
	if needle == "" {
		return true
	}
	for _, key := range []string{core.ITEM_FIELD_NAME, "normalised-name", core.ITEM_FIELD_DESC} {
		if strings.Contains(strings.ToLower(row[key]), needle) {
			return true
		}
	}
	return false
}
