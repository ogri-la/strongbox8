package strongbox

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// settings can be saved irregardless of the data
func Test_save_settings(t *testing.T) {
	output_path := filepath.Join(t.TempDir(), "foo.json")
	settings := Settings{}
	err := save_settings_file(settings, output_path)
	assert.Nil(t, err)
	assert.FileExists(t, output_path)
}

// attempting to read from an obviously bad path fails as expected
func Test_read_settings_file__bad_path(t *testing.T) {
	cases := []string{
		"",               // empty
		" ",              // still empty
		"     ",          // really empty
		"asdfsa",         // not a path
		"   adsfasd    ", // not a path padding with whitespace
		"foo/bar",        // not absolute
		"/foo/bar",       // absolute, but dne
		"/foo/bar.yaml",  // absolute, but yaml, not json
		"/foo/bar.json",  // absolute, json, but dne
	}
	expected := Settings{}
	for _, given := range cases {
		actual, err := read_settings_file(given)
		assert.NotNil(t, err)
		assert.Equal(t, expected, actual)
	}
}

// settings files circa 0.9.0 can be loaded and parsed as expected
func Test_configure_settings_file__0_9(t *testing.T) {
	fixture := test_fixture_user_config_0_9_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil,
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-bar",
			SelectedCatalogue:        CAT_SHORT.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_LIGHT,
		},

		// deprecated
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

func Test_configure_settings_file__0_10(t *testing.T) {
	fixture := test_fixture_user_config_0_10_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil,
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-bar",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_LIGHT,
		},

		// deprecated
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)

}

func Test_configure_settings_file__0_11(t *testing.T) {
	fixture := test_fixture_user_config_0_11_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil,
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-bar",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

func Test_configure_settings_file__0_12(t *testing.T) {
	fixture := test_fixture_user_config_0_12_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil,
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-bar",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

func Test_configure_settings_file__1_0(t *testing.T) {
	fixture := test_fixture_user_config_1_0_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          nil,
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced compound game tracks
func Test_configure_settings_file__3_1(t *testing.T) {
	fixture := test_fixture_user_config_3_1_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: false}, // compound game track
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced new themes
func Test_configure_settings_file__3_2(t *testing.T) {
	fixture := test_fixture_user_config_3_2_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: false}, // compound game track
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced classic-tbc gametrack
func Test_configure_settings_file__4_1(t *testing.T) {
	fixture := test_fixture_user_config_4_1_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns:          COL_LIST_DEFAULT,
			SelectedGUITheme:         GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced custom column lists
func Test_configure_settings_file__4_7(t *testing.T) {
	fixture := test_fixture_user_config_4_7_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"source",
				"name",
				"description",
				"available-version",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced the github catalogue
func Test_configure_settings_file__4_9(t *testing.T) {
	fixture := test_fixture_user_config_4_9_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"source",
				"name",
				"description",
				"available-version",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced new columns
func Test_configure_settings_file__5_0(t *testing.T) {
	fixture := test_fixture_user_config_5_0_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(false),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"source",
				"name",
				"description",
				"installed-version",
				"available-version",
				"game-version",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced new preference to keep user catalogue updated
func Test_configure_settings_file__6_0(t *testing.T) {
	fixture := test_fixture_user_config_6_0_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(true),
			KeepUserCatalogueUpdated: Ptr(true),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"source",
				"name",
				"description",
				"installed-version",
				"available-version",
				"game-version",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced new preference to check for updates
func Test_configure_settings_file__7_0(t *testing.T) {
	fixture := test_fixture_user_config_7_0_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_RETAIL, Strict: false},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(false),
			KeepUserCatalogueUpdated: Ptr(true),
			SelectedAddonsDir:        "/tmp/.strongbox-foo",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"source",
				"name",
				"description",
				"installed-version",
				"available-version",
				"game-version",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_GREEN,
		},

		// deprecated. to be removed in 10.0
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}

// introduced new preference to check for updates
func Test_configure_settings_file__8_0(t *testing.T) {
	fixture := test_fixture_user_config_8_0_0
	expected := Settings{
		AddonsDirList: []AddonsDir{
			{Path: "/tmp/.strongbox-bar", GameTrackID: GAMETRACK_RETAIL, Strict: true},
			{Path: "/tmp/.strongbox-foo", GameTrackID: GAMETRACK_CLASSIC_TBC, Strict: true},
		},
		CatalogueLocationList: DEFAULT_CATALOGUE_LOC_LIST,
		Preferences: Preferences{
			AddonZipsToKeep:          Ptr(uint8(3)),
			CheckForUpdate:           Ptr(false),
			KeepUserCatalogueUpdated: Ptr(true),
			SelectedAddonsDir:        "/tmp/.strongbox-bar",
			SelectedCatalogue:        CAT_FULL.Name,
			SelectedColumns: []string{
				"starred",
				"browse-local",
				"source",
				"name",
				"description",
				"combined-version",
				"updated-date",
				"uber-button",
			},
			SelectedGUITheme: GUI_THEME_DARK_ORANGE,
		},

		// deprecated
		DeprecatedGUITheme:          "",
		DeprecatedSelectedAddonDir:  "",
		DeprecatedSelectedCatalogue: "",
	}
	raw_settings, err := read_settings_file(fixture)
	assert.Nil(t, err)

	actual := configure_settings(raw_settings)
	assert.Equal(t, expected, actual)
}
