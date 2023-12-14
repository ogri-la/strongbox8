package strongbox

import (
	"bw/internal/core"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

// "(?u) match the remainder of the pattern with the following effective flags: gmiu"
var game_track_regex = regexp.MustCompile(`^(?i)(.+?)(?:[\-_]{1}(Mainline|Classic|Vanilla|TBC|BCC|Wrath){1})?\.toc$`)

// "returns a list of TOC structs at the given `addon_path`.
func find_toc_files(addon_path PathToAddon) ([]TOC, error) {
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

		game_track_id := ""
		if matches[2] != "" {
			game_track_id, err = GuessGameTrack(matches[2])
			if err != nil {
				game_track_id = "" // no long assume retail, new in v8
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

func slugify(str string) string {
	return slug.Make(str)
}

// strips any trailing version information from a string.
// "Some Title" => "Some Title", "Some Title 1.2.3" => "Some Title", "Some Title v1.2.3" => "Some Title"
func rm_trailing_version(title string) string {
	suffix_regex := regexp.MustCompile(` v?[\d\.]+$`)
	nothing := ""
	return suffix_regex.ReplaceAllString(title, nothing)
}

// "convert the 'Title' attribute in toc file to a curseforge-style slug."
func normalise_toc_title(title string) string {
	return slugify(rm_trailing_version(strings.ToLower(title)))
}

// toc.clj/parse-addon-toc
func populate_toc(kvs map[string]string, toc TOC) TOC {
	var err error
	title, has_title := kvs["title"]
	if !has_title {
		slog.Warn("addon with no 'Title' value found", "dir-name", toc.DirName, "toc-file", toc.FileName, "kvs", kvs)
	}
	toc.Title = title // preserve the original title, even if it's missing

	label := toc.DirName + " *" // "EveryAddon *"
	if has_title {
		label = title
	}
	toc.Label = label

	// originally used to create a match in the catalogue
	// "AdiBags" => "adibags", "AdiBags *" => "adibags", "AdiBags v1.2.3" => "adibags"
	toc.Name = normalise_toc_title(toc.Label)

	source_map_list := []SourceMap{}
	x_wowi_id, has_x_wowi_id := kvs["x-wowi-id"]
	if has_x_wowi_id {
		wowi_source := SourceMap{Source: SOURCE_WOWI, SourceID: FlexString(x_wowi_id)}
		source_map_list = append(source_map_list, wowi_source)
	}

	x_github_id, has_x_github_id := kvs["x-github"]
	if has_x_github_id {
		github_source := SourceMap{Source: SOURCE_GITHUB, SourceID: FlexString(x_github_id)}
		source_map_list = append(source_map_list, github_source)
	}

	toc.SourceMapList = source_map_list

	version, has_version := kvs["version"]
	toc.InstalledVersion = version

	// indications from the toc data that the addon should be ignored
	ignore_flag := false
	if has_version {
		if strings.Contains(version, "@project-version@") {
			slog.Debug("ignoring addon, 'Version' field in .toc file is unrendered", "dir-name", toc.DirName, "file-name", toc.FileName)
			ignore_flag = true
		}
	}
	toc.Ignored = ignore_flag

	interface_version, has_interface_version := kvs["interface"]
	var interface_version_int int
	if has_interface_version {
		// todo: spec interface_version, should be uint, should be in range, etc
		interface_version_int, err = core.StringToInt(interface_version)
		if err != nil {
			slog.Debug("couldn't parse interface version as integer, using default", "dir-name", toc.DirName, "file-name", toc.FileName, "interface-version", interface_version)
			interface_version_int = DEFAULT_INTERFACE_VERSION
		}
	}
	toc.InterfaceVersion = interface_version_int

	game_track, err := InterfaceVersionToGameTrack(interface_version_int)
	if err != nil {
		game_track, err = InterfaceVersionToGameTrack(DEFAULT_INTERFACE_VERSION)
		if err != nil {
			panic("programming error, default interface version cannot be converted to a game track")
		}
	}
	toc.InterfaceVersionGameTrackID = game_track

	if toc.FileNameGameTrackID != toc.InterfaceVersionGameTrackID {
		if toc.FileNameGameTrackID != "" {
			slog.Debug("game track from filename does not match game track from interface version", "filename-game-track", toc.FileNameGameTrackID, "interface-version-game-track", toc.InterfaceVersionGameTrackID, "dir-name", toc.DirName, "file-name", toc.FileName)
		}
	}

	toc.GameTrackID = toc.InterfaceVersionGameTrackID
	if toc.InterfaceVersionGameTrackID == "" {
		toc.GameTrackID = toc.FileNameGameTrackID
		if toc.FileNameGameTrackID == "" {
			slog.Debug("failed to guess a game track, defaulting to retail", "dir-name", toc.DirName, "file-name", toc.FileName)
			toc.GameTrackID = GAMETRACK_RETAIL
		}
	}

	notes, has_notes := kvs["notes"]
	if !has_notes {
		desc, has_desc := kvs["description"]
		if has_desc {
			notes = desc
		}
	}
	toc.Notes = notes

	// handled later in v8
	// ;; expanded upon in `parse-addon-toc-guard` when it knows about *all* available toc files
	// :supported-game-tracks [game-track]

	// dirsize calculations best done else in v8
	// (if-let [dirsize (:dirsize keyvals)]
	//         (assoc addon :dirsize dirsize)
	//         addon)

	// source
	// ;; prefers tukui over wowi, wowi over github. I'd like to prefer github over wowi, but github
	// ;; requires API calls to interact with and these are limited unless authenticated.
	// addon (merge addon
	//              github-source wowi-source tukui-source
	//              ignore-flag source-map-list)

	// validate.
	// todo. separate step?

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
func ParseAllAddonTocFiles(addon_path PathToAddon) (map[GameTrackID]TOC, error) {
	idx := map[GameTrackID]TOC{}

	toc_list, err := find_toc_files(addon_path)
	if err != nil {
		slog.Warn("failed to find toc files", "error", err)
		return idx, err
	}

	for _, toc := range toc_list {
		keyvals_map, err := ReadAddonTocFile(filepath.Join(addon_path, toc.FileName))
		if err != nil {
			slog.Warn("error reading contents of toc file, skipping", "dir-name", toc.DirName, "file-name", toc.FileName, "error", err)
			continue
		}
		populated_toc := populate_toc(keyvals_map, toc)
		idx[populated_toc.GameTrackID] = populated_toc
	}

	return idx, nil
}
