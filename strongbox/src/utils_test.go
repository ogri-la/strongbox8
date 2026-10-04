package strongbox

import (
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/assert"
)

// cases are ported from the Clojure implementation's `guess-game-track` test.
var guess_game_track_cases = []struct {
	given    string
	expected GameTrackID
}{
	// nothing to go on
	{"", ""},
	{"foo", ""},
	{"1.2.3", ""},
	{"Addon-v1.2.3.zip", ""},

	// classic-wotlk
	{"wotlk", GAMETRACK_CLASSIC_WOTLK},
	{"wrath", GAMETRACK_CLASSIC_WOTLK},
	{"classic-wotlk", GAMETRACK_CLASSIC_WOTLK},
	{"classic-wrath", GAMETRACK_CLASSIC_WOTLK},
	{"classic_wotlk", GAMETRACK_CLASSIC_WOTLK},
	{"classic_wrath", GAMETRACK_CLASSIC_WOTLK},
	{"classic-wotlk.no-lib", GAMETRACK_CLASSIC_WOTLK},
	{"1.2.3-classic-wotlk", GAMETRACK_CLASSIC_WOTLK},
	{"1.2.3-classic-wotlk-no-lib", GAMETRACK_CLASSIC_WOTLK},
	{"1.2.3-classic-wotlk.no-lib", GAMETRACK_CLASSIC_WOTLK},
	{"1.2.3_classic_wotlk_no-lib", GAMETRACK_CLASSIC_WOTLK},
	{"ShestakUI-1.6.2-wrath.zip", GAMETRACK_CLASSIC_WOTLK},

	// classic-tbc
	{"bcc", GAMETRACK_CLASSIC_TBC},
	{"classic-tbc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3-classic-tbc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3-classic-tbc-no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic-tbc-no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic-tbc.no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic_tbc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3_classic_tbc_no-lib", GAMETRACK_CLASSIC_TBC},
	{"LunaUnitFrames-bcc-4.330.zip", GAMETRACK_CLASSIC_TBC},

	// classic-tbc, edge cases
	{"beta-tbc", GAMETRACK_CLASSIC_TBC},
	{"beta-bc", GAMETRACK_CLASSIC_TBC},
	{"beta_tbc", GAMETRACK_CLASSIC_TBC},
	{"beta bc", GAMETRACK_CLASSIC_TBC},
	{"beta (tbc)", GAMETRACK_CLASSIC_TBC},
	{"beta (bc)", GAMETRACK_CLASSIC_TBC},
	{"beta (bcc)", GAMETRACK_CLASSIC_TBC},
	{"beta (tbc) 2.13", GAMETRACK_CLASSIC_TBC},
	{"beta (bcc) 3.24", GAMETRACK_CLASSIC_TBC},

	// 2021-06-02: '-bcc' was adopted by the BigWigs packager.
	{"WeakAuras-3.4.2-bcc.zip", GAMETRACK_CLASSIC_TBC},
	{"classic-bcc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3-classic-bcc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3-classic-bcc-no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic-bcc-no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic-bcc.no-lib", GAMETRACK_CLASSIC_TBC},
	{"classic_bcc", GAMETRACK_CLASSIC_TBC},
	{"1.2.3_classic_bcc_no-lib", GAMETRACK_CLASSIC_TBC},

	// classic-cata
	{"cata", GAMETRACK_CLASSIC_CATA},
	{"classic-cata", GAMETRACK_CLASSIC_CATA},

	// classic
	{"classic", GAMETRACK_CLASSIC},
	{"vanilla", GAMETRACK_CLASSIC},
	{"1.2.3-classic", GAMETRACK_CLASSIC},
	{"1.2.3-classic-no-lib", GAMETRACK_CLASSIC},
	{"classic-no-lib", GAMETRACK_CLASSIC},
	{"classic.no-lib", GAMETRACK_CLASSIC},
	{"1.2.3_classic_no-lib", GAMETRACK_CLASSIC},

	// retail
	{"retail", GAMETRACK_RETAIL},
	{"mainline", GAMETRACK_RETAIL},
	{"1.2.3-retail", GAMETRACK_RETAIL},
	{"1.2.3-retail-no-lib", GAMETRACK_RETAIL},
	{"retail-no-lib", GAMETRACK_RETAIL},
	{"retail.no-lib", GAMETRACK_RETAIL},
	{"1.2.3_retail_no-lib", GAMETRACK_RETAIL},

	// case insensitivity
	{"Mainline", GAMETRACK_RETAIL},
	{"Retail", GAMETRACK_RETAIL},
	{"Classic", GAMETRACK_CLASSIC},
	{"Vanilla", GAMETRACK_CLASSIC},
	{"Classic-TBC", GAMETRACK_CLASSIC_TBC},

	// priority: most specific game track wins
	{"retail-classic-tbc-classic-wotlk", GAMETRACK_CLASSIC_WOTLK},
	{"retail-classic-tbc-classic", GAMETRACK_CLASSIC_TBC},
	{"retail-classic-classic-tbc", GAMETRACK_CLASSIC_TBC},
	{"classic-classic-tbc", GAMETRACK_CLASSIC_TBC},
	{"retail-classic", GAMETRACK_CLASSIC},
}

