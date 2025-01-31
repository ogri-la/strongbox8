package strongbox

import (
	"bw/core"
	"encoding/json"
)

/*
   common types and model wrangling for strongbox.
   similar to constants.clj and specs.clj

   could probably do with a rename
*/

const DEFAULT_INTERFACE_VERSION = 100000

const NFO_FILENAME = ".strongbox.json"

// for converting fields that are either ints or strings to just strings.
// deprecated.
// inspiration from here:
// - https://docs.bitnami.com/tutorials/dealing-with-json-with-non-homogeneous-types-in-go
type FlexString string

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

type PathToFile = string        // "/path/to/some/file.ext"
type PathToDir = string         // "/path/to/some/dir/"
type PathToAddon = string       // "/path/to/addon-dir/Addon/"
type PathToDirOfAddons = string // "/path/to/addon-dir/"

type GameTrackID = string

const (
	GAMETRACK_RETAIL        GameTrackID = "retail"
	GAMETRACK_CLASSIC       GameTrackID = "classic"
	GAMETRACK_CLASSIC_TBC   GameTrackID = "classic-tbc"
	GAMETRACK_CLASSIC_WOTLK GameTrackID = "classic-wotlk"
)

// when game track matching is not-strict, this is the lookup order
var GT_PREF_MAP map[GameTrackID][]GameTrackID = map[GameTrackID][]GameTrackID{
	GAMETRACK_RETAIL:        {GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK},
	GAMETRACK_CLASSIC:       {GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_TBC:   {GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	GAMETRACK_CLASSIC_WOTLK: {GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
}

// deterministic, unique, IDs for finding strongbox data
const (
	ID_PREFERENCES    = "strongbox preferences"
	ID_CATALOGUE      = "strongbox catalogue"
	ID_USER_CATALOGUE = "strongbox user catalogue"
)

// namespaces for grouping common strongbox data
var (
	NS_CATALOGUE       = core.NS{Major: "strongbox", Minor: "catalogue", Type: ""}
	NS_CATALOGUE_LOC   = core.NS{Major: "strongbox", Minor: "catalogue", Type: "location"} // a catalogue location
	NS_CATALOGUE_USER  = core.NS{Major: "strongbox", Minor: "catalogue", Type: "user"}     // the user catalogue
	NS_CATALOGUE_ADDON = core.NS{Major: "strongbox", Minor: "catalogue", Type: "addon"}    // an addon within a catalogue

	NS_ADDON_DIR = core.NS{Major: "strongbox", Minor: "addons-dir", Type: "dir"} // a directory containing addons

	NS_ADDON           = core.NS{Major: "strongbox", Minor: "addon", Type: ""}                // a merging of different addon data
	NS_INSTALLED_ADDON = core.NS{Major: "strongbox", Minor: "addon", Type: "installed-addon"} // an addon within an addons-dir
	NS_TOC             = core.NS{Major: "strongbox", Minor: "addon", Type: "toc"}             // a .toc file within an installed-addon

	NS_PREFS = core.NS{Major: "strongbox", Minor: "settings", Type: "preference"} // a mapping of user preferences
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

var DISABLED_HOSTS = map[Source]bool{
	SOURCE_CURSEFORGE:          true,
	SOURCE_TUKUI:               true,
	SOURCE_TUKUI_CLASSIC:       true,
	SOURCE_TUKUI_CLASSIC_TBC:   true,
	SOURCE_TUKUI_CLASSIC_WOTLK: true,
}

// returns `true` if the addon host has been disabled
// addon.clj/host-disabled?
func HostDisabled(source Source) bool {
	_, present := DISABLED_HOSTS[source]
	return present
}
