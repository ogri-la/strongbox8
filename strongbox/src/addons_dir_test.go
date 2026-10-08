package strongbox

// adding, selecting and removing addons dirs, and changing their game track and strictness.
// clj: `core_test.clj/addon-dir-handling`, `game-strictness`

import (
	"bw/core"
	"os"
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// returns a running app holding `settings`, saving to a settings file in a temp dir.
func test_app_with_settings(t *testing.T, settings Settings) *core.App {
	t.Helper()
	app := core.NewApp()
	go app.ProcessUpdateLoop()
	t.Cleanup(app.Stop)
	root := t.TempDir()
	set_paths(app, GeneratePathMap(fake_env(map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_DATA_HOME":   filepath.Join(root, "data"),
	}), root), V7Paths{})
	settings_to_state(app, settings)
	return app
}

// returns the settings saved to disk.
func saved_settings(t *testing.T, app *core.App) Settings {
	t.Helper()
	b, err := os.ReadFile(get_paths(app)["strongbox.paths.cfg-file"])
	assert.NoError(t, err)
	parsed, err := parse_settings(b, always_available)
	assert.NoError(t, err)
	return parsed.Settings
}

// returns the addons dir results in app state, by path.
func addons_dir_results_in_state(app *core.App) map[string]AddonsDir {
	idx := map[string]AddonsDir{}
	for _, r := range app.FilterResultListByNS(NS_ADDONS_DIR) {
		idx[r.ID] = r.Item.(AddonsDir)
	}
	return idx
}

// returns a new directory named `name` in a temp dir.
func test_dir(t *testing.T, name ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{t.TempDir()}, name...)...)
	os.MkdirAll(path, 0o755)
	return path
}

func Test_AddAddonsDir(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	retail := test_dir(t, "wow", "_retail_", "Interface", "AddOns")

	assert.NoError(t, AddAddonsDir(app, retail))
	settings := FindSettings(app)
	assert.Equal(t, []AddonsDir{{Path: retail, GameTrackID: GAMETRACK_RETAIL, Strict: true}}, settings.AddonsDirList)
	assert.Equal(t, retail, settings.Preferences.SelectedAddonsDir)
	assert.True(t, addons_dir_results_in_state(app)[retail].selected)

	// saved straight away
	assert.Equal(t, settings.AddonsDirList, saved_settings(t, app).AddonsDirList)

	// the game track is guessed from the path
	classic := test_dir(t, "wow", "_classic_era_", "Interface", "AddOns")
	assert.NoError(t, AddAddonsDir(app, classic))
	settings = FindSettings(app)
	assert.Equal(t, GAMETRACK_CLASSIC, settings.AddonsDirList[1].GameTrackID)
	assert.Equal(t, classic, settings.Preferences.SelectedAddonsDir)
	assert.False(t, addons_dir_results_in_state(app)[retail].selected)

	// adding again changes nothing but selects it
	assert.NoError(t, AddAddonsDir(app, retail))
	assert.Len(t, FindSettings(app).AddonsDirList, 2)
	assert.Equal(t, retail, FindSettings(app).Preferences.SelectedAddonsDir)
}

func Test_AddAddonsDir__not_a_directory(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	assert.Error(t, AddAddonsDir(app, "/does/not/exist"))
	file := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	assert.Error(t, AddAddonsDir(app, file))
	assert.Empty(t, FindSettings(app).AddonsDirList)
}

func Test_SelectAddonsDir(t *testing.T) {
	a := test_dir(t, "a")
	b := test_dir(t, "b")
	test_gen_addon(t, b, test_addon_spec{DirList: []string{"EveryAddon"}})
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(a), MakeAddonsDir(b)}
	settings.Preferences.SelectedAddonsDir = a
	app := test_app_with_settings(t, settings)
	assert.Empty(t, addon_results(app), "the unselected addons dir is not loaded")

	assert.NoError(t, SelectAddonsDir(app, b))
	assert.Equal(t, b, FindSettings(app).Preferences.SelectedAddonsDir)
	assert.Equal(t, b, saved_settings(t, app).Preferences.SelectedAddonsDir)
	assert.Len(t, addon_results(app), 1, "the selected addons dir's addons are loaded")

	assert.Error(t, SelectAddonsDir(app, test_dir(t, "unknown")))
	assert.Error(t, SelectAddonsDir(app, "/does/not/exist"))
}