func TestGuessGameTrack(t *testing.T) {
	for _, c := range guess_game_track_cases {
		assert.Equal(t, c.expected, GuessGameTrack(c.given), c.given)
	}
}

// property: any string guesses a supported game track or none.
func FuzzGuessGameTrack(f *testing.F) {
	for _, c := range guess_game_track_cases {
		f.Add(c.given)
	}
	f.Fuzz(func(t *testing.T, given string) {
		actual := GuessGameTrack(given)
		if actual != "" && !SUPPORTED_GAME_TRACKS.Contains(actual) {
			t.Errorf("unsupported game track %q guessed from %q", actual, given)
		}
	})
}

// 'cata' and 'standard' match only as delimited words, so a longer word containing them
// does not.
func TestGuessGameTrack__cata_and_standard(t *testing.T) {
	var cases = []struct {
		given    string
		expected GameTrackID
	}{
		// cata
		{"cata.no-lib", GAMETRACK_CLASSIC_CATA},
		{"1.2.3-cata", GAMETRACK_CLASSIC_CATA},
		{"1.2.3_cata", GAMETRACK_CLASSIC_CATA},
		{"1.2.3.cata", GAMETRACK_CLASSIC_CATA},
		{"1.2.3-cata-no-lib", GAMETRACK_CLASSIC_CATA},
		{"1.2.3.cata.no-lib", GAMETRACK_CLASSIC_CATA},
		{"1.2.3_cata.no_lib", GAMETRACK_CLASSIC_CATA},
		{"WeakAuras-5.0.1-cata.zip", GAMETRACK_CLASSIC_CATA},
		{"Addon-1.2.3-classic-cata.zip", GAMETRACK_CLASSIC_CATA},
		{"Addon-1.2.3-CATA.zip", GAMETRACK_CLASSIC_CATA},

		// cata is checked before wotlk
		{"retail-classic-tbc-classic-wotlk-cata", GAMETRACK_CLASSIC_CATA},

		// cata inside a longer word
		{"Catalyst-1.0.zip", ""},
		{"catalogue", ""},
		{"Addon-1.2.3-catapult.zip", ""},

		// standard
		{"standard", GAMETRACK_RETAIL},
		{"1.2.3-standard.zip", GAMETRACK_RETAIL},
		{"Addon_Standard.zip", GAMETRACK_RETAIL},

		// standard inside a longer word
		{"nonstandard-1.0.zip", ""},
		{"Standardised-1.0.zip", ""},

		// a game track named elsewhere wins over standard
		{"1.2.3-standard-classic.zip", GAMETRACK_CLASSIC},

		// mists is not a supported game track
		{"mists", ""},
		{"1.2.3-mists", ""},
		{"Addon-1.2.3-mists.zip", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, GuessGameTrack(c.given), c.given)
	}
}

func TestIsBeforeClassic(t *testing.T) {
	// note: classic was "2019-08-26T00:00:00Z"
	var cases = []struct {
		given    time.Time
		expected bool
	}{
		// one second before
		{time.Date(2019, 8, 25, 23, 59, 59, 0, time.UTC), true},
		// exactly the same is not _before_
		{WOWClassicReleaseDate(), false},
		// one second after
		{time.Date(2019, 8, 26, 0, 0, 1, 0, time.UTC), false},
		// much later
		{time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC), false},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, IsBeforeClassic(c.given), i)
	}
}

func Test_parse_interface_version(t *testing.T) {
	var cases = []struct {
		given    int
		expected [3]int
	}{
		{10000, [3]int{1, 0, 0}},
		{11507, [3]int{1, 15, 7}},
		{16001, [3]int{1, 60, 1}},
		{110002, [3]int{11, 0, 2}},
		{999999, [3]int{99, 99, 99}},
	}
	for _, c := range cases {
		major, minor, patch, err := parse_interface_version(c.given)
		assert.Nil(t, err, c.given)
		assert.Equal(t, c.expected, [3]int{major, minor, patch}, c.given)
	}

	for _, given := range []int{9999, 1000000} {
		_, _, _, err := parse_interface_version(given)
		assert.NotNil(t, err, given)
	}
}

// cases are ported from the Clojure implementation's `format-interface-version` test.
// Go receives interface versions as integers, so its leading-zero cases ("00304") arrive
// as values below the 5 digit range and are invalid.
func TestInterfaceVersionToGameVersion(t *testing.T) {
	var cases = []struct {
		given    int
		expected string
	}{
		{10000, "1.0.0"},
		{20001, "2.0.1"},
		{30002, "3.0.2"},

		// six digits
		{100000, "10.0.0"},
		{100002, "10.0.2"},
		{100102, "10.1.2"},
		{110000, "11.0.0"},
		{110002, "11.0.2"},
		{120000, "12.0.0"},
		{200102, "20.1.2"},
		{300102, "30.1.2"},
		{101010, "10.10.10"},
		{999999, "99.99.99"},

		// two digit minor and patch versions
		{10100, "1.1.0"},
		{10123, "1.1.23"},
		{11302, "1.13.2"},
		{11507, "1.15.7"},
		{16001, "1.60.1"},
	}
	for _, c := range cases {
		actual, err := InterfaceVersionToGameVersion(c.given)
		assert.Nil(t, err, c.given)
		assert.Equal(t, c.expected, actual, c.given)
	}
}

