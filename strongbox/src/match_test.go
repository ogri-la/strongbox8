package strongbox

// clj: `core_test.clj/db-match-installed-addon-list-with-catalogue*`, `moosh-addons`,
// `db-addon-by-source-and-source-id`

import (
	"bw/http_utils"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the installed addons generated in a fresh addons dir by `spec_list`.
func installed_addons_from(t *testing.T, spec_list ...test_addon_spec) []Addon {
	t.Helper()
	ad := test_addons_dir(t)
	for _, spec := range spec_list {
		test_gen_addon(t, ad.Path, spec)
	}
	addon_list, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	return addon_list
}

func Test_match_addon_list__rule_order(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"})
	db := []CatalogueAddon{
		{Source: SOURCE_WOWI, SourceID: "1", Name: "everyaddon", Label: "EveryAddon (wowi)"},
		{Source: SOURCE_GITHUB, SourceID: "a/b", Name: "something-else", Label: "EveryAddon (github)"},
	}
	actual, matched := match_addon_list(db, addon_list)
	assert.Equal(t, 1, matched)
	assert.Equal(t, "EveryAddon (github)", actual[0].Label, "source and source ID beats name")
}

func Test_match_addon_list__fallbacks(t *testing.T) {
	cases := []struct {
		spec     test_addon_spec
		db       []CatalogueAddon
		expected string
	}{
		// name: a strongbox 7 nfo with a name but a source the catalogue doesn't have
		{test_addon_spec{DirList: []string{"EveryAddon"}, Title: "Whatever"},
			[]CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "whatever", Label: "Matched By Name"}}, "Matched By Name"},
		// label
		{test_addon_spec{DirList: []string{"DirName"}, Title: "Nice Label"},
			[]CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "other", Label: "Nice Label"}}, "Nice Label"},
		// directory name against label
		{test_addon_spec{DirList: []string{"AdiBags"}, Title: "Adi Bags Unmatched"},
			[]CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "other", Label: "AdiBags"}}, "AdiBags"},
	}
	for _, c := range cases {
		addon_list := installed_addons_from(t, c.spec)
		actual, matched := match_addon_list(c.db, addon_list)
		assert.Equal(t, 1, matched, c.expected)
		assert.Equal(t, c.expected, actual[0].Label)
	}
}

// strongbox 7 matched an installed source against a catalogue name. it is not a rule.
func Test_match_addon_list__source_is_not_a_name(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"Unrelated"}, Title: "Unrelated"})
	addon_list[0].Source = "github"
	db := []CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "github", Label: "Github"}}
	_, matched := match_addon_list(db, addon_list)
	assert.Equal(t, 0, matched)
}

// when a rule matches several catalogue addons the first in catalogue order wins.
func Test_match_addon_list__first_in_catalogue_order(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon"})
	db := []CatalogueAddon{
		{Source: "github", SourceID: "first/one", Name: "everyaddon", Label: "First"},
		{Source: "wowinterface", SourceID: "2", Name: "everyaddon", Label: "Second"},
	}
	actual, _ := match_addon_list(db, addon_list)
	assert.Equal(t, "First", actual[0].Label)
}

// clj: `--ignored-addons-are-skipped`
func Test_match_addon_list__ignored_not_matched(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon"})
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon", ".git"), 0o755)
	addon_list, _ := LoadAllInstalledAddons(ad)
	db := []CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "everyaddon", Label: "Matched"}}
	actual, matched := match_addon_list(db, addon_list)
	assert.Equal(t, 0, matched)
	assert.Equal(t, "EveryAddon", actual[0].Label)
	assert.Nil(t, actual[0].CatalogueAddon)
}

// clj: `moosh-addons`
func Test_match_addon_list__matched_details(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon"})
	addon_list[0].Primary.TOCMap["EveryAddon.toc"] = func() TOC { toc := addon_list[0].Primary.TOCMap["EveryAddon.toc"]; toc.Notes = "toc notes"; return toc }()
	db := []CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "everyaddon", Label: "Every Addon", URL: "https://github.com/x/y", TagList: []string{"bags"}}}
	actual, _ := match_addon_list(db, addon_list)
	assert.Equal(t, "Every Addon", actual[0].Label)
	assert.Equal(t, "https://github.com/x/y", actual[0].URL)
	assert.Equal(t, []string{"bags"}, actual[0].Tags)
	assert.Equal(t, "toc notes", actual[0].Description, "an empty catalogue description keeps the .toc's")
	assert.Equal(t, SOURCE_GITHUB, actual[0].Source)
}

// the source the addon was installed from wins over the catalogue's.
func Test_match_addon_list__nfo_source_kept(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "321"})
	db := []CatalogueAddon{{Source: "github", SourceID: "x/y", Name: "everyaddon", Label: "EveryAddon"}}
	actual, matched := match_addon_list(db, addon_list)
	assert.Equal(t, 1, matched)
	assert.Equal(t, SOURCE_WOWI, actual[0].Source)
	assert.Equal(t, "321", actual[0].SourceID)
}

func Test_Reconcile(t *testing.T) {
	path := test_dir(t, "AddOns")
	test_gen_addon(t, path, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon"})
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(path)}
	settings.Preferences.SelectedAddonsDir = path
	app := test_app_with_settings(t, settings)
	app.HTTPClient.Transport = http_utils.NewFixtureTransport(map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: test_fixture_bytes("catalogues/catalogue.json")},
	})
	assert.Error(t, Reconcile(app), "no catalogue loaded yet")

	assert.NoError(t, LoadCatalogue(app))
	ids_before := len(app.GetResultList())
	assert.NoError(t, Reconcile(app))
	actual := addon_results(app)[0].Item.(Addon)
	assert.NotNil(t, actual.CatalogueAddon)
	assert.Equal(t, "https://github.com/ogri-la/everyaddon", actual.URL)
	assert.Equal(t, ids_before, len(app.GetResultList()), "addons are updated in place")
}
