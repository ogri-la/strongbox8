package strongbox

// clj: `core_test.clj/-download-strongbox-release-list*`, `latest-strongbox-release!*`,
// `cli_test.clj/search-db*`

import (
	"bw/core"
	"bw/http_utils"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// a transport whose requests never complete, standing in for no network.
type blocking_transport struct{ release chan bool }

func (b blocking_transport) RoundTrip(r *http.Request) (*http.Response, error) {
	<-b.release
	return nil, http.ErrHandlerTimeout
}

// starting shows the installed addons without waiting on the network.
func Test_Start__offline(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	addons := test_dir(t, "AddOns")
	for _, dir := range []string{"One", "Two", "Three"} {
		test_gen_addon(t, addons, test_addon_spec{DirList: []string{dir}})
	}
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(addons)}
	settings.Preferences.SelectedAddonsDir = addons
	assert.NoError(t, save_settings_file(settings, filepath.Join(root, "config", APP_DIR_NAME, "config.json")))

	app := core.NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()
	block := blocking_transport{release: make(chan bool)}
	defer func() {
		// let the background refresh finish before the temporary dirs are removed
		close(block.release)
		app.WaitForJobs()
	}()
	app.HTTPClient.Transport = block

	done := make(chan error)
	go func() { done <- Start(app) }()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("starting waited on the network")
	}
	assert.Len(t, addon_results(app), 3)
	assert.NotEmpty(t, app.Jobs(), "the refresh carries on in the background")
}

// a second refresh while one runs does nothing, and refreshing again adds nothing twice.
func Test_Refresh__single_flight(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source:               {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "EveryAddon")))},
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("1.2.4", "retail"))},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"})
	apply_settings(app, func(s Settings) Settings { s.Preferences.CheckForUpdate = false; return s })

	refresh_running(app).Store(true)
	assert.False(t, Refresh(app))
	refresh_running(app).Store(false)

	assert.True(t, Refresh(app))
	total := len(app.GetResultList())
	assert.True(t, Refresh(app))
	assert.Equal(t, total, len(app.GetResultList()))
	r := addon_results(app)[0]
	assert.True(t, r.Tags.Contains(core.TAG_HAS_UPDATE), "updates offered")
	assert.Equal(t, "1.2.3", r.Item.(Addon).InstalledVersion, "but not applied")
	assert.Empty(t, app.Jobs())
}

// the catalogue already on disk is used before any download.
func Test_Refresh__local_catalogue_first(t *testing.T) {
	app, _, ft := app_with_installed(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Status: 500},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon"})
	apply_settings(app, func(s Settings) Settings { s.Preferences.CheckForUpdate = false; return s })
	path := CataloguePath(app, "short")
	core.MakeParents(path)
	write_atomic(path, []byte(catalogue_json(catalogue_entry("github", "a/b", "EveryAddon"))))
	old := time.Now().Add(-2 * time.Hour)
	chtimes(path, old)

	Refresh(app)
	assert.Contains(t, ft.Requested(), CAT_SHORT.Source, "the stale catalogue was asked for")
	assert.NotNil(t, addon_results(app)[0].Item.(Addon).CatalogueAddon, "matched against the local copy anyway")
}

