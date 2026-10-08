package strongbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// common strongbox logic

// returns `true` if the given `path` looks like an official Blizzard addon.
func BlizzardAddon(path string) bool {
	return strings.HasPrefix(filepath.Base(path), "Blizzard_")
}

// a pattern and the game track it names.
type game_track_pattern struct {
	regex      *regexp.Regexp
	game_track GameTrackID
}

// patterns checked in order, most specific game track first, so a string naming several
// game tracks gets the most specific one.
// compiled once: `GuessGameTrack` runs for every asset, release and .toc file name.
var GAME_TRACK_PATTERN_LIST = []game_track_pattern{
	// 'forever' and its codename 'camelot' as delimited words, so 'foreverything' does not match.
	{regexp.MustCompile(`(?i)(^|[^[:alnum:]])(forever|camelot)([^[:alnum:]]|$)`), GAMETRACK_FOREVER},

	// 'mists' as a delimited word.
	{regexp.MustCompile(`(?i)(^|[^[:alnum:]])mists([^[:alnum:]]|$)`), GAMETRACK_CLASSIC_MISTS},

	// 'cata' as a delimited word, so 'Catalyst' and 'catalogue' do not match.
	{regexp.MustCompile(`(?i)(^|[^[:alnum:]])cata([^[:alnum:]]|$)`), GAMETRACK_CLASSIC_CATA},

	// 'classic-wotlk', 'classic_wotlk', 'classic-wrath', 'classic_wrath', 'wotlk', 'wrath'
	{regexp.MustCompile(`(?i)(classic[\W_])?(wrath|wotlk){1}\W?`), GAMETRACK_CLASSIC_WOTLK},

	// 'classic-tbc', 'classic-bc', 'classic-bcc', 'classic_tbc', 'classic_bc', 'classic_bcc', 'tbc', 'tbcc', 'bc', 'bcc'
	// but not 'classictbc' or 'classicbc' or 'classicbcc'
	{regexp.MustCompile(`(?i)classic[\W_]t?bcc?|[\W_]t?bcc?\W?|t?bcc?$`), GAMETRACK_CLASSIC_TBC},

	{regexp.MustCompile(`(?i)classic|vanilla`), GAMETRACK_CLASSIC},

	// 'standard' as a delimited word, so 'nonstandard' does not match.
	{regexp.MustCompile(`(?i)retail|mainline|(^|[^[:alnum:]])standard([^[:alnum:]]|$)`), GAMETRACK_RETAIL},
}

// returns the game track named in the given `val`, or an empty string when none is found.
// an exact match against a known alias wins, otherwise the first match in
// `GAME_TRACK_PATTERN_LIST`.
func GuessGameTrack(val string) GameTrackID {
	// exact matches to known aliases, including release.json flavors
	game_track, present := GAMETRACK_ALIAS_MAP[val]
	if present {
		return game_track
	}

	for _, p := range GAME_TRACK_PATTERN_LIST {
		if p.regex.MatchString(val) {
			return p.game_track
		}
	}
	return ""
}

// the lowest and highest valid interface versions: 5 or 6 digits.
const (
	INTERFACE_VERSION_MIN = 10000
	INTERFACE_VERSION_MAX = 999999
)

// returns the major, minor and patch parts of the given `interface_version`.
// an interface version is `major * 10000 + minor * 100 + patch`, so 110002 is 11, 0, 2
// and 11507 is 1, 15, 7.
// returns an error when `interface_version` is outside `INTERFACE_VERSION_MIN` and
// `INTERFACE_VERSION_MAX`.
func parse_interface_version(interface_version int) (int, int, int, error) {
	if interface_version < INTERFACE_VERSION_MIN || interface_version > INTERFACE_VERSION_MAX {
		return 0, 0, 0, fmt.Errorf("interface version out of range: %d", interface_version)
	}
	major := interface_version / 10000
	minor := (interface_version / 100) % 100
	patch := interface_version % 100
	return major, minor, patch, nil
}

