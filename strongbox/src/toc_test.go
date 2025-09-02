package strongbox

import (
	"path/filepath"
	"testing"

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
		Label:                          "EveryAddon 1.2.3",
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
