package strongbox

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func always_available(string) bool { return true }

// returns the default settings changed by `fn`, for building expected values.
func expected_settings(fn func(s *Settings)) Settings {
	s := NewSettings()
	fn(&s)
	return s
}

var bar = "/tmp/.strongbox-bar"
var foo = "/tmp/.strongbox-foo"

// every historical settings file migrates to the expected settings.
// clj: `config_test.clj/load-settings-*`
func Test_parse_settings__fixtures(t *testing.T) {
	cases := []struct {
		fixture  string
		format   settings_format
		expected Settings
	}{
		{test_fixture_user_config_0_9_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = bar
		})},
		{test_fixture_user_config_0_10_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = bar
			s.Preferences.SelectedCatalogue = "full"
		})},
		{test_fixture_user_config_0_11_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = bar
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK
		})},
		{test_fixture_user_config_0_12_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = bar
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK
		})},
		{test_fixture_user_config_1_0_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK
		})},
		{test_fixture_user_config_3_1_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: false}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK
			s.Preferences.AddonZipsToKeep = new(3)
		})},
		{test_fixture_user_config_3_2_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: false}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC, Strict: true}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
		})},
		{test_fixture_user_config_4_1_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
		})},
		{test_fixture_user_config_4_7_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
			s.Preferences.SelectedColumns = []string{"source", "name", "description", "available-version", "uber-button"}
		})},
		{test_fixture_user_config_4_9_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
			s.Preferences.SelectedColumns = []string{"source", "name", "description", "available-version", "uber-button"}
		})},
		{test_fixture_user_config_5_0_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
		})},
		{test_fixture_user_config_6_0_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
			s.Preferences.KeepUserCatalogueUpdated = true
		})},
		{test_fixture_user_config_7_0_0, SETTINGS_FORMAT_V7, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_RETAIL, Strict: false}}
			s.Preferences.SelectedAddonsDir = foo
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_GREEN
			s.Preferences.AddonZipsToKeep = new(3)
			s.Preferences.KeepUserCatalogueUpdated = true
			s.Preferences.CheckForUpdate = false
		})},
		{test_fixture_user_config_8_0_0, SETTINGS_FORMAT_V8, expected_settings(func(s *Settings) {
			s.AddonsDirList = []AddonsDir{{Path: bar, GameTrackID: GAMETRACK_RETAIL, Strict: true}, {Path: foo, GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true}}
			s.Preferences.SelectedAddonsDir = bar
			s.Preferences.SelectedCatalogue = "full"
			s.Preferences.SelectedGUITheme = GUI_THEME_DARK_ORANGE
			s.Preferences.AddonZipsToKeep = new(3)
			s.Preferences.KeepUserCatalogueUpdated = true
			s.Preferences.CheckForUpdate = false
			s.Preferences.SelectedColumns = []string{"starred", "browse-local", "source", "name", "description", "combined-version", "updated-date", "uber-button"}
		})},
	}

	for _, c := range cases {
		t.Run(filepath.Base(c.fixture), func(t *testing.T) {
			given, err := os.ReadFile(c.fixture)
			assert.NoError(t, err)
			actual, err := parse_settings(given, always_available)
			assert.NoError(t, err)
			assert.Equal(t, c.format, actual.Format)
			assert.False(t, actual.ReadOnly)
			assert.Equal(t, c.expected, actual.Settings)
			for _, issue := range actual.Issues {
				assert.NotEqual(t, slog.LevelWarn, issue.Level, issue.Message)
			}
		})
	}
}

// returns the settings in `given` JSON, failing the test when it is not valid.
func must_parse_settings(t *testing.T, given string) parsed_settings {
	t.Helper()
	actual, err := parse_settings([]byte(given), always_available)
	assert.NoError(t, err)
	return actual
}

// returns `true` when `issue_list` has a WARN.
func has_warning(issue_list []Issue) bool {
	return slices.ContainsFunc(issue_list, func(i Issue) bool { return i.Level == slog.LevelWarn })
}

// clj: `config_test.clj/handle-install-dir`
func Test_parse_settings__install_dir(t *testing.T) {
	actual := must_parse_settings(t, `{"install-dir": "/tmp/addons"}`)
	assert.Equal(t, []AddonsDir{{Path: "/tmp/addons", GameTrackID: GAMETRACK_RETAIL, Strict: true}}, actual.Settings.AddonsDirList)

	actual = must_parse_settings(t, `{"install-dir": "/tmp/addons", "addon-dir-list": [{"addon-dir": "/tmp/addons", "game-track": "classic"}]}`)
	assert.Equal(t, []AddonsDir{{Path: "/tmp/addons", GameTrackID: GAMETRACK_CLASSIC, Strict: true}}, actual.Settings.AddonsDirList)
}