// returns the given `interface_version_int` as a game version.
// for example: 100105 => "10.1.5", 30402 => "3.4.2", 11507 => "1.15.7".
// returns an error when the interface version is not 5 or 6 digits.
// - https://wow.gamepedia.com/Patches
func InterfaceVersionToGameVersion(interface_version_int int) (string, error) {
	major, minor, patch, err := parse_interface_version(interface_version_int)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// a half-open range of interface versions, `[from, to)`, and its game track.
type interface_version_range struct {
	from       int
	to         int
	game_track GameTrackID
}

// interface version ranges in ascending order, covering every valid interface version.
// a sequence of ranges rather than a switch, so the mapping reads as data and adding a
// game track is adding a row.
var INTERFACE_VERSION_RANGE_LIST = []interface_version_range{
	{10000, 16000, GAMETRACK_CLASSIC},
	{16000, 20000, GAMETRACK_FOREVER}, // 1.60 to 1.99
	{20000, 30000, GAMETRACK_CLASSIC_TBC},
	{30000, 40000, GAMETRACK_CLASSIC_WOTLK},
	{40000, 50000, GAMETRACK_CLASSIC_CATA},
	{50000, 60000, GAMETRACK_CLASSIC_MISTS},
	{60000, INTERFACE_VERSION_MAX + 1, GAMETRACK_RETAIL},
}

// returns the game track for the given `interface_version`.
// for example: 100105 => retail, 40400 => classic-cata, 11507 => classic.
// returns an error when the interface version is not 5 or 6 digits.
func InterfaceVersionToGameTrack(interface_version int) (GameTrackID, error) {
	_, _, _, err := parse_interface_version(interface_version)
	if err != nil {
		return "", err
	}
	for _, r := range INTERFACE_VERSION_RANGE_LIST {
		if interface_version >= r.from && interface_version < r.to {
			return r.game_track, nil
		}
	}
	return "", nil
}

func IsBeforeClassic(dt time.Time) bool {
	return dt.Before(WOWClassicReleaseDate())
}

// "\|c" literal "|c"
// "[0-9a-fA-F]{8}" 8 hex characters 0-F, case insensitive
// or "\|r" literal "|r" (reset sequence)
const escape_sequence_regex_str = `\|c[0-9a-fA-F]{8}|\|r`

var escape_sequence_regex = regexp.MustCompile(escape_sequence_regex_str)

// returns `val` with any WoW colour escape sequences removed.
func RemoveEscapeSequences(val string) string {
	return escape_sequence_regex.ReplaceAllString(val, "")
}

// writes `data` to `path` so the file holds either its previous contents or `data`, never
// a mixture or a truncated file: the data is written to a temporary file beside `path`,
// synced, then renamed over it. parent directories are created.
func write_atomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp_path := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	close_err := tmp.Close()
	if err == nil {
		err = close_err
	}
	if err == nil {
		// keep the permissions of the file being replaced, otherwise readable by the user only
		mode := os.FileMode(0o644)
		if stat, serr := os.Stat(path); serr == nil {
			mode = stat.Mode().Perm()
		}
		err = os.Chmod(tmp_path, mode)
	}
	if err == nil {
		err = os.Rename(tmp_path, path)
	}
	if err != nil {
		os.Remove(tmp_path)
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// matches WoW's client directory names, such as '_retail_', '_classic_' and '_classic_era_'.
var WOW_CLIENT_DIR_REGEX = regexp.MustCompile(`^_[a-z_]+_$`)

// returns the game track named by the WoW client directory in `path`, retail when there
// is none.
// only client directories are considered, innermost first: guessing from every directory
// name would find game tracks in names like '/home/abc'.
// '/games/wow/_classic_era_/Interface/AddOns' => classic.
func guess_game_track_from_path(path string) GameTrackID {
	segment_list := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i := len(segment_list) - 1; i >= 0; i-- {
		segment := strings.ToLower(segment_list[i])
		if !WOW_CLIENT_DIR_REGEX.MatchString(segment) {
			continue
		}
		if game_track := GuessGameTrack(strings.Trim(segment, "_")); game_track != "" {
			return game_track
		}
	}
	return GAMETRACK_RETAIL
}
