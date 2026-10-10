package strongbox

// choosing updates, the updateable rules and checking addons for updates.
// clj: `addon_test.clj/test-updateable?`, `catalogue_test.clj/expand-summary--*`

import (
	"bw/core"
	"bw/http_utils"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// returns an update for `version` supporting `game_tracks`.
func su(version string, game_tracks ...GameTrackID) SourceUpdate {
	return SourceUpdate{Version: version, DownloadURL: "https://example.org/" + version + ".zip", GameTrackIDSet: mapset.NewSet(game_tracks...)}
}

// returns an installed addon at `installed_version` supporting retail, with nfo data
// changed by `nfo_fn`, given `updates`, in the addons dir `ad`.
func updateable_addon(ad AddonsDir, installed_version string, nfo_fn func(*NFO), updates ...SourceUpdate) Addon {
	toc := NewTOC()
	toc.InstalledVersion = installed_version
	toc.GameTrackIDSet = mapset.NewSet(GAMETRACK_RETAIL)
	toc.DirName = "EveryAddon"
	toc.Label = "EveryAddon"
	ia := MakeInstalledAddon("file:///x/EveryAddon", "EveryAddon", map[FileName]TOC{"EveryAddon.toc": toc}, NFOFile{}, false)
	nfo := NFO{GroupID: "g", Primary: true, InstalledVersion: installed_version, InstalledGameTrackID: GAMETRACK_RETAIL, Source: SOURCE_GITHUB, SourceID: "a/b"}
	if nfo_fn != nil {
		nfo_fn(&nfo)
	}
	ia.NFOFile = NFOFile{Stack: []NFO{nfo}}
	return MakeAddon(ad, []InstalledAddon{ia}, ia, &nfo, nil, updates)
}

var retail_strict = AddonsDir{Path: "/x", GameTrackID: GAMETRACK_RETAIL, Strict: true}

func Test_Updateable__truth_table(t *testing.T) {
	cases := []struct {
		name     string
		given    Addon
		expected bool
	}{
		{"no updates", updateable_addon(retail_strict, "1.2.3", nil), false},
		{"newer version", updateable_addon(retail_strict, "1.2.3", nil, su("1.2.4", GAMETRACK_RETAIL)), true},
		{"same version", updateable_addon(retail_strict, "1.2.3", nil, su("1.2.3", GAMETRACK_RETAIL)), false},
		{"ignored", updateable_addon(retail_strict, "1.2.3", func(n *NFO) { n.Ignored = new(true) }, su("1.2.4", GAMETRACK_RETAIL)), false},
		{"explicitly not ignored", updateable_addon(retail_strict, "1.2.3", func(n *NFO) { n.Ignored = new(false) }, su("1.2.4", GAMETRACK_RETAIL)), true},
		{"pinned at the installed version", updateable_addon(retail_strict, "1.2.3", func(n *NFO) { n.PinnedVersion = "1.2.3" }, su("1.2.4", GAMETRACK_RETAIL), su("1.2.3", GAMETRACK_RETAIL)), false},
		{"pinned at the installed version, pinned release gone", updateable_addon(retail_strict, "1.2.3", func(n *NFO) { n.PinnedVersion = "1.2.3" }, su("1.2.4", GAMETRACK_RETAIL)), false},
		{"pinned elsewhere, pinned version available",
			updateable_addon(retail_strict, "1.2.4", func(n *NFO) { n.PinnedVersion = "1.2.3" }, su("1.2.4", GAMETRACK_RETAIL), su("1.2.3", GAMETRACK_RETAIL)), true},
		{"pinned elsewhere, pinned version gone", updateable_addon(retail_strict, "1.2.4", func(n *NFO) { n.PinnedVersion = "1.0.0" }, su("1.2.5", GAMETRACK_RETAIL)), false},
		{"only classic update in a strict retail dir", updateable_addon(retail_strict, "1.2.3", nil, su("1.2.4", GAMETRACK_CLASSIC)), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, Updateable(c.given), c.name)
	}
}

// same version, but the installed addon covers none of the update's game tracks.
func Test_Updateable__same_version_wrong_game_track(t *testing.T) {
	relaxed_classic := AddonsDir{Path: "/x", GameTrackID: GAMETRACK_CLASSIC, Strict: false}
	given := updateable_addon(relaxed_classic, "1.2.3", func(n *NFO) { n.InstalledGameTrackID = GAMETRACK_CLASSIC_TBC }, su("1.2.3", GAMETRACK_CLASSIC))
	given.Primary.GametrackIDSet = mapset.NewSet(GAMETRACK_CLASSIC_TBC)
	assert.True(t, Updateable(given))

	// ... unless the installed game track recorded in the nfo is covered
	given = updateable_addon(relaxed_classic, "1.2.3", func(n *NFO) { n.InstalledGameTrackID = GAMETRACK_CLASSIC }, su("1.2.3", GAMETRACK_CLASSIC))
	given.Primary.GametrackIDSet = mapset.NewSet(GAMETRACK_CLASSIC_TBC)
	assert.False(t, Updateable(given))
}

// clj: `catalogue_test.clj/expand-summary--retail-strict--*`, `--classic-*`
func Test_pick_source_update(t *testing.T) {
	retail, classic := su("2.0", GAMETRACK_RETAIL), su("1.0", GAMETRACK_CLASSIC)
	cases := []struct {
		name     string
		updates  []SourceUpdate
		track    GameTrackID
		strict   bool
		expected string
	}{
		{"retail strict, just retail", []SourceUpdate{retail}, GAMETRACK_RETAIL, true, "2.0"},
		{"retail strict, just classic", []SourceUpdate{classic}, GAMETRACK_RETAIL, true, ""},
		{"retail strict, both", []SourceUpdate{retail, classic}, GAMETRACK_RETAIL, true, "2.0"},
		{"retail relaxed, just classic", []SourceUpdate{classic}, GAMETRACK_RETAIL, false, "1.0"},
		{"classic strict, just classic", []SourceUpdate{classic}, GAMETRACK_CLASSIC, true, "1.0"},
		{"classic strict, just retail", []SourceUpdate{retail}, GAMETRACK_CLASSIC, true, ""},
		{"classic strict, both", []SourceUpdate{retail, classic}, GAMETRACK_CLASSIC, true, "1.0"},
		{"classic relaxed, just retail", []SourceUpdate{retail}, GAMETRACK_CLASSIC, false, "2.0"},
	}
	for _, c := range cases {
		actual := _make_addon__pick_source_update(c.updates, c.track, c.strict, "")
		if c.expected == "" {
			assert.Nil(t, actual, c.name)
		} else {
			assert.Equal(t, c.expected, actual.Version, c.name)
		}
	}
}

// clj: `catalogue_test.clj/expand-summary--pinned--use-pinned`, `--use-latest`
func Test_pick_source_update__pinned(t *testing.T) {
	updates := []SourceUpdate{su("1.2.4", GAMETRACK_RETAIL), su("1.2.3", GAMETRACK_RETAIL)}
	assert.Equal(t, "1.2.3", _make_addon__pick_source_update(updates, GAMETRACK_RETAIL, true, "1.2.3").Version)
	assert.Equal(t, "1.2.4", _make_addon__pick_source_update(updates, GAMETRACK_RETAIL, true, "1.0.0").Version, "pinned release gone")
}

// clj: `catalogue_test.clj/expand-summary--not-found-message`
func Test_no_release_message(t *testing.T) {
	assert.Equal(t, "no 'Retail' release found on github.", no_release_message(AddonsDir{GameTrackID: GAMETRACK_RETAIL, Strict: true}, "github"))
	assert.Equal(t, "no 'Forever' or 'Retail' release found on github.", no_release_message(AddonsDir{GameTrackID: GAMETRACK_FOREVER}, "github"))
	assert.Equal(t, "no 'Classic (TBC)', 'Classic (WotLK)', 'Classic (Cata)', 'Classic (Mists)', 'Classic' or 'Retail' release found on wowinterface.",
		no_release_message(AddonsDir{GameTrackID: GAMETRACK_CLASSIC_TBC}, "wowinterface"))
}

// --- checking

// returns a running app with one selected addons dir holding the generated addons.
func app_with_installed(t *testing.T, routes map[string]http_utils.Fixture, spec_list ...test_addon_spec) (*core.App, string, *http_utils.FixtureTransport) {
	t.Helper()
	path := test_dir(t, "AddOns")
	for _, spec := range spec_list {
		test_gen_addon(t, path, spec)
	}
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(path)}
	settings.Preferences.SelectedAddonsDir = path
	app := test_app_with_settings(t, settings)
	ft := http_utils.NewFixtureTransport(routes)
	app.HTTPClient.Transport = ft
	return app, path, ft
}