// clj: `config_test.clj/catalogue-location-list`
func Test_parse_settings__catalogue_location_list(t *testing.T) {
	// missing gives the default
	actual := must_parse_settings(t, `{}`)
	assert.Equal(t, DEFAULT_CATALOGUE_LOC_LIST, actual.Settings.CatalogueLocationList)

	// empty is kept empty, and no catalogue is selected
	actual = must_parse_settings(t, `{"catalogue-location-list": []}`)
	assert.Empty(t, actual.Settings.CatalogueLocationList)
	assert.Equal(t, "", actual.Settings.Preferences.SelectedCatalogue)

	// not a list
	actual = must_parse_settings(t, `{"catalogue-location-list": "foo"}`)
	assert.Empty(t, actual.Settings.CatalogueLocationList)
	assert.True(t, has_warning(actual.Issues))

	// invalid entries are discarded, valid ones kept
	actual = must_parse_settings(t, `{"catalogue-location-list": [1, {}, {"name": "x"}, {"name": "mine", "label": "Mine", "source": "https://example.org/c.json"}]}`)
	assert.Equal(t, []CatalogueLocation{{Name: "mine", Label: "Mine", Source: "https://example.org/c.json"}}, actual.Settings.CatalogueLocationList)
	assert.True(t, has_warning(actual.Issues))

	// dead hosts are removed
	actual = must_parse_settings(t, `{"catalogue-location-list": [
		{"name": "short", "label": "Short", "source": "https://example.org/short.json"},
		{"name": "curseforge", "label": "C", "source": "https://example.org/c.json"},
		{"name": "tukui", "label": "T", "source": "https://example.org/t.json"}]}`)
	assert.Equal(t, []string{"short"}, []string{actual.Settings.CatalogueLocationList[0].Name})
	assert.Len(t, actual.Settings.CatalogueLocationList, 1)

	// a non-https source is discarded
	actual = must_parse_settings(t, `{"catalogue-location-list": [{"name": "x", "label": "X", "source": "http://example.org/c.json"}]}`)
	assert.Empty(t, actual.Settings.CatalogueLocationList)
}

// clj: `config_test.clj/invalid-addon-dirs-in-cfg`, changed: missing directories are kept
func Test_parse_settings__addon_dirs(t *testing.T) {
	given := `{"addon-dir-list": [
		{"addon-dir": "/tmp/a", "game-track": "classic-bfa"},
		{"addon-dir": "relative/path", "game-track": "retail"},
		{"addon-dir": "", "game-track": "retail"},
		{"addon-dir": "/tmp/b", "game-track": "retail"},
		{"addon-dir": "/tmp/b", "game-track": "classic"},
		"not an object",
		{"addon-dir": "/tmp/missing", "game-track": "classic-mists"},
		{"addon-dir": "/tmp/forever", "game-track": "forever", "strict?": false}]}`
	actual, err := parse_settings([]byte(given), func(path string) bool { return path != "/tmp/missing" })
	assert.NoError(t, err)
	expected := []AddonsDir{
		{Path: "/tmp/b", GameTrackID: GAMETRACK_RETAIL, Strict: true},
		{Path: "/tmp/missing", GameTrackID: GAMETRACK_CLASSIC_MISTS, Strict: true},
		{Path: "/tmp/forever", GameTrackID: GAMETRACK_FOREVER, Strict: false},
	}
	assert.Equal(t, expected, actual.Settings.AddonsDirList)
	issues := []string{}
	for _, issue := range actual.Issues {
		if issue.Level == slog.LevelWarn {
			issues = append(issues, issue.Message)
		}
	}
	assert.True(t, slices.ContainsFunc(issues, func(m string) bool {
		return strings.Contains(m, "/tmp/a") && strings.Contains(m, "classic-bfa")
	}), "a WARN names the discarded addons dir and its game track: %v", issues)
	assert.True(t, has_warning(actual.Issues))
	assert.Equal(t, "/tmp/b", actual.Settings.Preferences.SelectedAddonsDir)
}

