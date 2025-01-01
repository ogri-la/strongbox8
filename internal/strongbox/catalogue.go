package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
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
		"url",
		"name",
		"description",
		"source",
		"id",
		"updated",
		"downloads",
		"tags",
	}
}

func (ca CatalogueAddon) ItemMap() map[string]string {
	return map[string]string{
		"url":         ca.URL,
		"name":        ca.Label,
		"description": ca.Description,
		"source":      ca.Source,
		"id":          string(ca.SourceID),
		"updated":     ca.UpdatedDate,
		"downloads":   strconv.Itoa(ca.DownloadCount),
		"tags":        strings.Join(ca.TagList, ", "),
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

	catalogue := _db_load_catalogue(app)

	// wrap each CatalogueAddon in a core.Result
	result_list := []core.Result{}
	i := 0
	for _, addon := range catalogue.AddonSummaryList {
		id := core.UniqueID()
		result_list = append(result_list, core.NewResult(NS_CATALOGUE_ADDON, addon, id))
		i++
	}
	return result_list
}

var _ core.ItemInfo = (*Catalogue)(nil)

// ---

func catalogue_local_path(data_dir string, filename string) string {
	return filepath.Join(data_dir, filename)
}

func catalogue_path(app *core.App, catalogue_name string) string {
	val := app.KeyAnyVal("strongbox.paths.catalogue-dir")
	if val == nil {
		panic("attempted to access strongbox.paths.catalogue-dir before it was present")
	}
	return catalogue_local_path(val.(string), catalogue_name)
}

// catalogue.clj/read-catalogue
// reads the catalogue of addon data at the given `catalogue-path`.
func ReadCatalogue(cat_loc CatalogueLocation, catalogue_path PathToFile) (Catalogue, error) {
	empty_catalogue := Catalogue{CatalogueLocation: cat_loc}
	if !core.FileExists(catalogue_path) {
		return empty_catalogue, fmt.Errorf("no catalogue at given path: %s", catalogue_path)
	}
	b, err := core.SlurpBytes(catalogue_path)
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