func TestInterfaceVersionToGameVersion__invalid(t *testing.T) {
	given_list := []int{-10000, 0, 304, 1000, 1234, 9999, 1000000, 1234567}
	for _, given := range given_list {
		actual, err := InterfaceVersionToGameVersion(given)
		assert.NotNil(t, err, given)
		assert.Equal(t, "", actual, given)
	}
}

func TestInterfaceVersionToGameTrack(t *testing.T) {
	var cases = []struct {
		given    int
		expected GameTrackID
	}{
		{10000, GAMETRACK_CLASSIC},
		{11503, GAMETRACK_CLASSIC},
		{15999, GAMETRACK_CLASSIC},

		// forever, not supported yet
		{16000, ""},
		{16001, ""},
		{19999, ""},

		{20000, GAMETRACK_CLASSIC_TBC},
		{20504, GAMETRACK_CLASSIC_TBC},
		{30403, GAMETRACK_CLASSIC_WOTLK},
		{39999, GAMETRACK_CLASSIC_WOTLK},
		{40000, GAMETRACK_CLASSIC_CATA},
		{40400, GAMETRACK_CLASSIC_CATA},
		{49999, GAMETRACK_CLASSIC_CATA},

		// mists, not supported yet
		{50000, ""},
		{50500, ""},
		{59999, ""},

		{60000, GAMETRACK_RETAIL},
		{70000, GAMETRACK_RETAIL},
		{90207, GAMETRACK_RETAIL},
		{100105, GAMETRACK_RETAIL},
		{110002, GAMETRACK_RETAIL},
		{999999, GAMETRACK_RETAIL},
	}
	for _, c := range cases {
		actual, err := InterfaceVersionToGameTrack(c.given)
		assert.Nil(t, err, c.given)
		assert.Equal(t, c.expected, actual, c.given)
	}
}

func TestInterfaceVersionToGameTrack__invalid(t *testing.T) {
	given_list := []int{-10000, 0, 1234, 9999, 1000000, 1234567}
	for _, given := range given_list {
		actual, err := InterfaceVersionToGameTrack(given)
		assert.NotNil(t, err, given)
		assert.Equal(t, GameTrackID(""), actual, given)
	}
}

func TestRemoveEscapeSequences(t *testing.T) {
	var cases = []struct {
		given    string
		expected string
	}{
		{"", ""},
		{"foo", "foo"},
		// unknown prefix is preserved (no match)
		{"|b01234567", "|b01234567"},
		// correct prefix but too short so sequence is preserved (no match)
		{"|c0123456", "|c0123456"},
		// reset sequence is removed
		{"|r", ""},
		// might have unintended consequences
		{"kool|raid", "koolaid"},
		// real life examples
		{"|cff1784d1ElvUI|r |cff00c0faBenikUI|r |cfd9b9b9bClassic|r", "ElvUI BenikUI Classic"},
		{"Archaeo Helper |cffff7d0aby Biasha", "Archaeo Helper by Biasha"},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, RemoveEscapeSequences(c.given), i)
	}
}

// property: for any integer, `InterfaceVersionToGameTrack` returns an error exactly when
// the integer is out of range, and otherwise returns a supported game track or none.
func TestInterfaceVersionToGameTrack__property(t *testing.T) {
	any_int := func(given int) bool {
		actual, err := InterfaceVersionToGameTrack(given)
		in_range := given >= INTERFACE_VERSION_MIN && given <= INTERFACE_VERSION_MAX
		if !in_range {
			return err != nil && actual == ""
		}
		return err == nil && (actual == "" || SUPPORTED_GAME_TRACKS.Contains(actual))
	}
	assert.Nil(t, quick.Check(any_int, nil))

	// most random integers are out of range, so check valid interface versions directly.
	valid_int := func(n uint32) bool {
		return any_int(INTERFACE_VERSION_MIN + int(n%(INTERFACE_VERSION_MAX-INTERFACE_VERSION_MIN+1)))
	}
	assert.Nil(t, quick.Check(valid_int, &quick.Config{MaxCount: 10000}))
}

// property: the parts of a valid interface version recombine to the same interface version.
func Test_parse_interface_version__property(t *testing.T) {
	round_trip := func(n uint32) bool {
		given := INTERFACE_VERSION_MIN + int(n%(INTERFACE_VERSION_MAX-INTERFACE_VERSION_MIN+1))
		major, minor, patch, err := parse_interface_version(given)
		return err == nil && major*10000+minor*100+patch == given
	}
	assert.Nil(t, quick.Check(round_trip, &quick.Config{MaxCount: 10000}))
}
