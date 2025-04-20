package strongbox

import (
	"log/slog"
	"os"
	"path/filepath"
)

// the path that is returned is relative to the directory the test is
// being executed in.
func test_fixture_path(fixture_name string) string {
	return filepath.Join("testdata", fixture_name)
}

// returns the contents of `fixture_name` as a byte slice
func test_fixture_bytes(fixture_name string) []byte {
	bytes, err := os.ReadFile(test_fixture_path(fixture_name))
	if err != nil {
		slog.Error("failed to read test fixture", "fixture", fixture_name, "error", err.Error())
		panic("programming error")
	}
	return bytes
}

// returns the contents of `fixture_name` as a string.
func test_fixture_string(fixture_name string) string {
	return string(test_fixture_bytes(fixture_name))
}

//

// absolutely minimal addon .zip file.
// should contain just enough to be installed and that's it.
// probably won't change much over time.
var test_fixture_everyaddon_minimal_zip = test_fixture_path("zipfiles/everyaddon--1-2-3.zip")

// slightest tweak to the minimal fixture that can be used as an update
var test_fixture_everyaddon_minimal_update_zip = test_fixture_path("zipfiles/everyaddon--1-2-4.zip")

// path to the kitchen sink test fixture
var test_fixture_everyaddon_maximal = test_fixture_path("zipfiles/everyaddon--7-8-9")

// absolutely packed out addon .zip file
// should contain every property and every feature that might ever be seen.
// will probably change a lot over time.
var test_fixture_everyaddon_maximal_zip = test_fixture_path("zipfiles/everyaddon--7-8-9.zip")

// some addon 'EveryOtherAddon' that bundles a copy of EveryAddon,
// creating a mutual dependency if EveryAddon 1.2.3 is installed,
// but also entirely replacing EveryAddon 1.2.3.
var test_fixture_everyotheraddon_minimal_zip = test_fixture_path("zipfiles/everyotheraddon--2-3-4.zip")

// some addon 'EveryOtherAddon' that bundles a copy of EveryAddon,
// creating a mutual dependency if EveryAddon 7.8.9 is installed
var test_fixture_everyotheraddon_maximal_zip = test_fixture_path("zipfiles/everyotheraddon--2-3-4.zip")

// standard nfo file circa 7.0 with source-id fields as integers
var test_fixture_nfo_single_ints_json = test_fixture_bytes("nfofiles/single_with_ints.json")

// standard nfo file 8.0+ with source-id fields as strings only
var test_fixture_nfo_single_strs_json = test_fixture_bytes("nfofiles/single_with_strs.json")

// unmarshalled nfo file
var test_fixture_nfo_single = NFO{
	InstalledVersion:     "1.2.1",
	InstalledGameTrackID: GAMETRACK_RETAIL,
	Name:                 "EveryAddon",
	GroupID:              "https://foo.bar",
	Primary:              true,
	Source:               SOURCE_CURSEFORGE,
	SourceID:             "123", // string!
	SourceMapList: []SourceMap{
		{
			Source:   SOURCE_CURSEFORGE,
			SourceID: "123", // string!
		},
	},
}

var test_fixture_nfo_multi_mixed_json = test_fixture_bytes("nfofiles/multi_with_mixed.json")

// unmarshalled nfo file
var test_fixture_nfo_multi = []NFO{
	test_fixture_nfo_single,
	{
		InstalledVersion:     "2.3.2",
		InstalledGameTrackID: GAMETRACK_CLASSIC,
		Name:                 "EveryAddon",
		GroupID:              "https://bar.baz",
		Primary:              true,
		Source:               SOURCE_WOWI,
		SourceID:             "321", // string!
		SourceMapList: []SourceMap{
			{
				Source:   SOURCE_WOWI,
				SourceID: "321", // string!
			},
		},
	},
}