// returns a github releases response with one release of `version`, with an asset for
// each of `game_tracks`.
func github_releases_json(version string, game_tracks ...string) string {
	assets := []string{}
	for _, gt := range game_tracks {
		assets = append(assets, `{"name": "EveryAddon-`+version+`-`+gt+`.zip", "state": "uploaded", "content_type": "application/zip",
			"browser_download_url": "https://github.com/a/b/releases/download/`+version+`/EveryAddon-`+version+`-`+gt+`.zip"}`)
	}
	return `[{"name": "` + version + `", "tag_name": "` + version + `", "html_url": "https://github.com/a/b/releases/tag/` + version + `",
		"published_at": "2026-01-01T00:00:00Z", "assets": [` + join_strings(assets) + `]}]`
}

func join_strings(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func Test_CheckForUpdates(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("1.2.4", "retail"))},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3"})

	CheckForUpdates(app)
	r := addon_results(app)[0]
	a := r.Item.(Addon)
	assert.Equal(t, "1.2.4", a.AvailableVersion)
	assert.True(t, Updateable(a))
	assert.True(t, r.Tags.Contains(core.TAG_HAS_UPDATE))
	assert.False(t, r.Tags.Contains(core.TAG_BUSY))
	assert.Empty(t, app.Jobs())
}

