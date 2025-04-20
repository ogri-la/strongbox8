package strongbox

import (
	"encoding/json"
	"fmt"
	"log/slog"

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

func ParseReleaseJSON(b []byte) (ReleaseJSON, error) {
	empty_resp := ReleaseJSON{}
	var release_json ReleaseJSON
	err := json.Unmarshal(b, &release_json)
	if err != nil {
		slog.Error("failed to parse release.json", "e", err, "s", string(b))
		panic("")
		return empty_resp, fmt.Errorf("failed to parse release.json bytes: %w", err)
	}
	return release_json, nil
}

func ReleaseJSONGameTrackList(rj ReleaseJSON) mapset.Set[GameTrackID] {
	set := mapset.NewSet[GameTrackID]()
	for _, rl := range rj.ReleaseList {
		for _, md := range rl.MetadataList {
			set.Add(GuessGameTrack(md.Flavor))
		}
	}
	return set
}

func ReleaseJSONGameTrackMap(rj ReleaseJSON) map[string]mapset.Set[GameTrackID] {
	m := map[string]mapset.Set[GameTrackID]{}
	for _, rl := range rj.ReleaseList {
		set := mapset.NewSet[GameTrackID]()
		for _, md := range rl.MetadataList {
			set.Add(GuessGameTrack(md.Flavor))
		}
		m[rl.Filename] = set
	}
	return m
}
