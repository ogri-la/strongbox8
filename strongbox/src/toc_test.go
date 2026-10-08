package strongbox

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func Test_find_toc_files(t *testing.T) {
	path_list, err := find_toc_files(filepath.Join(test_fixture_everyaddon_maximal, "EveryAddon"))
	assert.Nil(t, err)
	expected := 5 // retail, vanilla, tbc, wrath, cata
	assert.Equal(t, expected, len(path_list))

	path_list2, err := find_toc_files(filepath.Join(test_fixture_everyaddon_maximal, "EveryAddon_Config"))
	assert.Nil(t, err)
	expected = 1 // toc with multiple interfaces
	assert.Equal(t, expected, len(path_list2))
}

func TestReadAddonTOCFile(t *testing.T) {
	fixture := filepath.Join(test_fixture_everyaddon_minimal, "EveryAddon", "EveryAddon.toc")
	expected := map[string]string{
		"author":         "John Doe",
		"defaultstate":   "enabled",
		"description":    "Does what no other addon does, slightly differently",
		"interface":      "70000",
		"savedvariables": "EveryAddon_Foo,EveryAddon_Bar",
		"title":          "EveryAddon 1.2.3",
		"version":        "1.2.3",
	}
	actual, err := ReadAddonTOCFile(fixture)
	assert.FileExists(t, fixture)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseTOCFile(t *testing.T) {
	fixture := filepath.Join(test_fixture_everyaddon_minimal, "EveryAddon", "EveryAddon.toc")
	expected := TOC{
		Name:                           "everyaddon",
		Label:                          "EveryAddon",
		Title:                          "EveryAddon 1.2.3",
		Notes:                          "Does what no other addon does, slightly differently",
		URL:                            "file://" + fixture,
		DirName:                        "EveryAddon",
		FileName:                       "EveryAddon.toc",
		InterfaceVersionSet:            mapset.NewSet(70000),
		InstalledVersion:               "1.2.3",
		SourceMapList:                  []SourceMap{},
		InterfaceVersionGameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL),
		FileNameGameTrackID:            "", // not able to guess
		GameTrackIDSet:                 mapset.NewSet(GAMETRACK_RETAIL),
	}
	actual, err := ParseTOCFile(fixture)
	assert.FileExists(t, fixture)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// a .toc's game tracks are the union of its interface versions' game tracks and the game
// track guessed from its file name.
// an invalid interface version contributes nothing.
func Test_coerce_toc_data__game_tracks(t *testing.T) {
	var cases = []struct {
		file_name         string
		interface_version string
		expected          mapset.Set[GameTrackID]
	}{
		{"EveryAddon.toc", "110002, 50500, 40400, 11503", mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC_MISTS, GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC)},
		{"EveryAddon.toc", "1234", mapset.NewSet[GameTrackID]()},
		{"EveryAddon.toc", "50500", mapset.NewSet(GAMETRACK_CLASSIC_MISTS)},
		{"EveryAddon.toc", "16000", mapset.NewSet(GAMETRACK_FOREVER)},
		{"EveryAddon_Cata.toc", "1234", mapset.NewSet(GAMETRACK_CLASSIC_CATA)},
		{"EveryAddon_Mists.toc", "1234", mapset.NewSet(GAMETRACK_CLASSIC_MISTS)},
		{"EveryAddon-Forever.toc", "1234", mapset.NewSet(GAMETRACK_FOREVER)},
		{"EveryAddon-Camelot.toc", "1234", mapset.NewSet(GAMETRACK_FOREVER)},
		{"EveryAddon-WOTLKC.toc", "1234", mapset.NewSet(GAMETRACK_CLASSIC_WOTLK)},
		{"EveryAddon.toc", "1234, 110002", mapset.NewSet(GAMETRACK_RETAIL)},
	}
	for _, c := range cases {
		given := map[string]string{"title": "EveryAddon", "interface": c.interface_version}
		actual := coerce_toc_data(given, filepath.Join("/path/to/EveryAddon", c.file_name))
		assert.Equal(t, c.expected, actual.GameTrackIDSet, c.file_name+" "+c.interface_version)
		assert.False(t, actual.InterfaceVersionGameTrackIDSet.Contains(""), c.interface_version)
	}
}

// property: whatever its interface versions and file name, a .toc's game tracks are all
// supported game tracks.
func Test_coerce_toc_data__property(t *testing.T) {
	suffix_list := []string{"", "_Mainline", "_Vanilla", "_Classic", "_TBC", "_BCC", "_Wrath", "_Cata", "_Mists", "_Forever", "_Standard", "_Config"}
	supported := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		interface_list := []string{}
		for range r.Intn(5) {
			// covers invalid versions either side of the valid range, and non-numbers.
			interface_list = append(interface_list, random_pick(r, []string{
				fmt.Sprint(r.Intn(1100000)), fmt.Sprint(-r.Intn(100)), "foo", "",
			}))
		}
		kvs := map[string]string{"title": "EveryAddon", "interface": strings.Join(interface_list, ", ")}
		file_name := "EveryAddon" + random_pick(r, suffix_list) + ".toc"
		toc := coerce_toc_data(kvs, filepath.Join("/path/to/EveryAddon", file_name))
		for game_track := range toc.GameTrackIDSet.Iter() {
			if !SUPPORTED_GAME_TRACKS.Contains(game_track) {
				return false
			}
		}
		return true
	}
	assert.Nil(t, quick.Check(supported, &quick.Config{MaxCount: 2000}))
}
