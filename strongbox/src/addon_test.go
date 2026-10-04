package strongbox

import (
	"errors"
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func Test_load_installed_addon__empty_dir(t *testing.T) {
	empty_addon_dir := t.TempDir()
	_, err := load_installed_addon(empty_addon_dir)
	assert.NotNil(t, err)
}

func Test_determine_primary_subdir(t *testing.T) {
	var cases = []struct {
		given    mapset.Set[string]
		expected string
	}{
		{mapset.NewSet("Foo"), "Foo"},
		{mapset.NewSet("Foo", "FooBar", "FooBarBaz"), "Foo"},
		{mapset.NewSet("FooBarBaz", "FooBar", "Foo"), "Foo"}, // maps have no order
	}
	for _, c := range cases {
		actual, err := determine_primary_subdir(c.given)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func Test_determine_primary_subdir__error_cases(t *testing.T) {
	var cases = []struct {
		given    mapset.Set[string]
		expected error
	}{
		{mapset.NewSet[string](), errors.New("empty set")},
		{mapset.NewSet("Foo", "Bar"), errors.New("no common directory prefix")},
		{mapset.NewSet("Foo", "Bar", "Baz"), errors.New("no common directory prefix")},
	}
	for _, c := range cases {
		_, err := determine_primary_subdir(c.given)
		assert.NotNil(t, err)
		assert.Equal(t, c.expected, err)
	}
}

// a basic set of data can create a valid Addon.
// (use to test expectations about derived values)
func TestMakeAddon__no_nfo_no_catalogue_match_no_source_update(t *testing.T) {
	addons_dir := AddonsDir{
		GameTrackID: GAMETRACK_RETAIL,
		Strict:      true,
	}

	nfo := NFO{}

	toc := NewTOC()
	toc.Notes = "TOC notes"
	toc.GameTrackIDSet = mapset.NewSet(GAMETRACK_RETAIL)
	toc.InterfaceVersionSet = mapset.NewSet(100000)

	installed_addon := NewInstalledAddon()
	installed_addon.TOCMap = map[PathToFile]TOC{"EveryAddon.toc": toc}
	installed_addon.NFOList = []NFO{nfo}

	primary_installed_addon := installed_addon
	source_update_list := NewSourceUpdate()

	expected := Addon{
		InstalledAddonGroup: []InstalledAddon{installed_addon},
		CatalogueAddon:      nil,
		SourceUpdateList:    []SourceUpdate{source_update_list},
		AddonsDir:           &addons_dir,
		Primary:             primary_installed_addon,
		NFO:                 &nfo,

		// ---

		Description:      "TOC notes",
		TOC:              &toc,
		Tags:             nil, // No catalogue match means no tags
		InterfaceVersion: "100000",
		GameVersion:      "10.0.0",
	}

	actual := MakeAddon(addons_dir, []InstalledAddon{installed_addon}, primary_installed_addon, &nfo, nil, []SourceUpdate{source_update_list})
	assert.Equal(t, expected, actual)
}

// an Addon can be created from a CatalogueAddon
// (use to test expectations about derived values)
func TestMakeAddonFromCatalogueAddon(t *testing.T) {
	ad := AddonsDir{
		GameTrackID: GAMETRACK_RETAIL,
		Strict:      true,
	}
	ca := test_fixture_catalogue.AddonSummaryList[0]
	nfo := NFO{
		GroupID: "https://github.com/ogri-la/everyaddon",
	}
	sul := []SourceUpdate{}
	expected := Addon{
		InstalledAddonGroup: []InstalledAddon{},
		CatalogueAddon:      &ca,
		SourceUpdateList:    sul,
		AddonsDir:           &ad,
		Primary:             InstalledAddon{},
		NFO:                 &nfo,

		// ---

		Source:      ca.Source,
		SourceID:    string(ca.SourceID),
		Name:        ca.Name,
		Label:       ca.Label,
		Description: ca.Description,
		URL:         ca.URL,
		Tags:        ca.TagList,
		Updated:     ca.UpdatedDate,
	}

	actual := MakeAddonFromCatalogueAddon(ad, ca, sul)
	assert.Equal(t, expected, actual)

}

// returns a .toc at `path` supporting `game_track_list`, for .toc selection tests.
func dummy_toc(path PathToFile, game_track_list ...GameTrackID) TOC {
	toc := NewTOC()
	toc.FileName = filepath.Base(path)
	toc.GameTrackIDSet = mapset.NewSet(game_track_list...)
	return toc
}

// returns an installed addon holding `toc_list`, keyed by file name.
func dummy_installed_addon(toc_list ...TOC) InstalledAddon {
	ia := NewInstalledAddon()
	for _, toc := range toc_list {
		ia.TOCMap[toc.FileName] = toc
	}
	return ia
}

func Test_make_addon__find_toc(t *testing.T) {
	retail := dummy_toc("EveryAddon_Mainline.toc", GAMETRACK_RETAIL)
	classic := dummy_toc("EveryAddon_Vanilla.toc", GAMETRACK_CLASSIC)
	classic_too := dummy_toc("EveryAddon_Classic.toc", GAMETRACK_CLASSIC)
	tbc := dummy_toc("EveryAddon_TBC.toc", GAMETRACK_CLASSIC_TBC)
	cata := dummy_toc("EveryAddon_Cata.toc", GAMETRACK_CLASSIC_CATA)
	multi := dummy_toc("EveryAddon.toc", GAMETRACK_RETAIL, GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC)

	var cases = []struct {
		desc       string
		game_track GameTrackID
		strict     bool
		given      InstalledAddon
		expected   *TOC
	}{
		{"strict match", GAMETRACK_CLASSIC, true, dummy_installed_addon(retail, classic), &classic},
		{"strict, most specific wins", GAMETRACK_CLASSIC_CATA, true, dummy_installed_addon(multi, cata), &cata},
		{"strict, equally specific, lowest path wins", GAMETRACK_CLASSIC, true, dummy_installed_addon(classic, classic_too), &classic_too},
		{"strict, no match", GAMETRACK_CLASSIC, true, dummy_installed_addon(retail), nil},
		{"relaxed, most preferred game track wins", GAMETRACK_CLASSIC_WOTLK, false, dummy_installed_addon(cata, tbc, classic), &cata},
		{"relaxed, exact match", GAMETRACK_RETAIL, false, dummy_installed_addon(retail, classic), &retail},
		{"relaxed, equally specific, lowest path wins", GAMETRACK_CLASSIC, false, dummy_installed_addon(classic, classic_too), &classic_too},
		{"relaxed, no toc", GAMETRACK_RETAIL, false, dummy_installed_addon(), nil},
	}
	for _, c := range cases {
		// map iteration order varies between runs, so repeat to catch a result that
		// depends on it.
		for range 50 {
			actual := _make_addon__find_toc(c.game_track, c.given, c.strict)
			assert.Equal(t, c.expected, actual, c.desc)
		}
	}
}

func Test_best_toc(t *testing.T) {
	cata := dummy_toc("EveryAddon_Cata.toc", GAMETRACK_CLASSIC_CATA)
	multi := dummy_toc("EveryAddon.toc", GAMETRACK_RETAIL, GAMETRACK_CLASSIC_CATA)
	classic := dummy_toc("EveryAddon_Vanilla.toc", GAMETRACK_CLASSIC)
	classic_too := dummy_toc("EveryAddon_Classic.toc", GAMETRACK_CLASSIC)

	var cases = []struct {
		desc       string
		given      InstalledAddon
		game_track GameTrackID
		expected   *TOC
	}{
		{"no tocs", dummy_installed_addon(), GAMETRACK_RETAIL, nil},
		{"no match", dummy_installed_addon(cata), GAMETRACK_RETAIL, nil},
		{"single match", dummy_installed_addon(cata, multi), GAMETRACK_RETAIL, &multi},
		{"fewest game tracks wins", dummy_installed_addon(multi, cata), GAMETRACK_CLASSIC_CATA, &cata},
		{"lowest path breaks a tie", dummy_installed_addon(classic, classic_too), GAMETRACK_CLASSIC, &classic_too},
	}
	for _, c := range cases {
		for range 50 {
			assert.Equal(t, c.expected, best_toc(c.given.TOCMap, c.game_track), c.desc)
		}
	}
}