// clj: `config_test.clj/handle-selected-addon-dir`
func Test_parse_settings__selected_addon_dir(t *testing.T) {
	// not in the list falls back to the first
	actual := must_parse_settings(t, `{"selected-addon-dir": "/tmp/nope", "addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "retail"}]}`)
	assert.Equal(t, "/tmp/a", actual.Settings.Preferences.SelectedAddonsDir)

	// an empty list selects nothing
	actual = must_parse_settings(t, `{"selected-addon-dir": "/tmp/nope"}`)
	assert.Equal(t, "", actual.Settings.Preferences.SelectedAddonsDir)

	// an unavailable selection falls back to the first available
	given := `{"selected-addon-dir": "/tmp/a", "addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "retail"}, {"addon-dir": "/tmp/b", "game-track": "retail"}]}`
	parsed, err := parse_settings([]byte(given), func(path string) bool { return path == "/tmp/b" })
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/b", parsed.Settings.Preferences.SelectedAddonsDir)
}

// clj: `config_test.clj/convert-compound-game-track`
func Test_convert_compound_game_track(t *testing.T) {
	assert.Equal(t, GAMETRACK_RETAIL, convert_compound_game_track(GAMETRACK_RETAIL_CLASSIC))
	assert.Equal(t, GAMETRACK_CLASSIC, convert_compound_game_track(GAMETRACK_CLASSIC_RETAIL))
	assert.Equal(t, GAMETRACK_CLASSIC_TBC, convert_compound_game_track(GAMETRACK_CLASSIC_TBC))

	actual := must_parse_settings(t, `{"addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "classic-retail"}]}`)
	assert.Equal(t, []AddonsDir{{Path: "/tmp/a", GameTrackID: GAMETRACK_CLASSIC, Strict: false}}, actual.Settings.AddonsDirList)

	// strictness given explicitly wins over the compound default
	actual = must_parse_settings(t, `{"addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "retail-classic", "strict?": true}]}`)
	assert.Equal(t, []AddonsDir{{Path: "/tmp/a", GameTrackID: GAMETRACK_RETAIL, Strict: true}}, actual.Settings.AddonsDirList)
}

func Test_parse_settings__bad_preferences(t *testing.T) {
	actual := must_parse_settings(t, `{"preferences": {"addon-zips-to-keep": "three", "check-for-update": false, "keep-user-catalogue-updated": "yes"}}`)
	assert.Nil(t, actual.Settings.Preferences.AddonZipsToKeep)
	assert.False(t, actual.Settings.Preferences.CheckForUpdate) // good values kept
	assert.False(t, actual.Settings.Preferences.KeepUserCatalogueUpdated)
	assert.True(t, has_warning(actual.Issues))

	actual = must_parse_settings(t, `{"preferences": {"addon-zips-to-keep": -1}}`)
	assert.Nil(t, actual.Settings.Preferences.AddonZipsToKeep)

	actual = must_parse_settings(t, `{"preferences": {"addon-zips-to-keep": null}}`)
	assert.Nil(t, actual.Settings.Preferences.AddonZipsToKeep)

	actual = must_parse_settings(t, `{"preferences": "nope", "addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "retail"}]}`)
	assert.Len(t, actual.Settings.AddonsDirList, 1)
	assert.True(t, has_warning(actual.Issues))
}

func Test_parse_settings__defaults(t *testing.T) {
	actual := must_parse_settings(t, `{}`)
	assert.Nil(t, actual.Settings.Preferences.AddonZipsToKeep)
	assert.False(t, actual.Settings.Preferences.KeepUserCatalogueUpdated)
	assert.True(t, actual.Settings.Preferences.CheckForUpdate)
	assert.Equal(t, "short", actual.Settings.Preferences.SelectedCatalogue)
}

func Test_parse_settings__unknown_keys_removed(t *testing.T) {
	actual := must_parse_settings(t, `{"debug?": true, "frobnicate": 1}`)
	messages := []string{}
	for _, issue := range actual.Issues {
		assert.Equal(t, slog.LevelDebug, issue.Level)
		messages = append(messages, issue.Message)
	}
	assert.Contains(t, messages, "removing unknown setting: debug?")
	assert.Contains(t, messages, "removing unknown setting: frobnicate")
}

func Test_parse_settings__not_an_object(t *testing.T) {
	for _, given := range []string{`{"addon-dir-list": [`, `[]`, `1`, `"x"`, ``} {
		_, err := parse_settings([]byte(given), always_available)
		assert.Error(t, err, given)
	}
}

func Test_parse_settings__newer_version_read_only(t *testing.T) {
	actual := must_parse_settings(t, `{"spec": {"version": 2}, "addon-dir-list": [{"addon-dir": "/tmp/a", "game-track": "retail", "strict": true}]}`)
	assert.True(t, actual.ReadOnly)
	assert.Len(t, actual.Settings.AddonsDirList, 1)
	assert.True(t, has_warning(actual.Issues))
}

