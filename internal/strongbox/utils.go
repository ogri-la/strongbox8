package strongbox

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// --- public

func AddonID(addon Addon) string {
	//dirname := addon.TOC.DirName    // not good. this will be 'Addons' for regular users.
	source := addon.NFO.Source      // "github"
	source_id := addon.NFO.SourceID // "adiaddons/adibags"
	return source + "/" + source_id // "github/adiaddons/adibags", "wowinterface/adibags"
}

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
