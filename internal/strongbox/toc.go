package strongbox

import (
	"bw/internal/core"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
)

// "(?u) match the remainder of the pattern with the following effective flags: gmiu"
var game_track_regex = regexp.MustCompile(`^(?i)(.+?)(?:[\-_]{1}(Mainline|Classic|Vanilla|TBC|BCC|Wrath){1})?\.toc$`)

// "returns a list of TOC structs at the given `addon_path`.
func find_toc_files(addon_path string) ([]TOC, error) {
	toc_data := []TOC{}

	file_list, err := core.ListFiles(addon_path)
	if err != nil {
		return []TOC{}, err
	}

	addon_dir := filepath.Base(addon_path)

	for _, file_path := range file_list {
		file_name := filepath.Base(file_path)

		// todo: remove the dirname from the toc file as it may interfere with the regex.
		// for example: 'LittleWigs_Classic' *could* have (but doesn't) 'LittleWigs_Classic_Classic.toc'.
		// it does 'LittleWigs_Classic_Vanilla.toc' instead
		// then, account for misnamed toc files ...

		// ["AdiBags.toc","AdiBags",""]
		// ["AdiBags_TBC.toc","AdiBags","TBC"]
		// ["AdiBags_Vanilla.toc","AdiBags","Vanilla"]
		matches := game_track_regex.FindStringSubmatch(file_name)
		if len(matches) < 2 {
			continue
		}

		var game_track_id GameTrackID
		if matches[2] == "" {
			// ["AdiBags.toc","AdiBags",""]}
			game_track_id = GAMETRACK_RETAIL
		} else {
			game_track_id, err = GuessGameTrack(matches[2])
			if err != nil {
				slog.Warn(err.Error() + ", assuming 'retail'")
				game_track_id = GAMETRACK_RETAIL
			}
		}

		toc := TOC{
			FileName:            file_name,     // "AdiBags_TBC.toc"
			DirName:             addon_dir,     // "AdiBags" in /path/to/addon/dir/Adibags/..."
			FileNameGameTrackID: game_track_id, // "classic-tbc" guessed from "AdiBags_TBC.toc"
		}
		toc_data = append(toc_data, toc)

	}
	return toc_data, nil
}

// toc.clj/parse-toc-file
// parses the contents of .toc file into the given `toc` struct.
// optimisation: read the .toc file header until comments end, parsing each line as we go
func parse_addon_toc_contents(toc_contents string) map[string]string {
	is_comment := func(row string) bool {
		return strings.HasPrefix(row, "##")
	}
	is_comment_comment := func(row string) bool {
		return strings.HasPrefix(row, "# ##")
	}
	is_interesting := func(row string) bool {
		return is_comment(row) || is_comment_comment(row)
	}

	parse_row := func(row string) (string, string, error) {
		bits := strings.SplitN(row, ":", 2) // "##Interface: 70300" => ["##Interface" " 70300"]
		//key, val := bits[0], bits[1]
		key := bits[0]
		if len(bits) == 1 {
			return "", "", fmt.Errorf("row has no value: %s", row)
		}

		val := bits[1]
		val = strings.TrimSpace(val)
		if val == "" {
			return "", "", fmt.Errorf("row has no value: %s", row)
		}

		if is_comment_comment(row) {
			// handles "# ##Interface" as well as "# ## Interface"
			key = "#" + strings.ToLower(strings.TrimLeft(key, "# ")) // "# ## Title" => "#title"
		} else {
			// handles "##Interface" as well as "## Interface"
			key = strings.ToLower(strings.TrimLeft(key, "# ")) // "# ## Title" => "title"
		}

		return key, val, nil
	}

	row_list := strings.Split(strings.ReplaceAll(toc_contents, "\r\n", "\n"), "\n")
	key_vals := map[string]string{}
	for _, row := range row_list {
		if is_interesting(row) {
			key, val, err := parse_row(row)
			if err != nil {
				//slog.Warn("skipping row", "error", err)
				continue
			}
			//fmt.Printf("%s: %s\n", key, val)
			key_vals[key] = val
		}
	}

	return key_vals
}

// toc.clj/parse-addon-toc
func populate_toc(kvs map[string]string, toc TOC) TOC {
	title, has_title := kvs["title"]
	if !has_title {
		slog.Warn("addon with no 'Title' value found", "dir-name", toc.DirName, "toc-file", toc.FileName, "kvs", kvs)
		title = toc.DirName + " *" // "EveryAddon *"
	}
	toc.Title = title

	source_map_list := []SourceMap{}
	x_wowi_id, has_x_wowi_id := kvs["x-wowi-id"]
	if has_x_wowi_id {
		wowi_source := SourceMap{Source: SOURCE_WOWI, SourceID: x_wowi_id}
		source_map_list = append(source_map_list, wowi_source)
	}

	toc.SourceMapList = source_map_list

	return toc
}

// --- public

// reads the contents of a *single* .toc file, returning a map of key+vals
func ReadAddonTocFile(addon_path string) (map[string]string, error) {
	empty_map := map[string]string{}
	toc_data, err := core.SlurpBytesUTF8(addon_path)
	if err != nil {
		return empty_map, err
	}
	return parse_addon_toc_contents(string(toc_data)), nil
}

// "wraps the `parse-addon-toc` function, attaching the list of `:supported-game-tracks` and sinking any errors."
func ParseAllAddonTocFiles(addon_path string) ([]TOC, error) {

	toc_list, err := find_toc_files(addon_path)
	if err != nil {
		return []TOC{}, err
	}

	populated_toc_list := []TOC{}

	for _, toc := range toc_list {
		keyvals_map, err := ReadAddonTocFile(filepath.Join(addon_path, toc.FileName))
		if err != nil {
			slog.Warn("error reading contents of toc file, skipping", "dir-name", toc.DirName, "file-name", toc.FileName, "error", err)
			continue
		}
		populated_toc := populate_toc(keyvals_map, toc)
		populated_toc_list = append(populated_toc_list, populated_toc)
	}

	return populated_toc_list, nil
}
