package strongbox

// strongbox reads data produced by two other projects. their game track vocabulary is
// copied here verbatim so a change on either side fails a test rather than silently
// discarding catalogue entries or misclassifying addons.
// - github-wow-addon-catalogue: `main.go` `FLAVOR_LIST`, `FLAVOR_ALIAS_MAP`, `INTERFACE_RANGES`
// - strongbox-catalogue-builder-go: `src/types/addon.go` `AllGameTracks`, `src/github/parser.go` `guessGameTrack`

import (
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// github-wow-addon-catalogue flavour => the strongbox game track it means.
var producer_flavor_game_tracks = map[string]GameTrackID{
	// FLAVOR_LIST
	"mainline": GAMETRACK_RETAIL,
	"vanilla":  GAMETRACK_CLASSIC,
	"forever":  GAMETRACK_FOREVER,
	"tbc":      GAMETRACK_CLASSIC_TBC,
	"wrath":    GAMETRACK_CLASSIC_WOTLK,
	"cata":     GAMETRACK_CLASSIC_CATA,
	"mists":    GAMETRACK_CLASSIC_MISTS,
	// FLAVOR_ALIAS_MAP
	"classic": GAMETRACK_CLASSIC,
	"camelot": GAMETRACK_FOREVER,
	"bcc":     GAMETRACK_CLASSIC_TBC,
	"wotlk":   GAMETRACK_CLASSIC_WOTLK,
	"wotlkc":  GAMETRACK_CLASSIC_WOTLK,
}

// strongbox-catalogue-builder-go `guessGameTrack` flavour => game track.
var builder_flavor_game_tracks = map[string]GameTrackID{
	"mainline": GAMETRACK_RETAIL, "retail": GAMETRACK_RETAIL,
	"classic": GAMETRACK_CLASSIC, "vanilla": GAMETRACK_CLASSIC,
	"forever": GAMETRACK_FOREVER,
	"bcc":     GAMETRACK_CLASSIC_TBC, "tbc": GAMETRACK_CLASSIC_TBC,
	"wrath": GAMETRACK_CLASSIC_WOTLK, "wotlk": GAMETRACK_CLASSIC_WOTLK,
	"cata": GAMETRACK_CLASSIC_CATA, "cataclysm": GAMETRACK_CLASSIC_CATA,
	"mists": GAMETRACK_CLASSIC_MISTS, "mop": GAMETRACK_CLASSIC_MISTS,
}

func Test_producers__flavors(t *testing.T) {
	for flavor, expected := range producer_flavor_game_tracks {
		assert.Equal(t, expected, GuessGameTrack(flavor), flavor)
	}
	for flavor, expected := range builder_flavor_game_tracks {
		assert.Equal(t, expected, GuessGameTrack(flavor), flavor)
	}
}

// a `.toc` file named with any flavour the scraper recognises gets that game track.
func Test_producers__toc_suffixes(t *testing.T) {
	for flavor, expected := range producer_flavor_game_tracks {
		for _, sep := range []string{"-", "_"} {
			file_name := "EveryAddon" + sep + flavor + ".toc"
			toc := coerce_toc_data(map[string]string{"title": "EveryAddon"}, filepath.Join("/path/to/EveryAddon", file_name))
			assert.Equal(t, mapset.NewSet(expected), toc.GameTrackIDSet, file_name)
		}
	}
}

// github-wow-addon-catalogue `INTERFACE_RANGES` and `INTERFACE_SUBRANGES`.
func Test_producers__interface_ranges(t *testing.T) {
	cases := map[int]GameTrackID{
		1_00_00: GAMETRACK_CLASSIC, 1_59_99: GAMETRACK_CLASSIC,
		1_60_00: GAMETRACK_FOREVER, 1_99_99: GAMETRACK_FOREVER,
		2_00_00: GAMETRACK_CLASSIC_TBC, 3_00_00: GAMETRACK_CLASSIC_WOTLK,
		4_00_00: GAMETRACK_CLASSIC_CATA, 5_00_00: GAMETRACK_CLASSIC_MISTS,
		6_00_00: GAMETRACK_RETAIL, 7_00_00: GAMETRACK_RETAIL, 8_00_00: GAMETRACK_RETAIL, 9_00_00: GAMETRACK_RETAIL,
		10_00_00: GAMETRACK_RETAIL, 11_00_00: GAMETRACK_RETAIL, 12_00_00: GAMETRACK_RETAIL,
	}
	for iv, expected := range cases {
		actual, err := InterfaceVersionToGameTrack(iv)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual, iv)
	}
}

// strongbox-catalogue-builder-go `AllGameTracks`, the values a catalogue's
// `game-track-list` may hold.
func Test_producers__catalogue_game_tracks(t *testing.T) {
	builder_game_tracks := mapset.NewSet[GameTrackID](
		"retail", "classic", "forever", "classic-tbc", "classic-wotlk", "classic-cata", "classic-mists",
	)
	assert.Equal(t, builder_game_tracks, SUPPORTED_GAME_TRACKS)
}
