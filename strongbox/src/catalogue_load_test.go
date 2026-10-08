package strongbox

// catalogue parsing, freshness, recovery and combining with the user catalogue.
// clj: `catalogue_test.clj`, `core_test.clj/re-download-catalogue-on-bad-data*`,
// `http-500-downloading-catalogue`, `add-user-addon!*`, `refresh-user-catalogue-item*`,
// `scheduled-user-catalogue-refresh`

import (
	"bw/core"
	"bw/http_utils"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// returns a catalogue file's contents holding `entries`, raw JSON objects.
func catalogue_json(entries ...string) string {
	return fmt.Sprintf(`{"spec": {"version": 2}, "datestamp": "2026-10-03", "total": %d, "addon-summary-list": [%s]}`,
		len(entries), strings.Join(entries, ","))
}

// returns a catalogue entry for `source`/`source_id` with the required fields only.
func catalogue_entry(source string, source_id string, label string) string {
	return fmt.Sprintf(`{"url": "https://example.org/%s", "name": "%s", "label": "%s", "updated-date": "2026-01-01T00:00:00Z",
		"source": "%s", "source-id": "%s", "game-track-list": ["retail"]}`, source_id, strings.ToLower(label), label, source, source_id)
}

func Test_parse_catalogue__fixtures(t *testing.T) {
	actual, issues, err := parse_catalogue(test_fixture_bytes("catalogues/catalogue.json"), CAT_SHORT)
	assert.NoError(t, err)
	assert.Empty(t, issues)
	assert.Len(t, actual.AddonSummaryList, 1)
	assert.Equal(t, "short", actual.Name)

	// a catalogue written by strongbox-catalogue-builder-go, with entries lacking tags and
	// downloads, and forever and mists game tracks
	b := test_fixture_bytes("catalogues/builder-short-catalogue.json")
	actual, issues, err = parse_catalogue(b, CAT_SHORT)
	assert.NoError(t, err)
	assert.Empty(t, issues)
	assert.Len(t, actual.AddonSummaryList, 42)
	game_tracks := map[GameTrackID]bool{}
	for _, ca := range actual.AddonSummaryList {
		assert.NotNil(t, ca.TagList)
		for _, gt := range ca.GameTrackIDList {
			game_tracks[gt] = true
		}
	}
	assert.True(t, game_tracks[GAMETRACK_FOREVER])
	assert.True(t, game_tracks[GAMETRACK_CLASSIC_MISTS])
}

// clj: `catalogue_test.clj/read-bad-catalogue`
func Test_parse_catalogue__invalid(t *testing.T) {
	cases := []string{
		``,
		`[]`,
		`{"spec": {"version": 1}, "datestamp": "2026-10-03", "total": 0, "addon-summary-list": []}`,
		`{"spec": {"version": 2}, "datestamp": "2026-10-03", "total": 5, "addon-summary-list": []}`,
		`{"spec": {"version": 2}, "datestamp": "not a date", "total": 0, "addon-summary-list": []}`,
		`{"spec": {"version": 2}, "datestamp": "2026-10-03", "addon-summary-list": []}`,
	}
	for _, given := range cases {
		_, _, err := parse_catalogue([]byte(given), CAT_SHORT)
		assert.Error(t, err, given)
	}
}

func Test_parse_catalogue__entries(t *testing.T) {
	long_description := strings.Repeat("é", 300)
	given := catalogue_json(
		catalogue_entry("github", "a/b", "Good"),
		`{"url": "x"}`, // missing fields
		`"not an object"`,
		catalogue_entry("curseforge", "1", "Dead"),
		catalogue_entry("tukui", "2", "AlsoDead"),
		catalogue_entry("github", "a/b", "Duplicate"),
		`{"url": "https://example.org/c", "name": "c", "label": "C", "updated-date": "2026-01-01T00:00:00Z", "source": "wowinterface",
		  "source-id": 3, "game-track-list": ["retail", "classic-bfa", "forever"], "description": "`+long_description+`"}`,
	)
	actual, issues, err := parse_catalogue([]byte(given), CAT_SHORT)
	assert.NoError(t, err)
	assert.Len(t, actual.AddonSummaryList, 2)
	assert.Equal(t, 2, actual.Total)
	assert.Equal(t, "Good", actual.AddonSummaryList[0].Label)
	assert.Equal(t, []string{}, actual.AddonSummaryList[0].TagList)
	assert.Equal(t, 0, actual.AddonSummaryList[0].DownloadCount)

	c := actual.AddonSummaryList[1]
	assert.Equal(t, FlexString("3"), c.SourceID)
	assert.Equal(t, []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_FOREVER}, c.GameTrackIDList)
	assert.Equal(t, CATALOGUE_DESCRIPTION_MAX, len([]rune(c.Description)))
	assert.NotEmpty(t, issues)
}

