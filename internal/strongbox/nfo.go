package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
)

// "given an installation directory and the directory name of an addon, return the absolute path to the nfo file."
func nfo_path(addon_dir PathToAddon) string {
	return filepath.Join(addon_dir, NFO_FILENAME) // "/path/to/addon-dir/Addon/.strongbox.json
}

// "reads the nfo file at the given `path` with basic transformations.
// an error is returned if the data cannot be loaded or the data is invalid.
func read_nfo_file(addon_dir PathToAddon) ([]NFO, error) {

	empty_data := []NFO{}

	path := nfo_path(addon_dir)
	if !core.FileExists(path) {
		return empty_data, nil
	}

	data := NFO{}
	nfo_list := []NFO{}
	data_bytes, err := core.SlurpBytes(path)
	if err != nil {
		return empty_data, err
	}

	err = json.Unmarshal(data_bytes, &data)
	if err != nil {
		err2 := json.Unmarshal(data_bytes, &nfo_list)
		if err2 != nil {
			return empty_data, err2
		}
	} else {
		nfo_list = append(nfo_list, data)
	}

	for _, nfo := range nfo_list {
		nfo := &nfo // todo: necessary?
		if nfo.Source != "" && len(nfo.SourceMapList) == 0 {
			sm := SourceMap{Source: nfo.Source, SourceID: nfo.SourceID}
			nfo.SourceMapList = append(nfo.SourceMapList, sm)
		}
	}

	return nfo_list, nil

}

// --- public

// "parses the contents of the .nfo file and checks if addon should be ignored or not"
// failure to load the json results in the file being deleted.
// failure to validate the json data results in the file being deleted."
func ReadNFO(addon_dir PathToAddon) []NFO {
	nfo_data_list, err := read_nfo_file(addon_dir)
	if err != nil {
		slog.Error("failed to read NFO data", "error", err)
		// delete file if it contains bad data
		// delete file if it contains invalid data
		return []NFO{}
	}
	if len(nfo_data_list) == 0 {
		return nfo_data_list
	}

	// when ignore? present and ignore? is true, debug stmt
	// when ignore? not present and vcs dir present, add ignore? and set to true
	//

	return []NFO{}
}
