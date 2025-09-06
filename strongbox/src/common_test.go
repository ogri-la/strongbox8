package strongbox

import (
	"bw/core"
	"log/slog"
	"os"
	"path/filepath"
)

// the path that is returned is relative to the directory the test is
// being executed in.
func test_fixture_path(fixture_name string) string {
	p, err := filepath.Abs(filepath.Join("testdata", fixture_name))
	if err != nil {
		panic("failed to create an absolute path to a test fixture")
	}
	return p
}

// returns the contents of `fixture_name` as a byte slice
func test_fixture_bytes(fixture_name string) []byte {
	bytes, err := os.ReadFile(test_fixture_path(fixture_name))
	if err != nil {
		slog.Error("failed to read test fixture", "fixture", fixture_name, "error", err.Error())
		panic("programming error")
	}
	return bytes
}

//

// absolutely minimal addon
var test_fixture_everyaddon_minimal = test_fixture_path("zipfiles/everyaddon--1-2-3")

// absolutely minimal addon .zip file.
// should contain just enough to be installed and that's it.
// probably won't change much over time.
var test_fixture_everyaddon_minimal_zip = test_fixture_path("zipfiles/everyaddon--1-2-3.zip")

// slightest tweak to the minimal fixture that can be used as an update
var test_fixture_everyaddon_minimal_update_zip = test_fixture_path("zipfiles/everyaddon--1-2-4.zip")

// path to the kitchen sink test fixture
var test_fixture_everyaddon_maximal = test_fixture_path("zipfiles/everyaddon--7-8-9")

// absolutely packed out addon .zip file
// should contain every property and every feature that might ever be seen.
// will probably change a lot over time.
var test_fixture_everyaddon_maximal_zip = test_fixture_path("zipfiles/everyaddon--7-8-9.zip")

// some addon 'EveryOtherAddon' that bundles a copy of EveryAddon,
// creating a mutual dependency if EveryAddon 1.2.3 is installed,
// but also entirely replacing EveryAddon 1.2.3.
var test_fixture_everyotheraddon_minimal_zip = test_fixture_path("zipfiles/everyotheraddon--2-3-4.zip")

// some addon 'EveryOtherAddon' that bundles a copy of EveryAddon,
// creating a mutual dependency if EveryAddon 7.8.9 is installed
var test_fixture_everyotheraddon_maximal_zip = test_fixture_path("zipfiles/everyotheraddon--2-3-4.zip")

// standard nfo file circa 7.0 with source-id fields as integers
var test_fixture_nfo_single_ints_json = test_fixture_bytes("nfofiles/single_with_ints.json")

// standard nfo file 8.0+ with source-id fields as strings only
var test_fixture_nfo_single_strs_json = test_fixture_bytes("nfofiles/single_with_strs.json")

// unmarshalled nfo file
var test_fixture_nfo_single = NFO{
	InstalledVersion:     "1.2.1",
	InstalledGameTrackID: GAMETRACK_RETAIL,
	Name:                 "EveryAddon",
	GroupID:              "https://foo.bar",
	Primary:              true,
	Source:               SOURCE_CURSEFORGE,
	SourceID:             "123", // string!
	SourceMapList: []SourceMap{
		{
			Source:   SOURCE_CURSEFORGE,
			SourceID: "123", // string!
		},
	},
}

var test_fixture_nfo_multi_mixed_json = test_fixture_bytes("nfofiles/multi_with_mixed.json")

// unmarshalled nfo file
var test_fixture_nfo_multi = []NFO{
	test_fixture_nfo_single,
	{
		InstalledVersion:     "2.3.2",
		InstalledGameTrackID: GAMETRACK_CLASSIC,
		Name:                 "EveryAddon",
		GroupID:              "https://bar.baz",
		Primary:              true,
		Source:               SOURCE_WOWI,
		SourceID:             "321", // string!
		SourceMapList: []SourceMap{
			{
				Source:   SOURCE_WOWI,
				SourceID: "321", // string!
			},
		},
	},
}