// an entry in both catalogues comes from the selected catalogue whole.
func Test_combine_catalogues(t *testing.T) {
	selected := Catalogue{AddonSummaryList: []CatalogueAddon{
		{Source: "github", SourceID: "a/b", Label: "AB"},
		{Source: "wowinterface", SourceID: "1", Label: "One"},
	}}
	user := Catalogue{AddonSummaryList: []CatalogueAddon{
		{Source: "github", SourceID: "a/b", Label: "AB (user)", Description: "user description"},
		{Source: "gitlab", SourceID: "g/h", Label: "GH"},
	}}
	actual := combine_catalogues(selected, user)
	assert.Equal(t, 3, actual.Total)
	assert.Equal(t, []string{"AB", "One", "GH"}, []string{actual.AddonSummaryList[0].Label, actual.AddonSummaryList[1].Label, actual.AddonSummaryList[2].Label})
	assert.Equal(t, "", actual.AddonSummaryList[0].Description, "no field comes from the user catalogue")
	assert.True(t, actual.AddonSummaryList[0].Starred())
	assert.False(t, actual.AddonSummaryList[1].Starred())
	assert.True(t, actual.AddonSummaryList[2].Starred())
}

// --- downloading and loading

// returns an app with default settings whose HTTP requests are answered by `routes`.
func test_app_with_routes(t *testing.T, routes map[string]http_utils.Fixture) (*core.App, *http_utils.FixtureTransport) {
	t.Helper()
	app := test_app_with_settings(t, NewSettings())
	ft := http_utils.NewFixtureTransport(routes)
	app.HTTPClient.Transport = ft
	return app, ft
}

func Test_LoadCatalogue(t *testing.T) {
	app, ft := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: test_fixture_bytes("catalogues/builder-short-catalogue.json")},
	})
	assert.NoError(t, LoadCatalogue(app))
	cat, ok := loaded_catalogue(app)
	assert.True(t, ok)
	assert.Len(t, cat.AddonSummaryList, 42)
	assert.Len(t, app.FilterResultListByNS(NS_CATALOGUE_ADDON), 42)
	assert.Equal(t, []string{CAT_SHORT.Source}, ft.Requested())

	// loading again replaces rather than adds
	assert.NoError(t, LoadCatalogue(app))
	assert.Len(t, app.FilterResultListByNS(NS_CATALOGUE_ADDON), 42)
}

func Test_LoadCatalogue__fresh_copy_reused(t *testing.T) {
	app, ft := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "AB")))},
	})
	assert.NoError(t, LoadCatalogue(app))
	assert.NoError(t, LoadCatalogue(app))
	assert.Len(t, ft.Requested(), 1)

	// a stale copy is downloaded again
	path := CataloguePath(app, "short")
	two_hours_ago := time.Now().Add(-2 * time.Hour)
	os.Chtimes(path, two_hours_ago, two_hours_ago)
	assert.NoError(t, LoadCatalogue(app))
	assert.Len(t, ft.Requested(), 2)
}

// clj: `core_test.clj/http-500-downloading-catalogue`
func Test_LoadCatalogue__server_error_keeps_old_copy(t *testing.T) {
	app, ft := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "AB")))},
	})
	assert.NoError(t, LoadCatalogue(app))
	path := CataloguePath(app, "short")
	before, _ := os.ReadFile(path)
	two_hours_ago := time.Now().Add(-2 * time.Hour)
	os.Chtimes(path, two_hours_ago, two_hours_ago)

	ft.Set(CAT_SHORT.Source, http_utils.Fixture{Status: 500, Body: []byte("oops")})
	assert.NoError(t, LoadCatalogue(app), "the old copy is still used")
	after, _ := os.ReadFile(path)
	assert.Equal(t, before, after)
}

