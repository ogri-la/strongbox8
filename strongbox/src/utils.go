package strongbox

import (
	"bw/internal/core"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

/*
   common strongbox logic
*/

// "returns `true` if given `path` looks like an official Blizzard addon"
func BlizzardAddon(path string) bool {
	// (-> path fs/base-name (.startsWith "Blizzard_")))
	return strings.HasPrefix(filepath.Base(path), "Blizzard_")
}

// returns the first game track it finds in the given string, preferring `:classic-wotlk`, then `:classic-tbc`, then `:classic`, then `:retail` (most to least specific).
// returns an error if no game track found.
func GuessGameTrack(val string) (GameTrackID, error) {

	// matches 'classic-wotlk', 'classic_wotlk', 'classic-wrath', 'classic_wrath', 'wotlk', 'wrath'
	classic_wotlk_regex := regexp.MustCompile(`(?i)(classic[\W_])?(wrath|wotlk){1}\W?`)

	// matches 'classic-tbc', 'classic-bc', 'classic-bcc', 'classic_tbc', 'classic_bc', 'classic_bcc', 'tbc', 'tbcc', 'bc', 'bcc'
	// but not 'classictbc' or 'classicbc' or 'classicbcc'
	// see tests.
	classic_tbc_regex := regexp.MustCompile(`(?i)classic[\W_]t?bcc?|[\W_]t?bcc?\W?|t?bcc?$`)
	classic_regex := regexp.MustCompile(`(?i)classic|vanilla`)
	retail_regex := regexp.MustCompile(`(?i)retail|mainline`)

	if classic_wotlk_regex.MatchString(val) {
		return GAMETRACK_CLASSIC_WOTLK, nil
	}
	if classic_tbc_regex.MatchString(val) {
		return GAMETRACK_CLASSIC_TBC, nil
	}
	if classic_regex.MatchString(val) {
		return GAMETRACK_CLASSIC, nil
	}
	if retail_regex.MatchString(val) {
		return GAMETRACK_RETAIL, nil
	}

	return "", fmt.Errorf("game track not found for value: '%s'", val)
}

// 100105 => 10.1.5, 30402 => 3.4.2, 11402 => 1.4.2
// see: https://wow.gamepedia.com/Patches
func InterfaceVersionToGameVersion(interface_version_int int) (string, error) {
	regex := regexp.MustCompile(`(?P<major>\d0|\d{1})\d(?P<minor>\d{1})\d(?P<patch>\d{1}\w?)`)
	matches := regex.FindStringSubmatch(core.IntToString(interface_version_int))
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
