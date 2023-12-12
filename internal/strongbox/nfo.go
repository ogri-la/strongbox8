package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"fmt"
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
	data_list := []NFO{}
	data_bytes, err := core.SlurpBytes(path)
	if err != nil {
		return empty_data, err
	}
	//fmt.Println(string(data_bytes))
	err = json.Unmarshal(data_bytes, &data)
	if err != nil {
		err2 := json.Unmarshal(data_bytes, &data_list)
		if err2 != nil {
			fmt.Println(err2.Error())
			return empty_data, err2
		}
	} else {
		data_list = append(data_list, data)
	}

	/*

	   coerce (fn [nfo-data]
	            (if (and (map? nfo-data)
	                     (contains? nfo-data :source)
	                     (not (contains? nfo-data :source-map-list)))
	              (assoc nfo-data :source-map-list (-> nfo-data utils/source-map vector))
	              nfo-data))
	*/

	// attach a source-map-list if one isn't present // seems like it should be
	return data_list, nil

}

// --- public

// "parses the contents of the .nfo file and checks if addon should be ignored or not"
// failure to load the json results in the file being deleted.
// failure to validate the json data results in the file being deleted."
func ReadNFO(addon_dir PathToAddon) []NFO {
	nfo_data_list, err := read_nfo_file(addon_dir)
	if err != nil {
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
