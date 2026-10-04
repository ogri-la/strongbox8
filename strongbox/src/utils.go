package strongbox

import (
	"fmt"
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
// an empty game track means the range belongs to a game track strongbox does not support.
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
	{16000, 20000, ""}, // forever, 1.60 to 1.99
	{20000, 30000, GAMETRACK_CLASSIC_TBC},
	{30000, 40000, GAMETRACK_CLASSIC_WOTLK},
	{40000, 50000, GAMETRACK_CLASSIC_CATA},
	{50000, 60000, ""}, // mists
	{60000, INTERFACE_VERSION_MAX + 1, GAMETRACK_RETAIL},
}

// returns the game track for the given `interface_version`.
// for example: 100105 => retail, 40400 => classic-cata, 11507 => classic.
// returns an empty game track for a valid interface version of an unsupported game track,
// such as forever or mists, and never assumes retail for it.
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

/* this path leads to madness.

// return an `Addon` struct from an `InstalledAddon` struct, filling in gaps the best we can.
// bit of a hack for when accuracy is less important.
func InstalledAddonToAddon(installed_addon InstalledAddon, parent *Addon) Addon {
	var toc_to_use TOC
	for _, gt := range GT_PREF_MAP[GAMETRACK_RETAIL] {
		toc, present := installed_addon.TOCMap[gt]
		if present {
			toc_to_use = toc
			break
		}
	}

	nfo_to_use, _ := PickNFO(installed_addon.NFOList)

	a := Addon{
		Primary: &installed_addon,
		TOC:     &toc_to_use,
		NFO:     &nfo_to_use,
	}

	return a
}

*/

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

// returns `true` when the given `gt` is one of the dead 'compound' game tracks.
func is_compound_game_track(gt GameTrackID) bool {
	return gt == GAMETRACK_RETAIL_CLASSIC || gt == GAMETRACK_CLASSIC_RETAIL
}

// returns the given `ad` with any dead 'compound' game track replaced by retail with
// strict matching off, which is what a compound track meant.
// an addons dir on a live game track is returned unchanged.
func convert_compound_game_track(ad AddonsDir) AddonsDir {
	if ad.GameTrackID == GAMETRACK_RETAIL_CLASSIC {
		ad.GameTrackID = GAMETRACK_RETAIL
		ad.Strict = false
	}

	if ad.GameTrackID == GAMETRACK_CLASSIC_RETAIL {
		ad.GameTrackID = GAMETRACK_RETAIL
		ad.Strict = false
	}
	return ad
}
