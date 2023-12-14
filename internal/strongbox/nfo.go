package strongbox

import (
	"bw/internal/core"
	"encoding/json"
	"log/slog"
	"path/filepath"
)

// "given an installation directory and the directory name of an addon, return the absolute path to the nfo file."
func nfo_path(addon_dir PathToAddon) string {
	return filepath.Join(addon_dir, NFO_FILENAME) // "/path/to/addon-dir/Addon/.strongbox.json
}

// returns the VCS directory found if given path contains a VCS directory,
// otherwise an empty string.
func version_control(addon_dir PathToAddon) (string, error) {
	dir_list, err := core.DirList(addon_dir)
	if err != nil {
		return "", err
	}
	ignorable_dir_set := map[string]bool{
		".git": true,
		".svn": true,
		".hg":  true,
	}
	for _, dir := range dir_list {
		_, present := ignorable_dir_set[dir]
		if present {
			return dir, nil
		}
	}
	return "", nil
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

		// add a SourceMapList if one isn't present
		// new in v8: previously only applied to top-level nfo
		if nfo.Source != "" && len(nfo.SourceMapList) == 0 {
			sm := SourceMap{Source: nfo.Source, SourceID: nfo.SourceID}
			nfo.SourceMapList = append(nfo.SourceMapList, sm)
		}

		// implicitly ignore addon when VCS directory present
		vcs, err := version_control(addon_dir)
		if err != nil {
			slog.Error("error checking addon directory for presence of a VCS", "addon-dir", addon_dir, "error", err)
		}
		if nfo.Ignored != nil && err == nil && vcs == "" {
			slog.Warn("addon directory contains a .git/.hg/.svn folder, ignoring", "addon-dir", addon_dir, "vcs", vcs)
			ignored := true
			nfo.Ignored = &ignored
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
		// todo: delete file if it contains bad data
		// todo: delete file if it contains invalid data
		return []NFO{}
	}

	if len(nfo_data_list) == 0 {
		return nfo_data_list
	}

	return []NFO{}
}

func NFOIgnored(nfo NFO) bool {
	if nfo.Ignored == nil {
		return false
	}
	return *nfo.Ignored
}
