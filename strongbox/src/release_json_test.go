package strongbox

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// a release.json with one zip per game track, the common shape.
var dummy_release_json = ReleaseJSON{
	ReleaseList: []ReleaseJSONRelease{
		{
			Name:     "EveryAddon",
			Version:  "1.2.3",
			Filename: "EveryAddon-1.2.3.zip",
			MetadataList: []ReleaseJSONMetadata{
				{Flavor: RELEASE_JSON_FLAVOR_MAINLINE, Interface: 100105},
			},
		},
		{
			Name:     "EveryAddon",
			Version:  "1.2.3",
			Filename: "EveryAddon-1.2.3-classic.zip",
			MetadataList: []ReleaseJSONMetadata{
				{Flavor: RELEASE_JSON_FLAVOR_CLASSIC, Interface: 11403},
			},
		},
	},
}

func TestParseReleaseJSON(t *testing.T) {
	given := []byte(`{
      "releases": [
        {
          "name": "EveryAddon",
          "version": "1.2.3",
          "filename": "EveryAddon-1.2.3.zip",
          "nolib": false,
          "metadata": [{"flavor": "mainline", "interface": 100105}]
        }
      ]
    }`)
	expected := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Name:     "EveryAddon",
				Version:  "1.2.3",
				Filename: "EveryAddon-1.2.3.zip",
				NoLib:    false,
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: RELEASE_JSON_FLAVOR_MAINLINE, Interface: 100105},
				},
			},
		},
	}
	actual, err := ParseReleaseJSON(given)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

// an empty release list parses without error.
func TestParseReleaseJSON__empty(t *testing.T) {
	given := []byte(`{"releases": []}`)
	expected := ReleaseJSON{ReleaseList: []ReleaseJSONRelease{}}
	actual, err := ParseReleaseJSON(given)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

// malformed JSON is an error, not a panic.
func TestParseReleaseJSON__malformed(t *testing.T) {
	given_list := [][]byte{
		[]byte(`{"releases": [`),
		[]byte(`not json`),
		[]byte(``),
		[]byte(`{"releases": "foo"}`),
	}
	for _, given := range given_list {
		actual, err := ParseReleaseJSON(given)
		assert.Error(t, err, string(given))
		assert.Equal(t, ReleaseJSON{}, actual, string(given))
	}
}

// every flavor a release.json can declare maps to a game track.
func TestParseReleaseJSON__all_flavors(t *testing.T) {
	var cases = []struct {
		given    ReleaseJSONFlavor
		expected GameTrackID
	}{
		{RELEASE_JSON_FLAVOR_MAINLINE, GAMETRACK_RETAIL},
		{RELEASE_JSON_FLAVOR_CLASSIC, GAMETRACK_CLASSIC},
		{RELEASE_JSON_FLAVOR_BCC, GAMETRACK_CLASSIC_TBC},
		{RELEASE_JSON_FLAVOR_WRATH, GAMETRACK_CLASSIC_WOTLK},
		{RELEASE_JSON_FLAVOR_CATA, GAMETRACK_CLASSIC_CATA},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, RELEASE_JSON_GAMETRACK_MAP[c.given], c.given)
		// the alias map is a superset of the flavor map, see `RELEASE_JSON_GAMETRACK_MAP`.
		assert.Equal(t, c.expected, GuessGameTrack(c.given), c.given)
	}
}

func TestReleaseJSONGameTrackList(t *testing.T) {
	expected := mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC)
	assert.Equal(t, expected, ReleaseJSONGameTrackList(dummy_release_json))
}

func TestReleaseJSONGameTrackList__empty(t *testing.T) {
	expected := mapset.NewSet[GameTrackID]()
	assert.Equal(t, expected, ReleaseJSONGameTrackList(ReleaseJSON{}))
}

func TestReleaseJSONGameTrackMap(t *testing.T) {
	expected := map[string]mapset.Set[GameTrackID]{
		"EveryAddon-1.2.3.zip":         mapset.NewSet(GAMETRACK_RETAIL),
		"EveryAddon-1.2.3-classic.zip": mapset.NewSet(GAMETRACK_CLASSIC),
	}
	assert.Equal(t, expected, ReleaseJSONGameTrackMap(dummy_release_json))
}

// a single zip supporting many game tracks yields a set with many entries.
func TestReleaseJSONGameTrackMap__multi_track_asset(t *testing.T) {
	given := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename: "EveryAddon-1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: RELEASE_JSON_FLAVOR_CLASSIC},
					{Flavor: RELEASE_JSON_FLAVOR_BCC},
					{Flavor: RELEASE_JSON_FLAVOR_WRATH},
				},
			},
		},
	}
	expected := map[string]mapset.Set[GameTrackID]{
		"EveryAddon-1.2.3.zip": mapset.NewSet(
			GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK),
	}
	assert.Equal(t, expected, ReleaseJSONGameTrackMap(given))
}

// an unrecognised flavor contributes no game track, and a release left with none is
// absent from the map.
func TestReleaseJSONGameTrackMap__unknown_flavor(t *testing.T) {
	given := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename: "EveryAddon-1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: "mists"},
				},
			},
		},
	}
	expected := map[string]mapset.Set[GameTrackID]{}
	assert.Equal(t, expected, ReleaseJSONGameTrackMap(given))

	expected_list := mapset.NewSet[GameTrackID]()
	assert.Equal(t, expected_list, ReleaseJSONGameTrackList(given))
}

// a known flavor alongside an unknown one yields only the known flavor's game track.
func TestReleaseJSONGameTrackMap__partially_unknown_flavor(t *testing.T) {
	given := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename: "EveryAddon-1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: RELEASE_JSON_FLAVOR_MAINLINE},
					{Flavor: "mists"},
				},
			},
		},
	}
	expected := map[string]mapset.Set[GameTrackID]{
		"EveryAddon-1.2.3.zip": mapset.NewSet(GAMETRACK_RETAIL),
	}
	assert.Equal(t, expected, ReleaseJSONGameTrackMap(given))
	assert.Equal(t, mapset.NewSet(GAMETRACK_RETAIL), ReleaseJSONGameTrackList(given))
}

// characterisation test: two releases sharing a file name collapse to the last one seen.
func TestReleaseJSONGameTrackMap__duplicate_filenames(t *testing.T) {
	given := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename:     "EveryAddon-1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{{Flavor: RELEASE_JSON_FLAVOR_MAINLINE}},
			},
			{
				Filename:     "EveryAddon-1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{{Flavor: RELEASE_JSON_FLAVOR_CLASSIC}},
			},
		},
	}
	expected := map[string]mapset.Set[GameTrackID]{
		"EveryAddon-1.2.3.zip": mapset.NewSet(GAMETRACK_CLASSIC),
	}
	assert.Equal(t, expected, ReleaseJSONGameTrackMap(given))
}