var test_fixture_user_config_0_9_0 = test_fixture_path("config/user-config-0.9.json")
var test_fixture_user_config_0_10_0 = test_fixture_path("config/user-config-0.10.json")
var test_fixture_user_config_0_11_0 = test_fixture_path("config/user-config-0.11.json")
var test_fixture_user_config_0_12_0 = test_fixture_path("config/user-config-0.12.json")
var test_fixture_user_config_1_0_0 = test_fixture_path("config/user-config-1.0.json")
var test_fixture_user_config_3_1_0 = test_fixture_path("config/user-config-3.1.json")
var test_fixture_user_config_3_2_0 = test_fixture_path("config/user-config-3.2.json")
var test_fixture_user_config_4_1_0 = test_fixture_path("config/user-config-4.1.json")
var test_fixture_user_config_4_7_0 = test_fixture_path("config/user-config-4.7.json")
var test_fixture_user_config_4_9_0 = test_fixture_path("config/user-config-4.9.json")
var test_fixture_user_config_5_0_0 = test_fixture_path("config/user-config-5.0.json")
var test_fixture_user_config_6_0_0 = test_fixture_path("config/user-config-6.0.json")
var test_fixture_user_config_7_0_0 = test_fixture_path("config/user-config-7.0.json")
var test_fixture_user_config_8_0_0 = test_fixture_path("config/user-config-8.0.json")

//

var test_fixture_catalogue_loc = CatalogueLocation{Name: "test", Label: "Test", Source: ""}
var test_fixture_catalogue_file = test_fixture_path("catalogues/catalogue.json")
var test_fixture_catalogue, _ = read_catalogue_file(test_fixture_catalogue_loc, test_fixture_catalogue_file)

//

// returns a `core.App` good for testing with.
func DummyApp() *core.App {
	app := core.NewApp()
	app.Downloader = core.MakeDummyDownloader(nil)
	return app
}

func DummyApp2(tmpdir PathToDir) (*core.App, func()) {

	// todo: the envvars above are not preventing the catalogue from loading

	///tmpdir := t.TempDir()

	data_dir := filepath.Join(tmpdir, "xdg-data")     // "/tmp/rand/xdg-data"
	config_dir := filepath.Join(tmpdir, "xdg-config") // "/tmp/rand/xdg-config"

	prev_data_dir := os.Getenv("XDG_DATA_HOME")
	prev_config_dir := os.Getenv("XDG_CONFIG_HOME")

	os.Setenv("XDG_DATA_HOME", data_dir)
	os.Setenv("XDG_CONFIG_HOME", config_dir)

	addons_dir := filepath.Join(tmpdir, "addons") // "/tmp/rand/addons"
	core.MakeDirs(addons_dir)

	// ---

	app := core.NewApp()
	app.Downloader = core.MakeDummyDownloader(nil)
	go app.ProcessUpdateLoop()

	strongbox := Provider(app)
	app.RegisterProvider(strongbox)
	app.StartProviders()

	return app, func() {
		// not sure if necessary but seems like good hygiene
		os.Setenv("XDG_DATA_HOME", prev_data_dir)
		os.Setenv("XDG_CONFIG_HOME", prev_config_dir)

		app.Stop()
	}
}

// installs the minimal EveryAddon into the given AddonsDir
func InstallAddonHelper(app *core.App, ad AddonsDir) error {
	ca := test_fixture_catalogue.AddonSummaryList[0]
	sul := []SourceUpdate{}
	a := MakeAddonFromCatalogueAddon(ad, ca, sul)

	zipfile := test_fixture_everyaddon_minimal_zip

	opts := InstallOpts{}
	err := install_addon_guard(app, ad, a, zipfile, opts) // be warned: this called LoadAllInstalledAddonsToState
	if err != nil {
		return err
	}

	//Reconcile(app)
	return nil
}
