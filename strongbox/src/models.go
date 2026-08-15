// a World of Warcraft addon manager, written as a `bw` provider.
// addons are installed into an 'addons dir', identified against a 'catalogue' of known
// addons and updated from a 'source' host such as Github or WoWInterface.
// each installed addon carries a `.strongbox.json` 'nfo' file recording where it came from.
package strongbox

import (
	"bw/core"
	"encoding/json"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

// common types and constants.
// the Clojure implementation split these across `constants.clj` and `specs.clj`.

const MASCOT = "ᕙ[°▿°]ᕗ"

// returns the date WoW Classic went live.
// used to guess possible game tracks when a release's track is ambiguous:
// addon development may predate this, so an older release can't be Classic.
// - https://warcraft.wiki.gg/wiki/Public_client_builds
// - https://worldofwarcraft.com/en-us/news/22990080/mark-your-calendars-wow-classic-launch-and-testing-schedule
func WOWClassicReleaseDate() time.Time {
	return time.Date(2019, 8, 26, 0, 0, 0, 0, time.UTC) // "2019-08-26T00:00:00Z"
}

const DEFAULT_INTERFACE_VERSION = int(100000)

const NFO_FILENAME = ".strongbox.json"

var VCS_DIR_SET = mapset.NewSet(".git", ".svn", ".hg")

// for converting fields that are either ints or strings to just strings.
// deprecated. all FlexString fields will become simple strings in the future.
// inspiration from here:
// - https://docs.bitnami.com/tutorials/dealing-with-json-with-non-homogeneous-types-in-go
type FlexString string

// reads a JSON string or a JSON int, storing either as a string.
func (fi *FlexString) UnmarshalJSON(b []byte) error {
	if b[0] != '"' {
		// we have an int (hopefully)
		var i int
		err := json.Unmarshal(b, &i)
		if err != nil {
			return err
		}
		*fi = FlexString(core.IntToString(i))
		return nil
	}
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*fi = FlexString(s)
	return nil
}

// just some aliases for readability
type PathToFile = string  // "/path/to/some/file.ext"
type PathToDir = string   // "/path/to/some/dir/"
type PathToAddon = string // "/path/to/addons-dir/Addon/"
type FileName = string    // "file.ext"
type URL = string         // "https://example.org/path/to/file#anchor?foo=bar&baz=bup"

type GameTrackID = string

const (
	GAMETRACK_RETAIL        GameTrackID = "retail"
	GAMETRACK_CLASSIC       GameTrackID = "classic"
	GAMETRACK_CLASSIC_TBC   GameTrackID = "classic-tbc"
	GAMETRACK_CLASSIC_WOTLK GameTrackID = "classic-wotlk"
	GAMETRACK_CLASSIC_CATA  GameTrackID = "classic-cata"

	// dead
	GAMETRACK_RETAIL_CLASSIC GameTrackID = "retail-classic"
	GAMETRACK_CLASSIC_RETAIL GameTrackID = "classic-retail"
)

var ALL_GAME_TRACKS = mapset.NewSet(
	GAMETRACK_RETAIL,
	GAMETRACK_CLASSIC,
	GAMETRACK_CLASSIC_TBC,
	GAMETRACK_CLASSIC_WOTLK,
	GAMETRACK_CLASSIC_CATA,
	GAMETRACK_RETAIL_CLASSIC,
	GAMETRACK_CLASSIC_RETAIL,
)

var SUPPORTED_GAME_TRACKS = mapset.NewSet(
	GAMETRACK_RETAIL,
	GAMETRACK_CLASSIC,
	GAMETRACK_CLASSIC_TBC,
	GAMETRACK_CLASSIC_WOTLK,
	GAMETRACK_CLASSIC_CATA,
)

var SUPPORTED_GAME_TRACKS_LIST = ALL_GAME_TRACKS.ToSlice()

// the order to check game tracks in when matching is not strict.
// when an addon has no release for the wanted game track, the next-best tracks are the
// ones 'closest' to it, newest to oldest.
// for example, when a wotlk release is missing but cata, tbc and vanilla exist, this
// prefers cata, then tbc, then vanilla.
var GAMETRACK_PREF_MAP = map[GameTrackID][]GameTrackID{
	GAMETRACK_RETAIL:        {GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_CATA},
	GAMETRACK_CLASSIC:       {GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_CATA, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_TBC:   {GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_WOTLK: {GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_CATA:  {GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
}

func gametrack_set() mapset.Set[GameTrackID] {
	return mapset.NewSetFromMapKeys(GAMETRACK_PREF_MAP)
}

// mapping of known gametrack aliases to strongbox canonical version
var GAMETRACK_ALIAS_MAP = map[string]GameTrackID{
	GAMETRACK_RETAIL: GAMETRACK_RETAIL,
	"mainline":       GAMETRACK_RETAIL,

	GAMETRACK_CLASSIC: GAMETRACK_CLASSIC,
	"vanilla":         GAMETRACK_CLASSIC,

	GAMETRACK_CLASSIC_TBC: GAMETRACK_CLASSIC_TBC,
	"tbc":                 GAMETRACK_CLASSIC_TBC,
	"bcc":                 GAMETRACK_CLASSIC_TBC,

	GAMETRACK_CLASSIC_WOTLK: GAMETRACK_CLASSIC_WOTLK,
	"wrath":                 GAMETRACK_CLASSIC_WOTLK,
	"wotlk":                 GAMETRACK_CLASSIC_WOTLK,

	GAMETRACK_CLASSIC_CATA: GAMETRACK_CLASSIC_CATA,
	"cata":                 GAMETRACK_CLASSIC_CATA,
}

// deterministic, unique, IDs for finding strongbox data
const (
	ID_SETTINGS       = "strongbox settings"
	ID_CATALOGUE      = "strongbox catalogue"
	ID_USER_CATALOGUE = "strongbox user catalogue"
)

// namespaces for grouping common strongbox data
var (
	NS_CATALOGUE       = core.NS{Major: "strongbox", Minor: "catalogue", Type: ""}
	NS_CATALOGUE_LOC   = core.NS{Major: "strongbox", Minor: "catalogue", Type: "location"} // a catalogue location
	NS_CATALOGUE_USER  = core.NS{Major: "strongbox", Minor: "catalogue", Type: "user"}     // the user catalogue
	NS_CATALOGUE_ADDON = core.NS{Major: "strongbox", Minor: "catalogue", Type: "addon"}    // an addon within a catalogue

	NS_ADDONS_DIR = core.NS{Major: "strongbox", Minor: "addons-dir", Type: "dir"} // a directory containing addons

	NS_SOURCE_UPDATE   = core.NS{Major: "strongbox", Minor: "addon", Type: "update"}
	NS_ADDON           = core.NS{Major: "strongbox", Minor: "addon", Type: ""}                // a merging of different addon data
	NS_INSTALLED_ADDON = core.NS{Major: "strongbox", Minor: "addon", Type: "installed-addon"} // an addon within an addons-dir
	NS_TOC             = core.NS{Major: "strongbox", Minor: "addon", Type: "toc"}             // a .toc file within an installed-addon

	NS_SETTINGS = core.NS{Major: "strongbox", Minor: "settings", Type: "preference"} // a mapping of user preferences
)

// extended information about a GameTrack
type GameTrack struct {
	ID    GameTrackID // Retail
	Label string
	// description ?
	// original release date ?
	// classic release date ?
}

var (
	GT_RETAIL        = GameTrack{GAMETRACK_RETAIL, "Retail"}
	GT_CLASSIC       = GameTrack{GAMETRACK_CLASSIC, "Classic"}
	GT_CLASSIC_TBC   = GameTrack{GAMETRACK_CLASSIC_TBC, "Classic (TBC)"}
	GT_CLASSIC_WOTLK = GameTrack{GAMETRACK_CLASSIC_WOTLK, "Classic (WotLK)"}
)

type Source = string

const (
	SOURCE_GITHUB Source = "github"
	SOURCE_GITLAB Source = "gitlab"
	SOURCE_WOWI   Source = "wowinterface"

	// dead
	SOURCE_CURSEFORGE          Source = "curseforge"
	SOURCE_TUKUI               Source = "tukui"
	SOURCE_TUKUI_CLASSIC       Source = "tukui-classic"
	SOURCE_TUKUI_CLASSIC_TBC   Source = "tukui-classic-tbc"
	SOURCE_TUKUI_CLASSIC_WOTLK Source = "tukui-classic-wotlk"
)

var SUPPORTED_HOSTS = mapset.NewSet(
	SOURCE_GITHUB,
	SOURCE_GITLAB,
	SOURCE_WOWI,
)

var SUPPORTED_HOSTS_LIST = SUPPORTED_HOSTS.ToSlice()

var DISABLED_HOSTS = mapset.NewSet(
	SOURCE_CURSEFORGE,
	SOURCE_TUKUI,
	SOURCE_TUKUI_CLASSIC,
	SOURCE_TUKUI_CLASSIC_TBC,
	SOURCE_TUKUI_CLASSIC_WOTLK,
)
