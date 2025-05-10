package strongbox

import (
	"bw/core"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// --- Catalogue Addon

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

func (ca CatalogueAddon) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_URL,
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		"source",
		core.ITEM_FIELD_DATE_UPDATED,
		"downloads",
		"tags",
	}
}

func (ca CatalogueAddon) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_URL:          ca.URL,
		core.ITEM_FIELD_NAME:         ca.Label,
		core.ITEM_FIELD_DESC:         ca.Description,
		"source":                     ca.Source,
		core.ITEM_FIELD_DATE_UPDATED: ca.UpdatedDate,
		"downloads":                  strconv.Itoa(ca.DownloadCount),
		"tags":                       strings.Join(ca.TagList, ", "),
	}
}

func (ca CatalogueAddon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (ca CatalogueAddon) ItemChildren(app *core.App) []core.Result {
	return nil
}

var _ core.ItemInfo = (*CatalogueAddon)(nil)

// --- Catalogue Location

type CatalogueLocation struct {
	Name   string `json:"name"`   // "short"
	Label  string `json:"label"`  // "Short"
	Source string `json:"source"` // "https://someurl.org/path/to/catalogue.json"
}

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

var _ core.ItemInfo = (*CatalogueLocation)(nil)

// --- Catalogue

type CatalogueSpec struct {
	Version int `json:"version"`
}

type Catalogue struct {
	CatalogueLocation
	Spec             CatalogueSpec    `json:"spec"`
	Datestamp        string           `json:"datestamp"` // todo: make this a timestamp
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

func (c Catalogue) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_LAZY
}

func (c Catalogue) ItemChildren(app *core.App) []core.Result {
	empty_result_list := []core.Result{}

	catalogue, err := _db_load_catalogue(app) // TODO: this isn't right.
	if err != nil {
		slog.Warn("failed to load catalogue, cannot expand Catalogue", "error", err)
		return empty_result_list
	}

	// wrap each CatalogueAddon in a core.Result
	result_list := []core.Result{}
	for _, addon := range catalogue.AddonSummaryList {
		id := core.UniqueID()
		result_list = append(result_list, core.MakeResult(NS_CATALOGUE_ADDON, addon, id))
	}
	return result_list
}

var _ core.ItemInfo = (*Catalogue)(nil)

// ---

func catalogue_local_path(data_dir string, filename string) string {
	return filepath.Join(data_dir, filename)
}

func catalogue_path(app *core.App, catalogue_name string) string {
	val := app.State.KeyAnyVal("strongbox.paths.catalogue-dir")
	if val == nil {
		panic("attempted to access strongbox.paths.catalogue-dir before it was present")
	}
	return catalogue_local_path(val.(string), catalogue_name)
}

// catalogue.clj/read-catalogue
// reads the catalogue of addon data at the given `catalogue-path`.
func read_catalogue_file(cat_loc CatalogueLocation, catalogue_path PathToFile) (Catalogue, error) {
	empty_catalogue := Catalogue{CatalogueLocation: cat_loc}
	if !core.FileExists(catalogue_path) {
		return empty_catalogue, fmt.Errorf("no catalogue at given path: %s", catalogue_path)
	}
	b, err := os.ReadFile(catalogue_path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("error reading contents of file: %w", err)
	}
	cat := Catalogue{CatalogueLocation: cat_loc}
	err = json.Unmarshal(b, &cat)
	if err != nil {
		return empty_catalogue, fmt.Errorf("error deserialising catalogue contents: %w", err)
	}
	return cat, nil
}

// returns all `CatalogueLocation` items in app state as a map keyed by catalogue name.
func catalogue_loc_map(app *core.App) map[string]CatalogueLocation {
	idx := map[string]CatalogueLocation{}
	for _, result := range app.GetResultList() {
		if result.NS == NS_CATALOGUE_LOC {
			// {"short" => CatalogueLocation{...}, ...}
			idx[result.Item.(CatalogueLocation).Name] = result.Item.(CatalogueLocation)
		}
	}
	return idx
}

// returns the currently selected `CatalogueLocation` found in the settings
func find_selected_catalogue(app *core.App) (CatalogueLocation, error) {
	selected_catalogue_name := FindSettings(app).Preferences.SelectedCatalogue

	empty_result := CatalogueLocation{}
	idx := catalogue_loc_map(app) // {"short": CatalogueLocation{...}, ...}
	catalogue_loc, present := idx[selected_catalogue_name]
	if !present {
		slog.Error("selected catalogue not found in known catalogues", "selected-catalogue", selected_catalogue_name, "known-catalogue-list", idx)
		return empty_result, errors.New("selected catalogue not available in list of known catalogues")
	}

	return catalogue_loc, nil
}

// core.clj/default-catalogue
// the 'default' catalogue is the first catalogue in the list of available catalogues.
// using the original set of catalogues that come with strongbox, this is the 'short' catalogue,
// however the user can specify their own catalogues so this isn't guaranteed.
// returns an error if no catalogues available.
func default_catalogue(app *core.App) (CatalogueLocation, error) {
	empty_cat_loc := CatalogueLocation{}
	cat_loc_list := app.FilterResultListByNS(NS_CATALOGUE_LOC)
	if len(cat_loc_list) < 1 {
		return empty_cat_loc, errors.New("cannot select a default catalogue, no catalogues available")
	}
	return cat_loc_list[0].Item.(CatalogueLocation), nil
}

// core.clj/get-catalogue-location, second arity
// returns the `CatalogueLocation` for the given `cat_loc_name`.
// returns and error if catalogue location not found or no catalogue location selected.
/*
func get_catalogue_location(app *core.App, cat_loc_name string) (CatalogueLocation, error) {
	empty_cat_loc := CatalogueLocation{}
	idx := catalogue_loc_map(app)
	cat_loc, is_present := idx[cat_loc_name]
	if !is_present {
		return empty_cat_loc, fmt.Errorf("catalogue '%s' not present in index", cat_loc_name)
	}
	return cat_loc, nil
}
*/

// core.clj/current-catalogue
// returns the currently selected `CatalogueLocation` or the first one it can find.
func current_catalogue_location(app *core.App) (CatalogueLocation, error) {
	cat_loc, err := find_selected_catalogue(app)
	if err != nil {
		cat_loc, err = default_catalogue(app)
		if err != nil {
			// no catalogue selected, no default catalogue available, cannot contine
			return CatalogueLocation{}, err
		}
	}
	return cat_loc, nil
}

// todo: needs to be a task that can be cancelled and cleaned up
// core.clj/download-catalogue
// downloads catalogue to expected location, nothing more
func download_catalogue(app *core.App, catalogue_loc CatalogueLocation, data_dir PathToDir) error {
	remote_catalogue := catalogue_loc.Source
	local_catalogue := catalogue_local_path(data_dir, catalogue_loc.Name)
	if core.FileExists(local_catalogue) {
		// todo: freshness check
		slog.Debug("catalogue exists, not downloading")
		return nil
	}
	err := core.DownloadFile(app, remote_catalogue, local_catalogue)
	if err != nil {
		slog.Error("failed to download catalogue", "remote-catalogue", remote_catalogue, "local-catalogue", local_catalogue, "error", err)
		return err
	}
	return nil
}

// core.clj/download-current-catalogue
// "downloads the currently selected (or default) catalogue."
func DownloadCurrentCatalogue(app *core.App) {
	catalogue_loc, err := current_catalogue_location(app)
	if err != nil {
		slog.Warn("failed to find a downloadable catalogue", "error", err)
		return
	}

	catalogue_dir := app.State.KeyVal("strongbox.paths.catalogue-dir")
	if catalogue_dir == "" {
		slog.Warn("'catalogue-dir' location not found, cannot download catalogue")
		return
	}

	_ = download_catalogue(app, catalogue_loc, catalogue_dir)

}

// core.clj/db-catalogue-loaded?
// returns `true` if the database has a catalogue loaded.
// A database may be `nil` if it simply hasn't been loaded yet or we attempted to load it and it failed to load.
// A database may fail to load if it simply isn't there, can't be downloaded or, once downloaded, the data is invalid.
// An empty database (`Catalogue{}`) is distinct from an unloaded database (nil pointer), see `db_catalogue_empty`.
func db_catalogue_loaded(app *core.App) bool {
	return app.HasResult(ID_CATALOGUE)
}

// new in v8
// returns `true` if a database has been loaded but the database is empty.
// A database may be empty *only* if the `addon-summary-list` key of a catalogue is empty.
// An empty database (`Catalogue{}`) is distinct from an unloaded database (nil pointer), see `db_catalogue_loaded`.
func db_catalogue_empty(app *core.App) bool {
	res := app.GetResult(ID_CATALOGUE)
	if res == nil {
		return true
	}
	cat, _ := res.Item.(Catalogue)
	return len(cat.AddonSummaryList) > 0
}

func _db_load_catalogue(app *core.App) (Catalogue, error) {

	var empty_catalogue Catalogue

	if db_catalogue_loaded(app) {
		return empty_catalogue, errors.New("catalogue already loaded")
	}

	cat_loc, err := current_catalogue_location(app)
	if err != nil {
		return empty_catalogue, errors.New("no catalogue selected or selectable")
	}

	slog.Info("loading catalogue", "name", cat_loc.Label)
	catalogue_path := catalogue_local_path(app.State.KeyVal("strongbox.paths.catalogue-dir"), cat_loc.Name)

	cat, err := read_catalogue_file(cat_loc, catalogue_path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to read catalogue: %w", err)
	}

	return cat, nil
}

// core.clj/db-load-catalogue
// core.clj/load-current-catalogue
// loads a catalogue from disk, assuming it has already been downloaded.
func DBLoadCatalogue(app *core.App) {
	catalogue, err := _db_load_catalogue(app)
	if err != nil {
		slog.Warn("failed to load catalogue", "error", err)
		return
	}
	wg := app.SetResults(core.MakeResult(NS_CATALOGUE, catalogue, ID_CATALOGUE))
	wg.Wait()

	r := app.GetResult(ID_CATALOGUE)
	if r == nil {
		panic("programming error, catalogue should have loaded") // todo: rm after dev
	}
}

// core.clj/get-user-catalogue
// returns the contents of the user catalogue as a `Catalogue`, removing any disable hosts.
// returns an error when the catalogue is not found,
// or the catalogue cannot be read,
// or the catalogue data is bad json.
func get_user_catalogue(app *core.App) (Catalogue, error) {

	empty_catalogue := Catalogue{}

	path := app.State.KeyVal("strongbox.paths.user-catalogue-file")
	if !core.FileExists(path) {
		return empty_catalogue, errors.New("user-catalogue not found")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to reading user-catalogue-file: %w", err)
	}

	var cat Catalogue
	err = json.Unmarshal(data, &cat)
	if err != nil {
		return empty_catalogue, fmt.Errorf("failed to unmarshal user-catalogue-file: %w", err)
	}

	new_addon_list := []CatalogueAddon{}
	for _, addon := range cat.AddonSummaryList {
		if HostDisabled(addon.Source) {
			continue
		}
		new_addon_list = append(new_addon_list, addon)
	}

	// todo: fix Catalogue.Total
	cat.AddonSummaryList = new_addon_list
	return cat, nil
}

// core.clj/db-load-user-catalogue
// loads the user catalogue into state, but only if it hasn't already been loaded.
func DBLoadUserCatalogue(app *core.App) {
	if app.HasResult(ID_USER_CATALOGUE) {
		return
	}
	user_cat, err := get_user_catalogue(app)
	if err != nil {
		slog.Warn("error loading user catalogue", "error", err)
	}

	// see: core.clj/set-user-catalogue!
	// todo: create an idx
	app.SetResults(core.Result{ID: ID_USER_CATALOGUE, NS: NS_CATALOGUE_USER, Item: user_cat})
}