// --- properties

// returns every settings fixture's contents.
func all_settings_fixtures(t *testing.T) map[string][]byte {
	t.Helper()
	fixtures := map[string][]byte{}
	for _, path := range []string{
		test_fixture_user_config_0_9_0, test_fixture_user_config_0_10_0, test_fixture_user_config_0_11_0,
		test_fixture_user_config_0_12_0, test_fixture_user_config_1_0_0, test_fixture_user_config_3_1_0,
		test_fixture_user_config_3_2_0, test_fixture_user_config_4_1_0, test_fixture_user_config_4_7_0,
		test_fixture_user_config_4_9_0, test_fixture_user_config_5_0_0, test_fixture_user_config_6_0_0,
		test_fixture_user_config_7_0_0, test_fixture_user_config_8_0_0,
	} {
		b, err := os.ReadFile(path)
		assert.NoError(t, err)
		fixtures[filepath.Base(path)] = b
	}
	return fixtures
}

// migrating migrated settings changes nothing, and saving then loading is the identity.
func Test_parse_settings__idempotent(t *testing.T) {
	for name, given := range all_settings_fixtures(t) {
		first, err := parse_settings(given, always_available)
		assert.NoError(t, err, name)

		saved, err := json.Marshal(first.Settings)
		assert.NoError(t, err)
		second, err := parse_settings(saved, always_available)
		assert.NoError(t, err, name)

		assert.Equal(t, SETTINGS_FORMAT_V8, second.Format, name)
		assert.Equal(t, first.Settings, second.Settings, name)
		assert.Empty(t, second.Issues, name)
	}
}

// generated settings survive a save and load unchanged.
func Test_parse_settings__generated_round_trip(t *testing.T) {
	track_list := SUPPORTED_GAME_TRACKS_LIST
	slices.Sort(track_list)
	for i := range 50 {
		given := NewSettings()
		for j := range i % 6 {
			given.AddonsDirList = append(given.AddonsDirList, AddonsDir{
				Path:        filepath.Join("/tmp", "dir-"+string(rune('a'+j))),
				GameTrackID: track_list[(i+j)%len(track_list)],
				Strict:      (i+j)%2 == 0,
			})
		}
		if len(given.AddonsDirList) > 0 {
			given.Preferences.SelectedAddonsDir = given.AddonsDirList[len(given.AddonsDirList)-1].Path
		}
		if i%3 == 0 {
			given.Preferences.AddonZipsToKeep = new(i)
		}
		given.Preferences.CheckForUpdate = i%2 == 0
		given.Preferences.KeepUserCatalogueUpdated = i%4 == 0
		given.Preferences.SelectedCatalogue = given.CatalogueLocationList[i%len(given.CatalogueLocationList)].Name

		b, err := json.Marshal(given)
		assert.NoError(t, err)
		actual, err := parse_settings(b, always_available)
		assert.NoError(t, err)
		assert.Equal(t, given, actual.Settings)
	}
}

// --- files

// returns paths for strongbox 8 and strongbox 7 settings files in a temp dir.
func test_settings_paths(t *testing.T) (string, V7Paths, string) {
	t.Helper()
	root := t.TempDir()
	v7 := V7Paths{
		CfgFile:           filepath.Join(root, "strongbox", "config.json"),
		UserCatalogueFile: filepath.Join(root, "strongbox", "user-catalogue.json"),
	}
	return filepath.Join(root, "strongbox8", "config.json"), v7, filepath.Join(root, "strongbox8", "user-catalogue.json")
}

func Test_write_atomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "file.json")
	assert.NoError(t, write_atomic(path, []byte("one")))
	assert.NoError(t, write_atomic(path, []byte("two")))
	actual, _ := os.ReadFile(path)
	assert.Equal(t, "two", string(actual))
	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1, "no temporary files left behind")
}

func Test_save_settings_file(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	given := NewSettings()
	given.Spec.Version = 0
	assert.NoError(t, save_settings_file(given, path))
	b, _ := os.ReadFile(path)
	actual, err := parse_settings(b, always_available)
	assert.NoError(t, err)
	assert.Equal(t, SETTINGS_VERSION, actual.Settings.Spec.Version)
}