// one failing host does not stop the others, and leaves its addon as it was.
func Test_CheckForUpdates__one_host_fails(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Status: 500},
		github_release_list_url("c/d"): {Body: []byte(github_releases_json("2.0", "retail"))},
	},
		test_addon_spec{DirList: []string{"Broken"}, Source: SOURCE_GITHUB, SourceID: "a/b"},
		test_addon_spec{DirList: []string{"Working"}, Source: SOURCE_GITHUB, SourceID: "c/d"})

	CheckForUpdates(app)
	for _, r := range addon_results(app) {
		a := r.Item.(Addon)
		assert.False(t, r.Tags.Contains(core.TAG_BUSY), a.Label)
		if a.Label == "Working" {
			assert.Equal(t, "2.0", a.AvailableVersion)
		} else {
			assert.Empty(t, a.SourceUpdateList)
		}
	}
}

// ignored addons and unsupported sources are never requested.
func Test_CheckForUpdates__skipped(t *testing.T) {
	app, path, ft := app_with_installed(t, nil,
		test_addon_spec{DirList: []string{"Ignored"}, Source: SOURCE_GITHUB, SourceID: "a/b"},
		test_addon_spec{DirList: []string{"Curse"}, Source: SOURCE_CURSEFORGE, SourceID: "123"},
		test_addon_spec{DirList: []string{"Unmanaged"}})
	os.MkdirAll(filepath.Join(path, "Ignored", ".git"), 0o755)
	assert.NoError(t, ReloadAddonsDir(app, MakeAddonsDir(path)))

	CheckForUpdates(app)
	assert.Empty(t, ft.Requested())
}

// rows are marked busy while they are checked.
func Test_check_addon__busy(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("1.2.4", "retail"))},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"})
	id := addon_results(app)[0].ID

	obs := &busy_observer{id: id}
	app.AddObserver(obs)
	assert.NoError(t, check_addon(app, id))
	app.Flush()
	obs.mu.Lock()
	defer obs.mu.Unlock()
	assert.Equal(t, []bool{true, false}, obs.seen)
}

// records each change to the busy mark of the result `id`.
type busy_observer struct {
	mu   sync.Mutex
	id   string
	seen []bool
}

func (o *busy_observer) OnResultsChanged(_, new_snapshot *core.Snapshot) {
	r := new_snapshot.GetResult(o.id)
	if r == nil {
		return
	}
	busy := r.Tags.Contains(core.TAG_BUSY)
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.seen) == 0 || o.seen[len(o.seen)-1] != busy {
		if len(o.seen) == 0 && !busy {
			return
		}
		o.seen = append(o.seen, busy)
	}
}

func (o *busy_observer) OnAction(core.Action) {}