func Test_RemoveAddonsDir(t *testing.T) {
	a := test_dir(t, "a")
	b := test_dir(t, "b")
	test_gen_addon(t, a, test_addon_spec{DirList: []string{"EveryAddon"}})
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(a), MakeAddonsDir(b)}
	settings.Preferences.SelectedAddonsDir = a
	app := test_app_with_settings(t, settings)
	assert.Len(t, addon_results(app), 1)

	RemoveAddonsDir(app, a)
	actual := FindSettings(app)
	assert.Equal(t, []AddonsDir{MakeAddonsDir(b)}, actual.AddonsDirList)
	assert.Equal(t, b, actual.Preferences.SelectedAddonsDir, "the next addons dir is selected")
	assert.NotContains(t, addons_dir_results_in_state(app), a, "the addons dir row is gone")
	assert.Empty(t, addon_results(app), "its addons are gone")
	assert.DirExists(t, filepath.Join(a, "EveryAddon"), "nothing on disk is touched")
	assert.Equal(t, actual.AddonsDirList, saved_settings(t, app).AddonsDirList)

	// removing the last addons dir selects nothing
	RemoveAddonsDir(app, b)
	assert.Equal(t, "", FindSettings(app).Preferences.SelectedAddonsDir)

	// removing an unknown addons dir is harmless
	RemoveAddonsDir(app, "/nope")
}

// changing the game track and strictness re-evaluates the addons in that addons dir.
func Test_SetAddonsDirGameTrack_and_strictness(t *testing.T) {
	path := test_dir(t, "AddOns")
	test_write_tree(t, path, test_file_tree{
		"EveryAddon/EveryAddon.toc":         test_gen_toc("EveryAddon", "1.2.3", "110000", map[string]string{"Notes": "retail notes"}),
		"EveryAddon/EveryAddon_Vanilla.toc": test_gen_toc("EveryAddon", "1.2.3", "11503", map[string]string{"Notes": "classic notes"}),
	})
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(path)}
	settings.Preferences.SelectedAddonsDir = path
	app := test_app_with_settings(t, settings)
	description := func() string { return addon_results(app)[0].Item.(Addon).Description }
	assert.Equal(t, "retail notes", description())

	assert.NoError(t, SetAddonsDirGameTrack(app, path, GAMETRACK_CLASSIC))
	assert.Equal(t, "classic notes", description())
	assert.Equal(t, GAMETRACK_CLASSIC, saved_settings(t, app).AddonsDirList[0].GameTrackID)

	// strict mists finds no .toc, relaxed mists falls back to classic
	assert.NoError(t, SetAddonsDirGameTrack(app, path, GAMETRACK_CLASSIC_MISTS))
	assert.Nil(t, addon_results(app)[0].Item.(Addon).TOC)
	SetAddonsDirStrict(app, path, false)
	assert.Equal(t, "classic notes", description())
	assert.False(t, saved_settings(t, app).AddonsDirList[0].Strict)

	assert.Error(t, SetAddonsDirGameTrack(app, path, "classic-bfa"))
}

// a strict retail addons dir offered only a classic update gets it once relaxed.
func Test_SetAddonsDirStrict__update_choice(t *testing.T) {
	path := test_dir(t, "AddOns")
	test_gen_addon(t, path, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "1"})
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(path)}
	settings.Preferences.SelectedAddonsDir = path
	app := test_app_with_settings(t, settings)

	r := addon_results(app)[0]
	app.UpdateResult(r.ID, func(r core.Result) core.Result {
		a := r.Item.(Addon)
		classic_update := SourceUpdate{Version: "9.9.9", GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)}
		r.Item = MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, a.NFO, a.CatalogueAddon, []SourceUpdate{classic_update})
		return r
	}).Wait()
	assert.Nil(t, addon_results(app)[0].Item.(Addon).SourceUpdate)

	SetAddonsDirStrict(app, path, false)
	actual := addon_results(app)[0]
	assert.Equal(t, "9.9.9", actual.Item.(Addon).SourceUpdate.Version)
	assert.True(t, actual.Tags.Contains(core.TAG_HAS_UPDATE))
}

// an addons dir whose directory is missing is kept, marked unavailable and never selected
// in place of an available one.
func Test_settings_to_state__unavailable(t *testing.T) {
	available := test_dir(t, "here")
	missing := filepath.Join(t.TempDir(), "unmounted")
	settings := NewSettings()
	settings.AddonsDirList = []AddonsDir{MakeAddonsDir(missing), MakeAddonsDir(available)}
	settings, _ = validate_settings(settings, core.DirExists)
	app := test_app_with_settings(t, settings)

	state := addons_dir_results_in_state(app)
	assert.False(t, state[missing].Available())
	assert.True(t, state[available].Available())
	assert.Equal(t, available, FindSettings(app).Preferences.SelectedAddonsDir)
	assert.Equal(t, core.ITEM_CHILDREN_LOAD_FALSE, state[missing].ItemHasChildren())
	assert.True(t, app.GetResult(missing).Tags.Contains(core.TAG_MUTED))

	assert.Error(t, SelectAddonsDir(app, missing))

	// it remains in the settings when saved
	SaveSettings(app)
	assert.Len(t, saved_settings(t, app).AddonsDirList, 2)
}
