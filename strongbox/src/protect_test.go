package strongbox

// clj: `core_test.clj/ignore-addon`, `clear-addon-ignore-flag*`, `cli_test.clj/pin-addon`,
// `unpin-addon`, `addon_test.clj/switch-source*`, `test-pinnable?`, `test-unpinnable?`

import (
	"bw/core"
	"bw/http_utils"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the single addon result's addon.
func only_addon(t *testing.T, app *core.App) (*core.Result, Addon) {
	t.Helper()
	results := addon_results(app)
	assert.Len(t, results, 1)
	return &results[0], results[0].Item.(Addon)
}

func Test_IgnoreAddons(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-Bundled"}, Source: SOURCE_WOWI, SourceID: "1"})
	r, _ := only_addon(t, app)
	assert.NoError(t, IgnoreAddons(app, []*core.Result{r}))
	r, a := only_addon(t, app)
	assert.True(t, a.IsIgnored)
	assert.True(t, r.Tags.Contains(core.TAG_MUTED))
	for _, dir := range []string{"EveryAddon", "EveryAddon-Bundled"} {
		top, _ := read_nfo(t, MakeAddonsDir(path), dir).Top()
		assert.Equal(t, new(true), top.Ignored, dir)
	}

	// clj: `clear-addon-ignore-flag`. the flag is removed, not set to false
	assert.NoError(t, StopIgnoringAddons(app, []*core.Result{r}))
	_, a = only_addon(t, app)
	assert.False(t, a.IsIgnored)
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Nil(t, top.Ignored)
}

// ignoring an addon strongbox did not install writes an ignore-only nfo file, and
// stopping removes it.
func Test_IgnoreAddons__unmanaged(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}})
	r, _ := only_addon(t, app)
	assert.NoError(t, IgnoreAddons(app, []*core.Result{r}))
	b, _ := os.ReadFile(nfo_path(filepath.Join(path, "EveryAddon")))
	assert.Equal(t, `{"ignore?":true}`, string(b))

	r, _ = only_addon(t, app)
	assert.NoError(t, StopIgnoringAddons(app, []*core.Result{r}))
	assert.NoFileExists(t, nfo_path(filepath.Join(path, "EveryAddon")))
}

// clj: `clear-addon-ignore-flag--implicit-ignore`
func Test_StopIgnoringAddons__git_checkout(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}})
	os.MkdirAll(filepath.Join(path, "EveryAddon", ".git"), 0o755)
	assert.NoError(t, ReloadAddonsDir(app, derive_addons_dir(MakeAddonsDir(path), FindSettings(app), core.DirExists)))
	r, a := only_addon(t, app)
	assert.True(t, a.IsIgnored)

	assert.NoError(t, StopIgnoringAddons(app, []*core.Result{r}))
	_, a = only_addon(t, app)
	assert.False(t, a.IsIgnored)
	b, _ := os.ReadFile(nfo_path(filepath.Join(path, "EveryAddon")))
	assert.Equal(t, `{"ignore?":false}`, string(b))
}

// clj: `clear-addon-ignore-flag--group-addons`
func Test_StopIgnoringAddons__group(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-Bundled"}, Source: SOURCE_WOWI, SourceID: "1"})
	f := read_nfo(t, MakeAddonsDir(path), "EveryAddon-Bundled")
	write_nfo_file(filepath.Join(path, "EveryAddon-Bundled"), nfo_file_ignore(f, false))
	assert.NoError(t, ReloadAddonsDir(app, derive_addons_dir(MakeAddonsDir(path), FindSettings(app), core.DirExists)))
	r, a := only_addon(t, app)
	assert.True(t, a.IsIgnored, "one ignored member ignores the group")

	assert.NoError(t, StopIgnoringAddons(app, []*core.Result{r}))
	_, a = only_addon(t, app)
	assert.False(t, a.IsIgnored)
}

// an addon strongbox did not install has no nfo data to keep a pin in.
func Test_PinAddons__requires_nfo(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "1.2.3"})
	r, a := only_addon(t, app)
	assert.False(t, pinnable(a))
	assert.NoError(t, PinAddons(app, []*core.Result{r}))
	_, a = only_addon(t, app)
	assert.False(t, a.IsPinned)
	assert.NoFileExists(t, filepath.Join(path, "EveryAddon", ".strongbox.json"))
}

