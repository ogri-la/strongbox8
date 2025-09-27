package strongbox

import (
	"errors"
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