func Test_load_settings__corrupt_file_preserved(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	given := `{"addon-dir-list": [`
	os.MkdirAll(filepath.Dir(cfg_file), 0o755)
	os.WriteFile(cfg_file, []byte(given), 0o644)
	now := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)

	actual := load_settings(cfg_file, v7, uc_file, always_available, now)
	assert.Equal(t, NewSettings(), actual.Settings)
	assert.False(t, actual.ReadOnly)

	preserved, err := os.ReadFile(cfg_file + ".20261005T010203Z.invalid")
	assert.NoError(t, err)
	assert.Equal(t, given, string(preserved))
}

func Test_load_settings__import_v7(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	os.MkdirAll(filepath.Dir(v7.CfgFile), 0o755)
	v7_config := test_fixture_bytes("config/user-config-7.0.json")
	os.WriteFile(v7.CfgFile, v7_config, 0o644)
	v7_catalogue := []byte(`{"spec": {"version": 2}, "datestamp": "2020-01-01", "total": 0, "addon-summary-list": []}`)
	os.WriteFile(v7.UserCatalogueFile, v7_catalogue, 0o644)

	actual := load_settings(cfg_file, v7, uc_file, always_available, time.Now())
	assert.Equal(t, v7.CfgFile, actual.Source)
	assert.Len(t, actual.Settings.AddonsDirList, 2)
	assert.Equal(t, foo, actual.Settings.Preferences.SelectedAddonsDir)
	assert.Equal(t, "full", actual.Settings.Preferences.SelectedCatalogue)

	// strongbox 7's files are untouched
	after, _ := os.ReadFile(v7.CfgFile)
	assert.Equal(t, v7_config, after)

	// the user catalogue is copied
	copied, _ := os.ReadFile(uc_file)
	assert.Equal(t, v7_catalogue, copied)
}

func Test_load_settings__later_runs_ignore_v7(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	os.MkdirAll(filepath.Dir(v7.CfgFile), 0o755)
	os.WriteFile(v7.CfgFile, test_fixture_bytes("config/user-config-7.0.json"), 0o644)

	mine := NewSettings()
	mine.AddonsDirList = []AddonsDir{{Path: "/tmp/mine", GameTrackID: GAMETRACK_RETAIL, Strict: true}}
	assert.NoError(t, save_settings_file(mine, cfg_file))

	actual := load_settings(cfg_file, v7, uc_file, always_available, time.Now())
	assert.Equal(t, cfg_file, actual.Source)
	assert.Equal(t, []AddonsDir{{Path: "/tmp/mine", GameTrackID: GAMETRACK_RETAIL, Strict: true}}, actual.Settings.AddonsDirList)
}

func Test_load_settings__unreadable_v7(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	os.MkdirAll(filepath.Dir(v7.CfgFile), 0o755)
	given := []byte("{not json")
	os.WriteFile(v7.CfgFile, given, 0o644)

	var actual loaded_settings
	log := capture_log(func() { actual = load_settings(cfg_file, v7, uc_file, always_available, time.Now()) })
	assert.Contains(t, log, "level=WARN")
	assert.Contains(t, log, v7.CfgFile)
	assert.Equal(t, NewSettings(), actual.Settings)
	after, _ := os.ReadFile(v7.CfgFile)
	assert.Equal(t, given, after)
	entries, _ := os.ReadDir(filepath.Dir(v7.CfgFile))
	assert.Len(t, entries, 1, "nothing written beside strongbox 7's settings")
}

func Test_load_settings__fresh(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	actual := load_settings(cfg_file, v7, uc_file, always_available, time.Now())
	assert.Equal(t, NewSettings(), actual.Settings)
	assert.Equal(t, "", actual.Source)
}

func Test_load_settings__newer_version(t *testing.T) {
	cfg_file, v7, uc_file := test_settings_paths(t)
	os.MkdirAll(filepath.Dir(cfg_file), 0o755)
	os.WriteFile(cfg_file, []byte(`{"spec": {"version": 99}}`), 0o644)
	actual := load_settings(cfg_file, v7, uc_file, always_available, time.Now())
	assert.True(t, actual.ReadOnly)
}

// settings written by a newer strongbox are never overwritten.
func Test_SaveSettings__read_only(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	cfg_file := get_paths(app)["strongbox.paths.cfg-file"]
	os.MkdirAll(filepath.Dir(cfg_file), 0o755)
	given := []byte(`{"spec": {"version": 99}, "addon-dir-list": []}`)
	os.WriteFile(cfg_file, given, 0o644)

	LoadSettings(app)
	apply_settings(app, func(s Settings) Settings { s.Preferences.CheckForUpdate = false; return s })
	SaveSettings(app)
	actual, _ := os.ReadFile(cfg_file)
	assert.Equal(t, given, actual)
}
