package strongbox

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// the supported game tracks are the live ones: every game track except the two dead
// compound tracks.
func TestGameTrackSets(t *testing.T) {
	expected := mapset.NewSet(GAMETRACK_RETAIL_CLASSIC, GAMETRACK_CLASSIC_RETAIL)
	assert.Equal(t, expected, ALL_GAME_TRACKS.Difference(SUPPORTED_GAME_TRACKS))
	assert.True(t, SUPPORTED_GAME_TRACKS.IsSubset(ALL_GAME_TRACKS))
}

func TestGameTrackLists(t *testing.T) {
	assert.Equal(t, ALL_GAME_TRACKS, mapset.NewSet(ALL_GAME_TRACKS_LIST...))
	assert.Equal(t, SUPPORTED_GAME_TRACKS, mapset.NewSet(SUPPORTED_GAME_TRACKS_LIST...))
}

func Test_gametrack_set(t *testing.T) {
	assert.Equal(t, SUPPORTED_GAME_TRACKS, gametrack_set())
}

// each call returns a set the caller may modify without affecting
// `SUPPORTED_GAME_TRACKS` or any set handed out earlier.
func Test_gametrack_set__returns_a_copy(t *testing.T) {
	given := gametrack_set()
	given.Remove(GAMETRACK_RETAIL)
	given.Add("nonsense")

	assert.True(t, SUPPORTED_GAME_TRACKS.Contains(GAMETRACK_RETAIL))
	assert.False(t, SUPPORTED_GAME_TRACKS.Contains("nonsense"))
	assert.Equal(t, SUPPORTED_GAME_TRACKS, gametrack_set())
}

// every supported game track has a preference list, and no dead game track has one.
func TestGameTrackPrefMapKeys(t *testing.T) {
	assert.Equal(t, SUPPORTED_GAME_TRACKS, mapset.NewSetFromMapKeys(GAMETRACK_PREF_MAP))
}

// every preference list offers each supported game track exactly once, most preferred
// first, starting with the game track itself.
func TestGameTrackPrefMapValues(t *testing.T) {
	for game_track_id, pref_list := range GAMETRACK_PREF_MAP {
		assert.Equal(t, game_track_id, pref_list[0], game_track_id)
		assert.Equal(t, SUPPORTED_GAME_TRACKS, mapset.NewSet(pref_list...), game_track_id)
		assert.Len(t, pref_list, SUPPORTED_GAME_TRACKS.Cardinality(), game_track_id)
	}
}

// every alias resolves to a supported game track.
func TestGameTrackAliasMap(t *testing.T) {
	for alias, game_track_id := range GAMETRACK_ALIAS_MAP {
		assert.True(t, SUPPORTED_GAME_TRACKS.Contains(game_track_id), alias)
	}
}
