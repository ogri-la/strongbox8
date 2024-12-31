package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// --- Catalogue

type CatalogueSpec struct {
	Version int `json:"version"`
}

type Catalogue struct {
	Spec             CatalogueSpec    `json:"spec"`
	Datestamp        string           `json:"datestamp"`
	Total            int              `json:"total"`
	AddonSummaryList []CatalogueAddon `json:"addon-summary-list"`
}

// --- Catalogue Addon
// data parsed from strongbox catalogues

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

func catalogue_local_path(data_dir string, filename string) string {
	return filepath.Join(data_dir, filename)
}

func CataloguePath(app *core.App, catalogue_name string) string {
	return catalogue_local_path(app.KeyVal("strongbox.paths.catalogue-dir"), catalogue_name)
}

// catalogue.clj/read-catalogue
// reads the catalogue of addon data at the given `catalogue-path`.
func ReadCatalogue(catalogue_path PathToFile) (Catalogue, error) {
	empty_catalogue := Catalogue{}
	if !core.FileExists(catalogue_path) {
		return empty_catalogue, fmt.Errorf("no catalogue at given path: %s", catalogue_path)
	}
	b, err := core.SlurpBytes(catalogue_path)
	if err != nil {
		return empty_catalogue, fmt.Errorf("error reading contents of file: %w", err)
	}
	var cat Catalogue
	err = json.Unmarshal(b, &cat)
	if err != nil {
		return empty_catalogue, fmt.Errorf("error deserialising catalogue contents: %w", err)
	}
	return cat, nil
}