func Test_PinAddons_UnpinAddons(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-Bundled"}, Source: SOURCE_WOWI, SourceID: "1", Version: "1.2.3"})
	r, a := only_addon(t, app)
	assert.True(t, pinnable(a))
	assert.NoError(t, PinAddons(app, []*core.Result{r}))
	r, a = only_addon(t, app)
	assert.True(t, a.IsPinned)
	assert.Equal(t, "1.2.3", a.PinnedVersion)
	for _, dir := range []string{"EveryAddon", "EveryAddon-Bundled"} {
		top, _ := read_nfo(t, MakeAddonsDir(path), dir).Top()
		assert.Equal(t, "1.2.3", top.PinnedVersion, dir)
	}

	assert.True(t, unpinnable(a))
	assert.NoError(t, UnpinAddons(app, []*core.Result{r}))
	_, a = only_addon(t, app)
	assert.False(t, a.IsPinned)
}

func Test_pinnable(t *testing.T) {
	assert.True(t, pinnable(updateable_addon(retail_strict, "1.0", nil)))
	assert.False(t, pinnable(updateable_addon(retail_strict, "1.0", func(n *NFO) { n.Ignored = new(true) })), "ignored")
	assert.False(t, pinnable(updateable_addon(retail_strict, "1.0", func(n *NFO) { n.PinnedVersion = "1.0" })), "already pinned")
	assert.False(t, pinnable(updateable_addon(retail_strict, "", func(n *NFO) { n.InstalledVersion = "" })), "no version")
	assert.False(t, unpinnable(updateable_addon(retail_strict, "1.0", func(n *NFO) { n.PinnedVersion = "1.0"; n.Ignored = new(true) })))
}

// a pinned addon is not updated past its pin.
func Test_PinAddons__blocks_update(t *testing.T) {
	app, _, _ := app_with_installed(t, everyaddon_routes(t, "1.2.4"),
		test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3"})
	r, _ := only_addon(t, app)
	assert.NoError(t, PinAddons(app, []*core.Result{r}))
	CheckForUpdates(app)
	_, a := only_addon(t, app)
	assert.False(t, Updateable(a))
}

// clj: `addon_test.clj/switch-source`, `--no-sources-available`, `--no-ignored`, `--no-pinned`
func Test_SwitchSource(t *testing.T) {
	routes := map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json("2.0", "retail"))},
	}
	app, path, _ := app_with_installed(t, routes, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "123"})
	test_write_tree(t, path, test_file_tree{"EveryAddon/EveryAddon.toc": test_gen_toc("EveryAddon", "1.2.3", "110000", map[string]string{"X-Github": "https://github.com/a/b"})})
	assert.NoError(t, ReloadAddonsDir(app, derive_addons_dir(MakeAddonsDir(path), FindSettings(app), core.DirExists)))

	r, a := only_addon(t, app)
	assert.Equal(t, []SourceMap{{SOURCE_WOWI, "123"}, {SOURCE_GITHUB, "a/b"}}, a.SourceMapList)
	assert.NoError(t, SwitchSource(app, r, SourceMap{SOURCE_GITHUB, "a/b"}))

	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, SOURCE_GITHUB, top.Source)
	assert.Equal(t, FlexString("a/b"), top.SourceID)
	assert.Equal(t, []SourceMap{{SOURCE_WOWI, "123"}, {SOURCE_GITHUB, "a/b"}}, top.SourceMapList)
	_, a = only_addon(t, app)
	assert.Equal(t, SOURCE_GITHUB, a.Source)
	assert.Equal(t, "2.0", a.AvailableVersion, "checked against the new source")

	// pinned addons cannot switch
	r, _ = only_addon(t, app)
	assert.NoError(t, PinAddons(app, []*core.Result{r}))
	r, _ = only_addon(t, app)
	assert.Error(t, SwitchSource(app, r, SourceMap{SOURCE_WOWI, "123"}))
	top, _ = read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, SOURCE_GITHUB, top.Source)
}

func Test_SwitchSource__no_other_source(t *testing.T) {
	app, _, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "123"})
	r, a := only_addon(t, app)
	assert.False(t, source_switchable(a))
	assert.Error(t, SwitchSource(app, r, SourceMap{SOURCE_GITHUB, "a/b"}))
}

// clj: `addon_test.clj/merge-toc-nfo--source-map-list*`
func Test_source_map_list(t *testing.T) {
	addon_list := installed_addons_from(t, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "123"})
	assert.Equal(t, []SourceMap{{SOURCE_WOWI, "123"}}, addon_list[0].SourceMapList)

	// dead hosts are dropped
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_CURSEFORGE, SourceID: "9"}
	test_gen_addon(t, ad.Path, spec)
	addon_list, _ = LoadAllInstalledAddons(ad)
	assert.Nil(t, addon_list[0].SourceMapList)
}
