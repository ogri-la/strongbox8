package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gosimple/slug"
)

// --- TOC
// subset of data parsed from .toc files

// todo: should a distinction be made between 'raw' and 'processed' values?
// for example, 'Title' and 'Notes' are raw, 'Label' is processed
type TOC struct {
	Title               string          // unmodified 'title' value. new in 8.0
	Notes               string          // 'description' in v7. some addons may use 'description' the majority use 'notes'
	URL                 string          // "file:///path/to/addon-dir/AdiBags/AdiBags.toc"
	InterfaceVersionSet mapset.Set[int] // game/WoW version 101001
	InstalledVersion    string          // Addon version "v1.200-beta-alpha-extreme"
	Ignored             bool            // indicates addon should be ignored
	SourceMapList       []SourceMap     // addon is available from different sources

	FileNameGameTrackID            GameTrackID             // game track from filename
	InterfaceVersionGameTrackIDSet mapset.Set[GameTrackID] // game track(s) from the interface version(s)

	// derived

	Name           string                  // a slugified/normalised 'label'
	Label          string                  // guaranteed representative value
	DirName        string                  // "EveryAddon" in "/path/to/addon-dir/EveryAddon/EveryAddon.toc"
	FileName       string                  // "EveryAddon.toc" in "/path/to/addon-dir/EveryAddon/EveryAddon.toc". new in 8.0
	GameTrackIDSet mapset.Set[GameTrackID] // a single set derived from the filename and interface versions
}

var _ core.ItemInfo = (*TOC)(nil)

func (t TOC) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	// a toc file doesn't have any semantically significant children.
	// I imagine if I implement an explode() function in the future then a .toc file could give rise to:
	// access and modification dates, file size integer, text blob, comments, etc
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (t TOC) ItemChildren(_ *core.App) []core.Result {
	return nil
}

func (t TOC) ItemKeys() []string {
	return []string{
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		core.ITEM_FIELD_URL,
		"installed-version",
		"game-version",
		"ignored",
	}
}

func (t TOC) ItemMap() map[string]string {
	// todo: shift this into TOC{} proper
	game_version_list := []string{}
	for _, iv := range t.InterfaceVersionSet.ToSlice() {
		gv, err := InterfaceVersionToGameVersion(iv)
		if err == nil {
			game_version_list = append(game_version_list, gv)
		}
	}
	slices.Sort(game_version_list)
	game_version_str := strings.Join(game_version_list, ", ")

	return map[string]string{
		core.ITEM_FIELD_NAME: t.FileName,
		core.ITEM_FIELD_DESC: t.Notes,
		core.ITEM_FIELD_URL:  t.URL,
		"installed-version":  t.InstalledVersion,
		"game-version":       game_version_str,
		"ignored":            fmt.Sprintf("%v", t.Ignored),
	}
}

// ^                Start of the string
// (?i)             Case-insensitive matching
// (.+?)            Capture the base name (lazily)
// (?:              Open a non-capturing group
// [\-_]{1}         Match either '-' or '_' exactly once
// (Mainline|Classic|Vanilla|TBC|BCC|Wrath){1}  Capture game track (case insensitive), single instance
// )?               Close the non-capturing group, making it optional (zero or one match)
// \.toc$           Ends with .toc
// var game_track_regex = regexp.MustCompile(`^(?i)(.+?)(?:[\-_]{1}(Mainline|Classic|Vanilla|TBC|BCC|Wrath){1})?\.toc$`)
var game_track_regex = regexp.MustCompile(`^(?i)(.+?)(?:[\-_]{1}(.+))?\.toc$`)

// "returns a list of TOC structs at the given `addon_path`.
func find_toc_files(addon_path PathToAddon) ([]TOC, error) {
	empty_response := []TOC{}

	file_list, err := core.ListFiles(addon_path)
	if err != nil {
		return empty_response, err
	}

	if len(file_list) == 0 {
		return empty_response, fmt.Errorf("addon directory is empty: %s", addon_path)
	}

	toc_data := []TOC{}
	addon_dir := filepath.Base(addon_path) // "/path/to/Addon" => "Addon"

	for _, file_path := range file_list {
		file_name := filepath.Base(file_path) // "/path/to/Addon/Foo.toc" => "Foo.toc"

		// todo: remove the dirname from the toc file as it may interfere with the regex.
		// for example: 'LittleWigs_Classic' *could* have (but doesn't) 'LittleWigs_Classic_Classic.toc'.
		// it does 'LittleWigs_Classic_Vanilla.toc' instead
		// then, account for misnamed toc files ...

		// ["EveryAddon.toc","EveryAddon",""]
		// ["EveryAddon_TBC.toc","EveryAddon","TBC"]
		// ["EveryAddon_Foo.toc","AdiBags","Foo"]
		matches := game_track_regex.FindStringSubmatch(file_name)
		if len(matches) < 2 {
			// could this case ever happen?
			continue
		}

		game_track_id := ""
		if matches[2] != "" {
			game_track_id = GuessGameTrack(strings.ToLower(matches[2]))
		}

		toc := TOC{
			URL:                            "file://" + file_path, // "file:///path/to/addon/dir/AdiBags/AdiBags_TBC.toc"
			FileName:                       file_name,             // "AdiBags_TBC.toc"
			DirName:                        addon_dir,             // "AdiBags" in "/path/to/addon/dir/Adibags/AdiBags_TBC.toc"
			FileNameGameTrackID:            game_track_id,         // "classic-tbc" guessed from "AdiBags_TBC.toc"
			GameTrackIDSet:                 mapset.NewSet[GameTrackID](),
			InterfaceVersionSet:            mapset.NewSet[int](),
			SourceMapList:                  []SourceMap{},
			InterfaceVersionGameTrackIDSet: mapset.NewSet[GameTrackID](),
		}
		toc_data = append(toc_data, toc)

	}
	return toc_data, nil
}

