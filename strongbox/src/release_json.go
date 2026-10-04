package strongbox

import (
	"encoding/json"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
)

type ReleaseJSONFlavor = string

var (
	RELEASE_JSON_FLAVOR_MAINLINE ReleaseJSONFlavor = "mainline"
	RELEASE_JSON_FLAVOR_CLASSIC  ReleaseJSONFlavor = "classic"
	RELEASE_JSON_FLAVOR_BCC      ReleaseJSONFlavor = "bcc"
	RELEASE_JSON_FLAVOR_WRATH    ReleaseJSONFlavor = "wrath"
	RELEASE_JSON_FLAVOR_CATA     ReleaseJSONFlavor = "cata"
)

// mapping of release.json flavors/gametracks to strongbox canonical gametracks.
// note: these are also captured entirely in the `GAMETRACK_ALIAS_MAP`
var RELEASE_JSON_GAMETRACK_MAP = map[ReleaseJSONFlavor]GameTrackID{
	RELEASE_JSON_FLAVOR_MAINLINE: GAMETRACK_RETAIL,
	RELEASE_JSON_FLAVOR_CLASSIC:  GAMETRACK_CLASSIC,
	RELEASE_JSON_FLAVOR_BCC:      GAMETRACK_CLASSIC_TBC,
	RELEASE_JSON_FLAVOR_WRATH:    GAMETRACK_CLASSIC_WOTLK,
	RELEASE_JSON_FLAVOR_CATA:     GAMETRACK_CLASSIC_CATA,
}

type ReleaseJSONMetadata struct {
	Flavor    ReleaseJSONFlavor `json:"flavor"`
	Interface int               `json:"interface"`
}

type ReleaseJSONRelease struct {
	Name         string                `json:"name"`
	Version      string                `json:"version"`
	Filename     string                `json:"filename"`
	NoLib        bool                  `json:"nolib"`
	MetadataList []ReleaseJSONMetadata `json:"metadata"`
}

type ReleaseJSON struct {
	ReleaseList []ReleaseJSONRelease `json:"releases"`
}

// returns the given bytes `b` as a `ReleaseJSON`.
// returns an error when `b` is not a valid release.json. A release.json is remote data
// written by an addon author, so a bad one is an error for the caller to recover from.
func ParseReleaseJSON(b []byte) (ReleaseJSON, error) {
	var release_json ReleaseJSON
	err := json.Unmarshal(b, &release_json)
	if err != nil {
		return ReleaseJSON{}, fmt.Errorf("failed to parse release.json bytes: %w", err)
	}
	return release_json, nil
}

// returns the game tracks of the recognised flavors in `metadata_list`.
// an unrecognised flavor contributes nothing.
func release_json_game_tracks(metadata_list []ReleaseJSONMetadata) mapset.Set[GameTrackID] {
	set := mapset.NewSet[GameTrackID]()
	for _, md := range metadata_list {
		game_track := GuessGameTrack(md.Flavor)
		if game_track != "" {
			set.Add(game_track)
		}
	}
	return set
}

// returns every recognised game track mentioned across all releases in `rj`.
func ReleaseJSONGameTrackList(rj ReleaseJSON) mapset.Set[GameTrackID] {
	set := mapset.NewSet[GameTrackID]()
	for _, rl := range rj.ReleaseList {
		set = set.Union(release_json_game_tracks(rl.MetadataList))
	}
	return set
}

// returns the recognised game tracks of each release in `rj`, keyed by release file name.
// a release with no recognised flavor is absent, so it cannot replace a game track guessed
// by other means.
func ReleaseJSONGameTrackMap(rj ReleaseJSON) map[string]mapset.Set[GameTrackID] {
	m := map[string]mapset.Set[GameTrackID]{}
	for _, rl := range rj.ReleaseList {
		set := release_json_game_tracks(rl.MetadataList)
		if set.IsEmpty() {
			continue
		}
		m[rl.Filename] = set
	}
	return m
}
