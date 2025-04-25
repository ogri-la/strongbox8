package strongbox

import (
	"bw/core"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

/*
   common strongbox logic
*/

// "returns `true` if given `path` looks like an official Blizzard addon"
func BlizzardAddon(path string) bool {
	// (-> path fs/base-name (.startsWith "Blizzard_")))
	return strings.HasPrefix(filepath.Base(path), "Blizzard_")
}

// returns the first game track it finds in the given string,
// preferring `:classic-wotlk`, then `:classic-tbc`, then `:classic`, then `:retail` (most to least specific).
// returns an empty string if a game track couldn't be guessed.
func GuessGameTrack(val string) GameTrackID {

	// short-circuit for exact matches to known aliases, including release.json flavors
	gametrack_from_common_cases, present := GAMETRACK_ALIAS_MAP[val]
	if present {
		return gametrack_from_common_cases
	}

	// fuzzier matching

	// matches 'classic-wotlk', 'classic_wotlk', 'classic-wrath', 'classic_wrath', 'wotlk', 'wrath'
	classic_wotlk_regex := regexp.MustCompile(`(?i)(classic[\W_])?(wrath|wotlk){1}\W?`)

	// matches 'classic-tbc', 'classic-bc', 'classic-bcc', 'classic_tbc', 'classic_bc', 'classic_bcc', 'tbc', 'tbcc', 'bc', 'bcc'
	// but not 'classictbc' or 'classicbc' or 'classicbcc'
	// see tests.
	classic_tbc_regex := regexp.MustCompile(`(?i)classic[\W_]t?bcc?|[\W_]t?bcc?\W?|t?bcc?$`)
	classic_regex := regexp.MustCompile(`(?i)classic|vanilla`)
	retail_regex := regexp.MustCompile(`(?i)retail|mainline`)

	if classic_wotlk_regex.MatchString(val) {
		return GAMETRACK_CLASSIC_WOTLK
	}
	if classic_tbc_regex.MatchString(val) {
		return GAMETRACK_CLASSIC_TBC
	}
	if classic_regex.MatchString(val) {
		return GAMETRACK_CLASSIC
	}
	if retail_regex.MatchString(val) {
		return GAMETRACK_RETAIL
	}

	return ""
}

var InterfaceVersionToGameVersion_regex = regexp.MustCompile(`(?P<major>\d0|\d{1})\d(?P<minor>\d{1})\d(?P<patch>\d{1}\w?)`)

// 100105 => 10.1.5, 30402 => 3.4.2, 11402 => 1.4.2
// see: https://wow.gamepedia.com/Patches
func InterfaceVersionToGameVersion(interface_version_int int) (string, error) {
	matches := InterfaceVersionToGameVersion_regex.FindStringSubmatch(core.IntToString(interface_version_int))
	if len(matches) != 4 {
		return "", fmt.Errorf("could not parse interface game track from interface version: %d", interface_version_int)
	}
	return fmt.Sprintf("%s.%s.%s", matches[1], matches[2], matches[3]), nil
}

// 10.1.0 => retail, 1.14.3 => classic, etc
func GameVersionToGameTrack(game_version string) GameTrackID {
	entry, present := map[string]string{
		"1.": GAMETRACK_CLASSIC,
		"2.": GAMETRACK_CLASSIC_TBC,
		"3.": GAMETRACK_CLASSIC_WOTLK,
	}[game_version[:2]] // "1.14.3" => "1."
	if !present {
		return GAMETRACK_RETAIL
	}
	return entry
}

// 100105 => retail, 30402 => classic-wotlk, 11402 => classic, etc
func InterfaceVersionToGameTrack(interface_version int) (GameTrackID, error) {
	game_version, err := InterfaceVersionToGameVersion(interface_version)
	if err != nil {
		return "", err
	}
	return GameVersionToGameTrack(game_version), nil
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

func RemoveEscapeSequences(val string) string {
	return escape_sequence_regex.ReplaceAllString(val, "")
}

func Ptr[T any](v T) *T {
	return &v
}

// returns true if the given game track is using an dead 'compound' game track
func is_compound_game_track(gt GameTrackID) bool {
	return gt == GAMETRACK_RETAIL_CLASSIC || gt == GAMETRACK_CLASSIC_RETAIL
}

// converts an AddonDir using a dead 'compound' game track
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
