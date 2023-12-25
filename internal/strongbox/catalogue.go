package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
)

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