func Test_LoadCatalogue__server_error_no_copy(t *testing.T) {
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Status: 500},
	})
	assert.Error(t, LoadCatalogue(app))
	assert.NoFileExists(t, CataloguePath(app, "short"))
	cat, ok := loaded_catalogue(app)
	assert.True(t, ok, "an empty catalogue is loaded so the user catalogue still works")
	assert.Empty(t, cat.AddonSummaryList)
}

// clj: `core_test.clj/re-download-catalogue-on-bad-data`
func Test_LoadCatalogue__corrupt_copy_downloaded_again(t *testing.T) {
	app, ft := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "AB")))},
	})
	path := CataloguePath(app, "short")
	core.MakeParents(path)
	os.WriteFile(path, []byte(""), 0o644) // fresh, but empty

	assert.NoError(t, LoadCatalogue(app))
	cat, _ := loaded_catalogue(app)
	assert.Len(t, cat.AddonSummaryList, 1)
	assert.Len(t, ft.Requested(), 1)
}

// clj: `core_test.clj/re-download-catalogue-on-bad-data-2`
func Test_LoadCatalogue__corrupt_twice(t *testing.T) {
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte("{not json")},
	})
	assert.Error(t, LoadCatalogue(app))
	cat, _ := loaded_catalogue(app)
	assert.Empty(t, cat.AddonSummaryList)
}

func Test_SwitchCatalogue(t *testing.T) {
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source:  {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "Short")))},
		CAT_GITHUB.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "c/d", "Github1"), catalogue_entry("github", "e/f", "Github2")))},
	})
	assert.NoError(t, LoadCatalogue(app))
	assert.NoError(t, SwitchCatalogue(app, "github"))
	cat, _ := loaded_catalogue(app)
	assert.Len(t, cat.AddonSummaryList, 2)
	assert.Len(t, app.FilterResultListByNS(NS_CATALOGUE_ADDON), 2, "search results are replaced")
	assert.Equal(t, "github", saved_settings(t, app).Preferences.SelectedCatalogue)

	assert.Error(t, SwitchCatalogue(app, "nope"))
}

// --- user catalogue

// clj: `core_test.clj/add-user-addon!`, `--idempotence`, `remove-user-addon!`
func Test_user_catalogue_star_unstar(t *testing.T) {
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "AB")))},
	})
	assert.NoError(t, LoadCatalogue(app))
	path := get_paths(app)["strongbox.paths.user-catalogue-file"]
	assert.NoFileExists(t, path, "no user catalogue until an addon is added")

	ca := CatalogueAddon{URL: "https://gitlab.com/g/h", Name: "gh", Label: "GH", Source: "gitlab", SourceID: "g/h",
		UpdatedDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), GameTrackIDList: []GameTrackID{"retail"}, TagList: []string{}}
	for range 3 {
		assert.NoError(t, StarCatalogueAddon(app, ca))
	}
	user := read_user_catalogue(path)
	assert.Len(t, user.AddonSummaryList, 1)
	cat, _ := loaded_catalogue(app)
	assert.Len(t, cat.AddonSummaryList, 2, "the starred addon can be found")
	assert.True(t, cat.AddonSummaryList[1].Starred())

	assert.NoError(t, UnstarCatalogueAddon(app, ca))
	assert.NoError(t, UnstarCatalogueAddon(app, ca), "removing an absent addon is harmless")
	assert.Empty(t, read_user_catalogue(path).AddonSummaryList)
	cat, _ = loaded_catalogue(app)
	assert.Len(t, cat.AddonSummaryList, 1)
}