// a check selects the addon's update by the addons dir's game track.
func Test_CheckForUpdates__game_track(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("1.2.4", "classic"))},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"})
	CheckForUpdates(app)
	a := addon_results(app)[0].Item.(Addon)
	assert.Len(t, a.SourceUpdateList, 1)
	assert.Nil(t, a.SourceUpdate, "strict retail ignores a classic update")
}

// an unsupported source is never requested and never marked as having an update.
func Test_CheckForUpdates__unsupported_source_not_marked(t *testing.T) {
	app, _, ft := app_with_installed(t, nil,
		test_addon_spec{DirList: []string{"Curse"}, Source: SOURCE_CURSEFORGE, SourceID: "123"})
	CheckForUpdates(app)
	assert.Empty(t, ft.Requested())
	r := addon_results(app)[0]
	assert.False(t, r.Tags.Contains(core.TAG_HAS_UPDATE))
	assert.False(t, Updateable(r.Item.(Addon)))
}

// a failing host is logged at WARN naming its addon, and the others are still marked.
func Test_CheckForUpdates__one_host_fails_logged(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Status: 500},
		github_release_list_url("c/d"): {Body: []byte(github_releases_json("2.0", "retail"))},
	},
		test_addon_spec{DirList: []string{"Broken"}, Source: SOURCE_GITHUB, SourceID: "a/b"},
		test_addon_spec{DirList: []string{"Working"}, Source: SOURCE_GITHUB, SourceID: "c/d"})

	actual := capture_log(func() { CheckForUpdates(app) })
	assert.Contains(t, actual, "level=WARN")
	assert.Contains(t, actual, "addon=Broken")
	r := addon_result_by_label(t, app, "Working")
	assert.True(t, r.Tags.Contains(core.TAG_HAS_UPDATE))
}

// a release published since the last check is offered once the cached response expires.
func Test_CheckForUpdates__cache_expires(t *testing.T) {
	app, _, ft := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("1.2.4", "retail"))},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3"})
	now := time.Now()
	app.HTTPClient.Transport = &http_utils.FileCachingRequest{Dir: t.TempDir(), Next: ft, Now: func() time.Time { return now }}

	CheckForUpdates(app)
	assert.Equal(t, "1.2.4", addon_results(app)[0].Item.(Addon).AvailableVersion)

	ft.Set(github_release_list_url("a/b"), http_utils.Fixture{Body: []byte(github_releases_json("1.2.5", "retail"))})
	CheckForUpdates(app)
	assert.Equal(t, "1.2.4", addon_results(app)[0].Item.(Addon).AvailableVersion, "the cached response is still fresh")

	now = now.Add(2 * time.Hour)
	CheckForUpdates(app)
	assert.Equal(t, "1.2.5", addon_results(app)[0].Item.(Addon).AvailableVersion)
}

// a host with no releases at all is reported at INFO, naming the game tracks searched.
func Test_check_addon__no_releases_reported(t *testing.T) {
	app, _, _ := app_with_installed(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte("[]")},
	}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"})
	actual := capture_log(func() { CheckForUpdates(app) })
	assert.Contains(t, actual, "level=INFO")
	assert.Contains(t, actual, "no 'Retail' release found on github.")
}

// without nfo data, an addon at the update's version whose .toc files cover none of the
// update's game tracks is updateable; one whose .toc files cover them is not.
func Test_Updateable__same_version_no_nfo(t *testing.T) {
	toc := NewTOC()
	toc.InstalledVersion = "1.2.3"
	toc.GameTrackIDSet = mapset.NewSet(GAMETRACK_CLASSIC_TBC)
	toc.DirName = "EveryAddon"
	toc.Label = "EveryAddon"
	ia := MakeInstalledAddon("file:///x/EveryAddon", "EveryAddon", map[FileName]TOC{"EveryAddon.toc": toc}, NFOFile{}, false)
	ca := everyaddon_ca
	relaxed_classic := AddonsDir{Path: "/x", GameTrackID: GAMETRACK_CLASSIC, Strict: false}

	given := MakeAddon(relaxed_classic, []InstalledAddon{ia}, ia, nil, &ca, []SourceUpdate{su("1.2.3", GAMETRACK_CLASSIC)})
	assert.True(t, Updateable(given))

	given = MakeAddon(relaxed_classic, []InstalledAddon{ia}, ia, nil, &ca, []SourceUpdate{su("1.2.3", GAMETRACK_CLASSIC_TBC)})
	assert.False(t, Updateable(given))
}