// toc.clj/parse-toc-file
// parses the contents of .toc file into the given `toc` struct.
func parse_toc_file(toc_contents string) map[string]string {
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
				slog.Debug("skipping row", "error", err)
				continue
			}
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
// take the raw data from .toc file and parse/validate/ignore/derive new values
func parse_addon_toc(kvs map[string]string, toc TOC) TOC {
	title, has_title := kvs["title"]
	if !has_title {
		slog.Warn("addon with no 'Title' value found", "dir-name", toc.DirName, "toc-file", toc.FileName, "kvs", kvs)
	}
	toc.Title = title // preserve the original title, even if it's missing

	toc.Label = toc.DirName + " *" // "EveryAddon *"
	if has_title {
		toc.Label = title
	}

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

	x_website, has_x_website := kvs["x-website"]
	if has_x_website && !has_x_github_id {
		// if x-website points to github, use that
		p, err := url.Parse(strings.ToLower(x_website))
		if err == nil {
			if p.Hostname() == "github.com" {
				bits := strings.Split(p.Path, "/") // "ogri-la/strongbox" => ["ogri-la", "strongbox"]
				if len(bits) == 2 && bits[0] != "" && bits[1] != "" {
					github_source2 := SourceMap{Source: SOURCE_GITHUB, SourceID: FlexString(p.Path)}
					source_map_list = append(source_map_list, github_source2)
				}
			}
		}
	}

	toc.SourceMapList = source_map_list

	version, has_version := kvs["version"]
	if version == "" {
		slog.Debug("TOC file missing 'Version'", "dir-name", toc.DirName, "file-name", toc.FileName)
	}
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
	interface_version_set := mapset.NewSet[int]()
	if has_interface_version {
		bit_list := strings.Split(interface_version, ",")
		for _, bit := range bit_list {
			bit_int, err := core.StringToInt(bit)
			if err != nil {
				slog.Warn("failed to parse interface version to an integer, ignoring", "interface-version", bit, "error", err)
				continue
			}
			interface_version_set.Add(bit_int)
		}
	}
	toc.InterfaceVersionSet = interface_version_set

	for _, iv := range interface_version_set.ToSlice() {
		game_track, err := InterfaceVersionToGameTrack(iv)
		if err == nil {
			toc.InterfaceVersionGameTrackIDSet.Add(game_track)
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

// reads the contents of a *single* .toc file, returning a map of key+vals
func ReadAddonTocFile(toc_path PathToFile) (map[string]string, error) {
	empty_map := map[string]string{}
	toc_data, err := core.SlurpBytesUTF8(toc_path)
	if err != nil {
		return empty_map, err
	}
	return parse_toc_file(string(toc_data)), nil
}

// "wraps the `parse-addon-toc` function, attaching the list of `:supported-game-tracks` and sinking any errors."
func ParseAllAddonTocFiles(addon_path PathToAddon) (map[FileName]TOC, error) {
	idx := map[FileName]TOC{} // {"EveryAddon.toc": TOC{...}, ...}

	toc_file_list, err := find_toc_files(addon_path)
	if err != nil {
		return idx, fmt.Errorf("failed to parse .toc files: %w", err)
	}

	if len(toc_file_list) == 0 {
		return idx, fmt.Errorf("failed to find .toc files: %s", addon_path)
	}

	for _, toc_file := range toc_file_list {
		keyvals_map, err := ReadAddonTocFile(filepath.Join(addon_path, toc_file.FileName))
		if err != nil {
			slog.Warn("failed to parse .toc file, skipping", "path", addon_path, "error", err)
			continue
		}
		toc := parse_addon_toc(keyvals_map, toc_file)
		idx[toc.FileName] = toc
	}

	return idx, nil
}
