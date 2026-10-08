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
// every game track prefers itself first, falls back to every other supported game track
// except forever, and forever falls back only to retail.
func TestGameTrackPrefMapValues(t *testing.T) {
	assert.Equal(t, SUPPORTED_GAME_TRACKS.Cardinality(), len(GAMETRACK_PREF_MAP))
	for game_track_id, pref_list := range GAMETRACK_PREF_MAP {
		assert.Equal(t, game_track_id, pref_list[0], game_track_id)
		assert.Len(t, pref_list, mapset.NewSet(pref_list...).Cardinality(), "no duplicates in %s", game_track_id)
		if game_track_id == GAMETRACK_FOREVER {
			assert.Equal(t, []GameTrackID{GAMETRACK_FOREVER, GAMETRACK_RETAIL}, pref_list)
			continue
		}
		expected := SUPPORTED_GAME_TRACKS.Clone()
		expected.Remove(GAMETRACK_FOREVER)
		assert.Equal(t, expected, mapset.NewSet(pref_list...), game_track_id)
	}
}

// the preference orders in the toc-selection spec.
func TestGameTrackPrefMap__spec(t *testing.T) {
	expected := map[GameTrackID][]GameTrackID{
		GAMETRACK_RETAIL:        {"retail", "classic", "classic-tbc", "classic-wotlk", "classic-cata", "classic-mists"},
		GAMETRACK_CLASSIC:       {"classic", "classic-tbc", "classic-wotlk", "classic-cata", "classic-mists", "retail"},
		GAMETRACK_CLASSIC_TBC:   {"classic-tbc", "classic-wotlk", "classic-cata", "classic-mists", "classic", "retail"},
		GAMETRACK_CLASSIC_WOTLK: {"classic-wotlk", "classic-cata", "classic-mists", "classic-tbc", "classic", "retail"},
		GAMETRACK_CLASSIC_CATA:  {"classic-cata", "classic-mists", "classic-wotlk", "classic-tbc", "classic", "retail"},
		GAMETRACK_CLASSIC_MISTS: {"classic-mists", "classic-cata", "classic-wotlk", "classic-tbc", "classic", "retail"},
		GAMETRACK_FOREVER:       {"forever", "retail"},
	}
	assert.Equal(t, expected, GAMETRACK_PREF_MAP)
}

// clj: `specs_test.clj/game-tracks-label-map`
func TestGameTrackLabels(t *testing.T) {
	expected := map[GameTrackID]string{
		"retail": "Retail", "classic": "Classic", "classic-tbc": "Classic (TBC)", "classic-wotlk": "Classic (WotLK)",
		"classic-cata": "Classic (Cata)", "classic-mists": "Classic (Mists)", "forever": "Forever",
	}
	actual := map[GameTrackID]string{}
	for _, gt := range GAME_TRACK_LIST {
		actual[gt.ID] = gt.Label
	}
	assert.Equal(t, expected, actual)
	assert.Equal(t, "Forever", GameTrackLabel(GAMETRACK_FOREVER))
	assert.Equal(t, "classic-bfa", GameTrackLabel("classic-bfa"))
}

// every alias resolves to a supported game track.
func TestGameTrackAliasMap(t *testing.T) {
	for alias, game_track_id := range GAMETRACK_ALIAS_MAP {
		assert.True(t, SUPPORTED_GAME_TRACKS.Contains(game_track_id), alias)
	}
}
