package strongbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the game track issues raised for an nfo holding `game_track_id`.
// the fixture is otherwise valid under the full nfo schema, so any issue is the game
// track's.
func nfo_game_track_issues(game_track_id GameTrackID) int {
	nfo := test_fixture_nfo_single
	nfo.Source = SOURCE_GITHUB // the fixture's curseforge is no longer a supported host
	nfo.SourceMapList = []SourceMap{{Source: SOURCE_GITHUB, SourceID: nfo.SourceID}}
	nfo.InstalledGameTrackID = game_track_id
	return len(_nfo_schema.Validate(&nfo)["InstalledGameTrackID"])
}

func TestNFOSchema__game_track(t *testing.T) {
	for _, game_track_id := range SUPPORTED_GAME_TRACKS.ToSlice() {
		assert.Equal(t, 0, nfo_game_track_issues(game_track_id), game_track_id)
	}
}

func TestNFOSchema__game_track__rejected(t *testing.T) {
	var cases = []GameTrackID{"nonsense", "mainline", "wrath", ""}
	for _, game_track_id := range cases {
		assert.NotEqual(t, 0, nfo_game_track_issues(game_track_id), game_track_id)
	}
}

// strongbox writes only live game tracks. an nfo written by strongbox 7.x under a
// compound game track is still read, see `valid_nfo_for_read`.
func TestNFOSchema__game_track__compound_rejected(t *testing.T) {
	assert.NotEqual(t, 0, nfo_game_track_issues(GAMETRACK_RETAIL_CLASSIC))
	assert.NotEqual(t, 0, nfo_game_track_issues(GAMETRACK_CLASSIC_RETAIL))
	assert.NoError(t, valid_nfo_for_read(NFO{GroupID: "x", InstalledGameTrackID: GAMETRACK_RETAIL_CLASSIC}))
}