// a refresh brings an out of date user catalogue up to date when the preference is on.
// clj: `core.clj/scheduled-user-catalogue-refresh`
func Test_Refresh__scheduled_user_catalogue_refresh(t *testing.T) {
	app, _, ft := app_with_installed(t, map[string]http_utils.Fixture{
		CAT_SHORT.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "EveryAddon")))},
		CAT_FULL.Source:  {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "EveryAddon Renamed")))},
	})
	apply_settings(app, func(s Settings) Settings {
		s.Preferences.CheckForUpdate = false
		s.Preferences.KeepUserCatalogueUpdated = true
		return s
	})
	user_cat_path := app.State().GetKeyVal("strongbox.paths.user-catalogue-file")
	given := Catalogue{Spec: CatalogueSpec{Version: CATALOGUE_VERSION}, Datestamp: "2020-01-01",
		Total: 1, AddonSummaryList: []CatalogueAddon{everyaddon_ca}}
	core.MakeParents(user_cat_path)
	b, err := json.Marshal(given)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(user_cat_path, b, 0644))

	Refresh(app)

	assert.Empty(t, ft.Unrouted())
	assert.Contains(t, ft.Requested(), CAT_FULL.Source)
	actual := read_user_catalogue(user_cat_path)
	assert.Equal(t, time.Now().UTC().Format("2006-01-02"), actual.Datestamp)
	assert.Len(t, actual.AddonSummaryList, 1)
	assert.Equal(t, "EveryAddon Renamed", actual.AddonSummaryList[0].Label)

	// refreshed today, it is not refreshed again
	requested := len(ft.Requested())
	Refresh(app)
	for _, url := range ft.Requested()[requested:] {
		assert.NotEqual(t, CAT_FULL.Source, url)
	}
}

func Test_RefuseRoot(t *testing.T) {
	assert.Error(t, RefuseRoot(0))
	assert.NoError(t, RefuseRoot(1000))
}

// clj: `cli_test.clj/search-db`, `search-db--empty-term`
func Test_CatalogueSearchFilter(t *testing.T) {
	row := CatalogueAddon{Label: "Chinchilla", Name: "chinchilla", Description: "Minimap with Auction House alerts"}.ItemMap()
	assert.True(t, CatalogueSearchFilter("", row))
	assert.True(t, CatalogueSearchFilter("chin", row))
	assert.True(t, CatalogueSearchFilter("CHIN", row))
	assert.True(t, CatalogueSearchFilter("auction", row))
	assert.False(t, CatalogueSearchFilter("bags", row))
}

func Test_compare_semver(t *testing.T) {
	ordered := []string{"7.8.0", "8.0.0-alpha.3", "8.0.0-beta.1", "8.0.0", "v8.0.1", "8.1", "10.0.0"}
	for i := range ordered {
		for j := range ordered {
			expected := 0
			if i < j {
				expected = -1
			} else if i > j {
				expected = 1
			}
			assert.Equal(t, expected, compare_semver(ordered[i], ordered[j]), "%s vs %s", ordered[i], ordered[j])
		}
	}
}

// clj: `core_test.clj/latest-strongbox-release!`, `--throttled`
func Test_CheckForStrongboxUpdate(t *testing.T) {
	releases := `[{"tag_name": "9.0.0-beta.1", "prerelease": true}, {"tag_name": "8.1.0"}, {"tag_name": "8.0.0"}, {"tag_name": "9.9.9", "draft": true}]`
	app, ft := test_app_with_routes(t, map[string]http_utils.Fixture{STRONGBOX_RELEASES_URL: {Body: []byte(releases)}})
	prev := VERSION
	defer func() { VERSION = prev }()
	VERSION = "8.0.0"

	CheckForStrongboxUpdate(app)
	assert.Equal(t, "8.1.0", app.State().GetKeyVal(KV_UPDATE_AVAILABLE))

	// already the newest
	app2, _ := test_app_with_routes(t, map[string]http_utils.Fixture{STRONGBOX_RELEASES_URL: {Body: []byte(releases)}})
	VERSION = "8.1.0"
	CheckForStrongboxUpdate(app2)
	assert.Equal(t, "", app2.State().GetKeyVal(KV_UPDATE_AVAILABLE))

	// throttled
	ft.Set(STRONGBOX_RELEASES_URL, http_utils.Fixture{Status: 403})
	app.State().SetKeyAnyVal(KV_UPDATE_AVAILABLE, "")
	CheckForStrongboxUpdate(app)
	assert.Equal(t, "", app.State().GetKeyVal(KV_UPDATE_AVAILABLE))

	// preference off: nothing is requested
	app3, ft3 := test_app_with_routes(t, nil)
	apply_settings(app3, func(s Settings) Settings { s.Preferences.CheckForUpdate = false; return s })
	CheckForStrongboxUpdate(app3)
	assert.Empty(t, ft3.Requested())
}
