package strongbox

import (
	"bw/core"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns a running app holding the addons dir `ad`, with its addons loaded.
func app_with_addons_dir(t *testing.T, ad AddonsDir) *core.App {
	t.Helper()
	app := core.NewApp()
	go app.ProcessUpdateLoop()
	t.Cleanup(app.Stop)
	ad.selected = true
	ad.available = true
	app.AddReplaceResults(MakeAddonsDirResult(ad)).Wait()
	return app
}

// returns the addon results in app state.
func addon_results(app *core.App) []core.Result {
	return app.FilterResultList(func(r core.Result) bool { return r.NS == NS_ADDON })
}

func Test_ReloadAddonsDir__no_duplicates(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_WOWI, SourceID: "1"})
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryOtherAddon"}})
	app := app_with_addons_dir(t, ad)

	first := addon_results(app)
	assert.Len(t, first, 2)
	total := len(app.GetResultList())

	for range 2 {
		assert.NoError(t, ReloadAddonsDir(app, ad))
	}
	second := addon_results(app)
	assert.Len(t, second, 2)
	assert.Equal(t, total, len(app.GetResultList()), "children are replaced too")

	ids := func(rl []core.Result) []string {
		out := []string{}
		for _, r := range rl {
			out = append(out, r.ID)
		}
		return out
	}
	assert.ElementsMatch(t, ids(first), ids(second), "the same addons get the same IDs")
	for _, r := range second {
		assert.Equal(t, ad.Path, r.ParentID)
	}
}

func Test_ReloadAddonsDir__deleted_outside_strongbox(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryOtherAddon"}})
	app := app_with_addons_dir(t, ad)
	assert.Len(t, addon_results(app), 2)

	os.RemoveAll(filepath.Join(ad.Path, "EveryOtherAddon"))
	assert.NoError(t, ReloadAddonsDir(app, ad))
	actual := addon_results(app)
	assert.Len(t, actual, 1)
	assert.Equal(t, "EveryAddon", actual[0].Item.(Addon).Label)
}

func Test_ReloadAddonsDir__not_in_state(t *testing.T) {
	app := core.NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()
	assert.Error(t, ReloadAddonsDir(app, test_addons_dir(t)))
}

// a directory shared by two addons is a child of both, without an ID clash.
func Test_addon_children__shared_directory(t *testing.T) {
	ad := test_addons_dir(t)
	everyaddon := test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_WOWI, SourceID: "1"}
	other := test_addon_spec{DirList: []string{"EveryOtherAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b"}
	test_gen_addon(t, ad.Path, everyaddon)
	test_gen_addon(t, ad.Path, other)
	test_write_nfo(t, filepath.Join(ad.Path, "EveryAddon-BundledAddon"), test_addon_nfo(everyaddon, "EveryAddon-BundledAddon"), test_addon_nfo(other, "EveryAddon-BundledAddon"))

	app := app_with_addons_dir(t, ad)
	seen := map[string]bool{}
	for _, r := range app.GetResultList() {
		assert.False(t, seen[r.ID], "duplicate id %s", r.ID)
		seen[r.ID] = true
	}
	installed := app.FilterResultList(func(r core.Result) bool { return r.NS == NS_INSTALLED_ADDON })
	assert.Len(t, installed, 3, "the shared directory appears under its top owner only")
}