func Test_write_user_catalogue__preserves_unreadable(t *testing.T) {
	path := t.TempDir() + "/user-catalogue.json"
	os.WriteFile(path, []byte("{broken"), 0o644)
	assert.Empty(t, read_user_catalogue(path).AddonSummaryList)

	now := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)
	assert.NoError(t, write_user_catalogue(path, Catalogue{}, now))
	preserved, err := os.ReadFile(invalid_copy_path(path, now))
	assert.NoError(t, err)
	assert.Equal(t, "{broken", string(preserved))
	written := read_user_catalogue(path)
	assert.Equal(t, "2026-10-05", written.Datestamp)
}

// clj: `core_test.clj/refresh-user-catalogue-item*`
func Test_refresh_user_catalogue(t *testing.T) {
	user := Catalogue{AddonSummaryList: []CatalogueAddon{
		{Source: "github", SourceID: "a/b", Label: "AB", DownloadCount: 10},
		{Source: "gitlab", SourceID: "g/h", Label: "GH"},
		{Source: "github", SourceID: "x/y", Label: "XY"},
	}}
	full := Catalogue{AddonSummaryList: []CatalogueAddon{{Source: "github", SourceID: "a/b", Label: "AB", DownloadCount: 20}}}
	find_addon := func(ca CatalogueAddon) (CatalogueAddon, error) {
		if ca.SourceID == "x/y" {
			return CatalogueAddon{}, errors.New("host unreachable")
		}
		ca.DownloadCount = 5
		return ca, nil
	}
	actual := refresh_user_catalogue(user, full, find_addon)
	assert.Equal(t, 20, actual.AddonSummaryList[0].DownloadCount, "from the full catalogue")
	assert.Equal(t, 5, actual.AddonSummaryList[1].DownloadCount, "found at its host")
	assert.Equal(t, "XY", actual.AddonSummaryList[2].Label, "kept when it cannot be found")
	assert.Equal(t, 3, actual.Total)
}

// clj: `core_test.clj/scheduled-user-catalogue-refresh`
func Test_user_catalogue_refresh_due(t *testing.T) {
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	old := Catalogue{Datestamp: "2026-08-26", AddonSummaryList: []CatalogueAddon{{Label: "x"}}}
	recent := Catalogue{Datestamp: "2026-10-01", AddonSummaryList: []CatalogueAddon{{Label: "x"}}}
	assert.True(t, user_catalogue_refresh_due(old, true, now))
	assert.False(t, user_catalogue_refresh_due(old, false, now), "preference off")
	assert.False(t, user_catalogue_refresh_due(recent, true, now))
	assert.False(t, user_catalogue_refresh_due(Catalogue{Datestamp: "2020-01-01"}, true, now), "empty")
}

func Test_RefreshUserCatalogue(t *testing.T) {
	full := catalogue_json(catalogue_entry("github", "a/b", "AB"))
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_FULL.Source:                     {Body: []byte(full)},
		github_release_list_url("x/y"):      {Status: 404},
		github_release_list_url("aviana/c"): fixture("v7/github-repo-releases--aviana-healcomm.json"),
	})
	path := get_paths(app)["strongbox.paths.user-catalogue-file"]
	user := Catalogue{AddonSummaryList: []CatalogueAddon{
		{Source: "github", SourceID: "a/b", Label: "AB old", URL: "u", Name: "ab", GameTrackIDList: []GameTrackID{"retail"}, TagList: []string{}},
		{Source: "github", SourceID: "x/y", Label: "XY", URL: "u", Name: "xy", GameTrackIDList: []GameTrackID{"retail"}, TagList: []string{}},
		{Source: "github", SourceID: "aviana/c", Label: "C", URL: "u", Name: "c", GameTrackIDList: []GameTrackID{"retail"}, TagList: []string{}},
	}}
	assert.NoError(t, write_user_catalogue(path, user, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)))

	assert.NoError(t, RefreshUserCatalogue(app))
	actual := read_user_catalogue(path)
	assert.Equal(t, time.Now().UTC().Format("2006-01-02"), actual.Datestamp)
	labels := []string{}
	for _, ca := range actual.AddonSummaryList {
		labels = append(labels, ca.Label)
	}
	assert.Equal(t, []string{"AB", "XY", "HealComm"}, labels)
}
