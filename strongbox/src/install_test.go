package strongbox

// installing, updating and uninstalling addons.
// clj: `zip_test.clj`, `core_test.clj/install-addon-guard*`, `install-addon*`,
// `uninstall-*`, `install-addons-with-mutual-dependencies*`, `cli_test.clj/import-addon*`,
// `install-addon-from-file*`, `install-update-these-in-parallel--bad-download`

import (
	"archive/zip"
	"bw/core"
	"bw/http_utils"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// --- zips

// clj: `zip_test.clj/valid-zip-file?`, `valid-addon-zip-file?`
func Test_valid_addon_zip_file(t *testing.T) {
	for _, name := range []string{"v7/bad-empty.zip", "v7/bad-truncated.zip"} {
		_, err := inspect_zipfile(test_fixture_path(name))
		assert.Error(t, err, name)
	}

	report, err := inspect_zipfile(test_fixture_path("v7/empty.zip"))
	assert.NoError(t, err, "a valid zip")
	assert.ErrorContains(t, valid_addon_zip_file(report), "top level files: empty.txt")

	report, _ = inspect_zipfile(test_fixture_path("v7/everyaddon--1-2-3--non-addon-tld.zip"))
	assert.ErrorContains(t, valid_addon_zip_file(report), "__MACOS")

	report, _ = inspect_zipfile(test_fixture_everyaddon_minimal_zip)
	assert.NoError(t, valid_addon_zip_file(report))
}

// many zip tools write no directory entries, the directories are implied.
func Test_inspect_zipfile__no_directory_entries(t *testing.T) {
	zip_path := filepath.Join(t.TempDir(), "x.zip")
	fh, _ := os.Create(zip_path)
	zw := zip.NewWriter(fh)
	w, _ := zw.Create("EveryAddon/EveryAddon.toc")
	w.Write([]byte("## Title: EveryAddon\n"))
	zw.Close()
	fh.Close()

	report, err := inspect_zipfile(zip_path)
	assert.NoError(t, err)
	assert.Equal(t, mapset.NewSet("EveryAddon"), report.TopLevelDirs)
	assert.NoError(t, valid_addon_zip_file(report))
}

func Test_inspect_zipfile__path_traversal(t *testing.T) {
	zip_path := filepath.Join(t.TempDir(), "evil.zip")
	test_write_zip(t, zip_path, test_file_tree{
		"EveryAddon/EveryAddon.toc": "## Title: x\n",
		"EveryAddon/../../evil.lua": "boom",
	})
	report, err := inspect_zipfile(zip_path)
	assert.NoError(t, err)
	assert.ErrorContains(t, valid_addon_zip_file(report), "outside the addons directory")

	dest := t.TempDir()
	_, err = unzip_file(zip_path, filepath.Join(dest, "AddOns"))
	assert.Error(t, err)
	assert.NoFileExists(t, filepath.Join(dest, "evil.lua"))
}

// clj: `zip_test.clj/unzip-file`
func Test_unzip_file__corrupt(t *testing.T) {
	_, err := unzip_file(test_fixture_path("v7/bad-truncated.zip"), t.TempDir())
	assert.Error(t, err)
}

// clj: `zip_test.clj/suspicious-subdirs`
func Test_inconsistently_prefixed(t *testing.T) {
	cases := []struct {
		given    []string
		expected []string
	}{
		{[]string{"EveryAddon"}, nil},
		{[]string{"EveryAddon", "EveryAddon-Bundled"}, nil},
		{[]string{"Foo", "Bar"}, nil}, // ambiguous
		{[]string{"Foo", "Foo-Bar", "bup"}, []string{"bup"}},
		{[]string{"Auc-Advanced", "Auc-Stat-Histogram", "BeanCounter", "Enchantrix", "SlideBar"}, []string{"BeanCounter", "Enchantrix", "SlideBar"}},
		{[]string{"Altoholic", "Altoholic_A", "Altoholic_B", "Altoholic_C", "DataStore", "DataStore_A", "DataStore_B", "DataStore_C"}, nil},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, inconsistently_prefixed(mapset.NewSet(c.given...)), c.given)
	}
}

// a name `safe_entry_name` accepts never extracts outside the destination.
func FuzzZipEntryPath(f *testing.F) {
	for _, seed := range []string{"EveryAddon/x.lua", "../x", "a/../../b", "/etc/passwd", "C:\\x", "a\\..\\..\\b", "./a/b", "a//b"} {
		f.Add(seed)
	}
	dest := "/tmp/dest"
	f.Fuzz(func(t *testing.T, name string) {
		name = normalise_entry_name(name)
		if !safe_entry_name(name) {
			return
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			t.Errorf("safe name escapes the destination: %q => %q", name, target)
		}
	})
}

// clj: `addon_test.clj/determine-primary-subdir`
func Test_determine_primary_subdir__v7(t *testing.T) {
	cases := []struct {
		given    []string
		expected string
	}{
		{[]string{"Foo"}, "Foo"},
		{[]string{"Foo-Bar", "Foo"}, "Foo"},
		{[]string{"HealBot", "HealBot_de", "HealBot_Tips", "HealBot_br"}, "HealBot"},
		{[]string{"Foo", "Bar"}, ""},
		{[]string{"z", "az"}, ""},
	}
	for _, c := range cases {
		actual, _ := determine_primary_subdir(mapset.NewSet(c.given...))
		assert.Equal(t, c.expected, actual, c.given)
	}
	_, err := determine_primary_subdir(mapset.NewSet[string]())
	assert.Error(t, err)
}

// --- planning and installing zips

// returns a zip of the generated addon `spec` in a temp dir.
func zip_of(t *testing.T, spec test_addon_spec) string {
	t.Helper()
	return test_gen_addon_zip(t, t.TempDir(), spec)
}

// returns the addon to install for `spec` as though from the catalogue, with an update.
func catalogue_target(ad AddonsDir, spec test_addon_spec) Addon {
	version := spec.Version
	if version == "" {
		version = "1.2.3"
	}
	ca := CatalogueAddon{URL: "https://example.org/" + spec.DirList[0], Name: strings.ToLower(spec.DirList[0]), Label: spec.DirList[0],
		Source: SOURCE_GITHUB, SourceID: FlexString("a/" + strings.ToLower(spec.DirList[0])), GameTrackIDList: []GameTrackID{GAMETRACK_RETAIL}}
	return MakeAddonFromCatalogueAddon(ad, ca, []SourceUpdate{su(version, GAMETRACK_RETAIL)})
}

// installs `spec` into `ad` as though from the catalogue.
func install_spec(t *testing.T, ad AddonsDir, spec test_addon_spec, opts InstallOpts) error {
	t.Helper()
	return install_zip(ad, catalogue_target(ad, spec), zip_of(t, spec), opts)
}

// returns the nfo data in `dir` of `ad`.
func read_nfo(t *testing.T, ad AddonsDir, dir string) NFOFile {
	t.Helper()
	f, err := read_nfo_file(filepath.Join(ad.Path, dir))
	assert.NoError(t, err, dir)
	return f
}

// clj: `core_test.clj/install-addon-guard--bundled-addon`
func Test_install_zip__bundled(t *testing.T) {
	ad := test_addons_dir(t)
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}}, InstallOpts{}))
	assert.Equal(t, []string{"EveryAddon", "EveryAddon-BundledAddon"}, test_dir_contents(t, ad.Path))

	primary, _ := read_nfo(t, ad, "EveryAddon").Top()
	bundled, _ := read_nfo(t, ad, "EveryAddon-BundledAddon").Top()
	assert.True(t, primary.Primary)
	assert.False(t, bundled.Primary)
	assert.Equal(t, primary.GroupID, bundled.GroupID)
	assert.Equal(t, "1.2.3", bundled.InstalledVersion)
	assert.Equal(t, SOURCE_GITHUB, bundled.Source)
}

// clj: `core_test.clj/install-addon-guard--bundled-addon-overwriting-ignored-addon`
func Test_install_zip__refuses_ignored(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon-BundledAddon"}, Version: "0.0.1"})
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon-BundledAddon", ".git"), 0o755)

	err := install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}}, InstallOpts{})
	assert.ErrorContains(t, err, "ignored")
	assert.Equal(t, []string{"EveryAddon-BundledAddon"}, test_dir_contents(t, ad.Path))
}

// clj: `core_test.clj/install-addon-guard--bundled-addon-overwriting-pinned-addon`
func Test_install_zip__refuses_pinned(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "1"}
	test_gen_addon(t, ad.Path, spec)
	nfo := test_addon_nfo(spec, "EveryAddon")
	nfo.PinnedVersion = "1.2.3"
	test_write_nfo(t, filepath.Join(ad.Path, "EveryAddon"), nfo)

	err := install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "2.0"}, InstallOpts{})
	assert.ErrorContains(t, err, "pinned")

	// with the pin removed on purpose
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "2.0"}, InstallOpts{UnpinPinned: true}))
	top, _ := read_nfo(t, ad, "EveryAddon").Top()
	assert.Equal(t, "", top.PinnedVersion)
}

// updating a pinned addon to its pinned version keeps the pin.
func Test_install_zip__update_to_pinned_version(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddon"}}
	assert.NoError(t, install_spec(t, ad, spec, InstallOpts{}))
	f := read_nfo(t, ad, "EveryAddon")
	assert.NoError(t, write_nfo_file(filepath.Join(ad.Path, "EveryAddon"), nfo_file_pin(f, "1.0.0")))

	installed, _ := LoadAllInstalledAddons(ad)
	target := installed[0]
	release := su("1.0.0", GAMETRACK_RETAIL)
	target.SourceUpdate = &release
	assert.NoError(t, install_zip(ad, target, zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "1.0.0"}), InstallOpts{}))
	top, _ := read_nfo(t, ad, "EveryAddon").Top()
	assert.Equal(t, "1.0.0", top.PinnedVersion)
	assert.Equal(t, "1.0.0", top.InstalledVersion)
}

// clj: `core_test.clj/install-addon--uninstall-fully-replaced-mutual-dependencies`
func Test_install_zip__completely_replaced(t *testing.T) {
	ad := test_addons_dir(t)
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddonOne"}}, InstallOpts{}))
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddonTwo"}}, InstallOpts{}))
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddonThree", "EveryAddonOne", "EveryAddonTwo"}}, InstallOpts{}))

	installed, _ := LoadAllInstalledAddons(ad)
	assert.Len(t, installed, 1)
	for _, dir := range []string{"EveryAddonOne", "EveryAddonTwo", "EveryAddonThree"} {
		f := read_nfo(t, ad, dir)
		assert.Len(t, f.Stack, 1, dir)
		assert.Equal(t, "https://example.org/EveryAddonThree", f.Stack[0].GroupID)
	}
}

// clj: `core_test.clj/install-addons-with-mutual-dependencies`
func Test_install_zip__partial_overwrite_is_shared(t *testing.T) {
	ad := test_addons_dir(t)
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Version: "0.1.2"}, InstallOpts{}))
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryOtherAddon", "EveryAddon-BundledAddon"}, Version: "5.6.7"}, InstallOpts{}))

	f := read_nfo(t, ad, "EveryAddon-BundledAddon")
	assert.Len(t, f.Stack, 2)
	assert.Equal(t, "https://example.org/EveryOtherAddon", f.Stack[1].GroupID)

	b, _ := os.ReadFile(nfo_path(filepath.Join(ad.Path, "EveryAddon-BundledAddon")))
	assert.Equal(t, byte('['), b[0], "written as a list strongbox 7 reads as shared")
	b, _ = os.ReadFile(nfo_path(filepath.Join(ad.Path, "EveryAddon")))
	assert.Equal(t, byte('{'), b[0])

	installed, _ := LoadAllInstalledAddons(ad)
	assert.Len(t, installed, 2)
}

// clj: `core_test.clj/uninstall-installed-addon`
func Test_install_zip__upgrade_removes_obsolete_dir(t *testing.T) {
	ad := test_addons_dir(t)
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Version: "0.1.2"}, InstallOpts{}))
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "1.2.3"}, InstallOpts{}))
	assert.Equal(t, []string{"EveryAddon"}, test_dir_contents(t, ad.Path))
}

// clj: `core_test.clj/install-addon-guard--invalid-zip-file`
func Test_install_zip__invalid_zip(t *testing.T) {
	ad := test_addons_dir(t)
	err := install_zip(ad, catalogue_target(ad, test_addon_spec{DirList: []string{"EveryAddon"}}), test_fixture_path("v7/bad-truncated.zip"), InstallOpts{})
	assert.Error(t, err)
	assert.Empty(t, test_dir_contents(t, ad.Path))
}

// a failed extraction writes no nfo data.
func Test_install_zip__extraction_fails(t *testing.T) {
	ad := test_addons_dir(t)
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon", "EveryAddon.lua"), 0o755) // a directory where a file goes
	err := install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon"}}, InstallOpts{})
	assert.Error(t, err)
	assert.NoFileExists(t, nfo_path(filepath.Join(ad.Path, "EveryAddon")))
}

// clj: `zip_test.clj` HealBot, `addon_test.clj/determine-primary-subdir`
func Test_install_zip__primary_and_suspicious(t *testing.T) {
	ad := test_addons_dir(t)
	assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"HealBot", "HealBot_de", "HealBot_Tips"}}, InstallOpts{}))
	top, _ := read_nfo(t, ad, "HealBot").Top()
	assert.True(t, top.Primary)
	tips, _ := read_nfo(t, ad, "HealBot_Tips").Top()
	assert.False(t, tips.Primary)

	installed, _ := LoadAllInstalledAddons(ad)
	report, _ := inspect_zipfile(zip_of(t, test_addon_spec{DirList: []string{"Auc-Advanced", "Auc-Stat-Histogram", "BeanCounter"}}))
	plan, err := plan_install(installed, catalogue_target(ad, test_addon_spec{DirList: []string{"Auc-Advanced"}}), report, InstallOpts{})
	assert.NoError(t, err)
	assert.Contains(t, plan.Messages, "Auc-Advanced will also install these addons: BeanCounter")
}

// clj: `addon_test.clj/zips-to-prune`, `core_test.clj/install-addon-guard--remove-multiple-zips`
func Test_zips_to_prune(t *testing.T) {
	given := []zip_file_info{
		{"everyaddon--1-2-1.zip", 1}, {"everyaddon--1-2-2.zip", 2}, {"everyaddon--1-2-3.zip", 3},
		{"everyaddon--1-2-4.zip", 4}, {"everyaddon--1-2-5.zip", 5},
		{"everyotheraddon--1-0.zip", 0}, {"notes.txt", 0},
	}
	assert.Nil(t, zips_to_prune(given, "EveryAddon", nil))
	assert.ElementsMatch(t, []string{"everyaddon--1-2-1.zip", "everyaddon--1-2-2.zip"}, zips_to_prune(given, "EveryAddon", new(3)))
	assert.Len(t, zips_to_prune(given, "EveryAddon", new(0)), 5)
}

// --- installing through the app

// returns routes for github repository `a/b` offering EveryAddon `version` for retail,
// and the zip it downloads.
func everyaddon_routes(t *testing.T, version string) map[string]http_utils.Fixture {
	t.Helper()
	zip_path := zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}, Version: version})
	zip_bytes, _ := os.ReadFile(zip_path)
	return map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Body: []byte(github_releases_json(version, "retail"))},
		"https://github.com/a/b/releases/download/" + version + "/EveryAddon-" + version + "-retail.zip": {Body: zip_bytes},
	}
}

var everyaddon_ca = CatalogueAddon{URL: "https://github.com/a/b", Name: "everyaddon", Label: "EveryAddon", Source: SOURCE_GITHUB,
	SourceID: "a/b", GameTrackIDList: []GameTrackID{GAMETRACK_RETAIL}, TagList: []string{}, UpdatedDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

func Test_InstallCatalogueAddon(t *testing.T) {
	app, path, ft := app_with_installed(t, everyaddon_routes(t, "1.2.3"))
	catalogue_to_state(app, Catalogue{AddonSummaryList: []CatalogueAddon{everyaddon_ca}}, Catalogue{})

	assert.NoError(t, InstallCatalogueAddon(app, everyaddon_ca))
	assert.Empty(t, ft.Unrouted())
	assert.DirExists(t, filepath.Join(path, "EveryAddon"))
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "https://github.com/a/b", top.GroupID)
	assert.Equal(t, "1.2.3", top.InstalledVersion)

	results := addon_results(app)
	assert.Len(t, results, 1)
	a := results[0].Item.(Addon)
	assert.NotNil(t, a.CatalogueAddon, "matched once installed")
	assert.False(t, Updateable(a))
	assert.FileExists(t, filepath.Join(path, "everyaddon--1-2-3.zip"), "zips are kept by default")
}

// clj: `core_test.clj/read-strange-catalogue--unknown-source`
func Test_InstallCatalogueAddon__unsupported_source(t *testing.T) {
	app, path, ft := app_with_installed(t, nil)
	ca := everyaddon_ca
	ca.Source = "gitplex"
	assert.ErrorContains(t, InstallCatalogueAddon(app, ca), "unsupported source 'gitplex'")
	assert.Empty(t, test_dir_contents(t, path))
	assert.Empty(t, ft.Requested())
}

func Test_InstallCatalogueAddon__no_addons_dir(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	assert.ErrorContains(t, InstallCatalogueAddon(app, everyaddon_ca), "no addons directory")
}

// clj: `catalogue_test.clj/expand-summary--retail-strict--just-classic`
func Test_InstallCatalogueAddon__no_release_for_game_track(t *testing.T) {
	routes := everyaddon_routes(t, "1.2.3")
	routes[github_release_list_url("a/b")] = http_utils.Fixture{Body: []byte(github_releases_json("1.2.3", "classic"))}
	app, path, _ := app_with_installed(t, routes)
	assert.ErrorContains(t, InstallCatalogueAddon(app, everyaddon_ca), "no 'Retail' release found on github.")
	assert.Empty(t, test_dir_contents(t, path))
}

// clj: `core_test.clj/install-addon-guard--remove-zip`
func Test_InstallCatalogueAddon__keep_no_zips(t *testing.T) {
	app, path, _ := app_with_installed(t, everyaddon_routes(t, "1.2.3"))
	apply_settings(app, func(s Settings) Settings {
		s.Preferences.AddonZipsToKeep = new(0)
		return s
	})
	assert.NoError(t, InstallCatalogueAddon(app, everyaddon_ca))
	assert.Equal(t, []string{"EveryAddon"}, test_dir_contents(t, path))
}

// an addon already installed is updated, not installed again.
func Test_InstallCatalogueAddon__already_installed(t *testing.T) {
	routes := everyaddon_routes(t, "1.2.4")
	app, path, _ := app_with_installed(t, routes, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3", GroupID: "https://github.com/a/b"})
	CheckForUpdates(app)
	assert.NoError(t, InstallCatalogueAddon(app, everyaddon_ca))
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "1.2.4", top.InstalledVersion)
	assert.Len(t, addon_results(app), 1)
}

// clj: `cli_test.clj/install-update-these-in-parallel--bad-download`
func Test_install__download_not_a_zip(t *testing.T) {
	ca := CatalogueAddon{URL: "https://www.wowinterface.com/downloads/info1", Name: "everyaddon", Label: "EveryAddon",
		Source: SOURCE_WOWI, SourceID: "1", GameTrackIDList: []GameTrackID{GAMETRACK_RETAIL}}
	app, path, _ := app_with_installed(t, map[string]http_utils.Fixture{
		wowinterface_release_url("1"):  fixture("v7/everyaddon--wowinterface--detail.json"),
		wowinterface_download_url("1"): fixture("v7/wowinterface--download-not-approved.html"),
	})
	assert.ErrorContains(t, InstallCatalogueAddon(app, ca), "not a valid zip")
	assert.Empty(t, test_dir_contents(t, path), "nothing installed, the download deleted")
}

// clj: `cli_test.clj/import-addon--github`
func Test_InstallAddonFromURL(t *testing.T) {
	app, path, ft := app_with_installed(t, everyaddon_routes(t, "1.2.3"))
	assert.NoError(t, InstallAddonFromURL(app, "https://github.com/a/b/releases"))
	assert.Empty(t, ft.Unrouted())
	assert.DirExists(t, filepath.Join(path, "EveryAddon"))

	user := read_user_catalogue(get_paths(app)["strongbox.paths.user-catalogue-file"])
	assert.Len(t, user.AddonSummaryList, 1)
	assert.Equal(t, FlexString("a/b"), user.AddonSummaryList[0].SourceID)
}

// clj: `cli_test.clj/import-addon--curseforge`, `--tukui`
func Test_InstallAddonFromURL__refused(t *testing.T) {
	app, path, ft := app_with_installed(t, nil)
	for _, given := range []string{"https://www.curseforge.com/wow/addons/x", "https://www.tukui.org/addons.php?id=1", "https://example.org/x"} {
		assert.Error(t, InstallAddonFromURL(app, given), given)
	}
	assert.Empty(t, test_dir_contents(t, path))
	assert.Empty(t, ft.Requested())
	assert.NoFileExists(t, get_paths(app)["strongbox.paths.user-catalogue-file"])
}

// clj: `cli_test.clj/install-addon-from-file-in-parallel`, `unique-group-id-from-zip-file`
func Test_InstallAddonFromZip(t *testing.T) {
	app, path, _ := app_with_installed(t, nil)
	zipfile := zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}})
	assert.NoError(t, InstallAddonFromZip(app, zipfile))
	assert.FileExists(t, zipfile, "the user's zip is never deleted")

	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Regexp(t, `^everyaddon-[0-9a-f]{8}$`, top.GroupID)
	assert.True(t, top.Primary)
	assert.Equal(t, "", top.Source, "grouping only")
	assert.Len(t, addon_results(app), 1)
}

// clj: `cli_test.clj/install-addon--ignore-then-update-from-file`
func Test_InstallAddonFromZip__over_ignored(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "0.1"})
	os.MkdirAll(filepath.Join(path, "EveryAddon", ".git"), 0o755)
	assert.NoError(t, InstallAddonFromZip(app, zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}})))
	installed, _ := LoadAllInstalledAddons(MakeAddonsDir(path))
	assert.True(t, installed[0].IsIgnored)
}

// clj: `cli_test.clj/install-addon-from-file--then-update`
func Test_InstallAddonFromZip__then_updated(t *testing.T) {
	app, path, _ := app_with_installed(t, everyaddon_routes(t, "1.2.4"))
	catalogue_to_state(app, Catalogue{AddonSummaryList: []CatalogueAddon{everyaddon_ca}}, Catalogue{})
	assert.NoError(t, InstallAddonFromZip(app, zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}, Title: "EveryAddon", Version: "1.2.3"})))
	CheckForUpdates(app)
	r := addon_results(app)[0]
	assert.True(t, Updateable(r.Item.(Addon)))

	assert.NoError(t, UpdateAddons(app, []string{r.ID}))
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "1.2.4", top.InstalledVersion)
	assert.Equal(t, SOURCE_GITHUB, top.Source, "a full nfo once installed from a host")
}

// --- updating

func Test_UpdateAddons(t *testing.T) {
	app, path, _ := app_with_installed(t, everyaddon_routes(t, "1.2.4"),
		test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3", GroupID: "https://github.com/a/b"},
		test_addon_spec{DirList: []string{"Other"}, Source: SOURCE_GITHUB, SourceID: "c/d"})
	CheckForUpdates(app) // c/d fails, so Other is not updateable
	id_list := []string{}
	for _, r := range addon_results(app) {
		id_list = append(id_list, r.ID)
	}
	assert.NoError(t, UpdateAddons(app, id_list))

	ad := MakeAddonsDir(path)
	top, _ := read_nfo(t, ad, "EveryAddon").Top()
	assert.Equal(t, "1.2.4", top.InstalledVersion)
	for _, r := range addon_results(app) {
		a := r.Item.(Addon)
		assert.False(t, Updateable(a), a.Label)
		assert.False(t, r.Tags.Contains(core.TAG_HAS_UPDATE), a.Label)
		assert.False(t, r.Tags.Contains(core.TAG_BUSY), a.Label)
	}
	other, _ := read_nfo(t, ad, "Other").Top()
	assert.Equal(t, "1.2.3", other.InstalledVersion, "not updateable, untouched")
}

func Test_UpdateAll(t *testing.T) {
	app, path, _ := app_with_installed(t, everyaddon_routes(t, "1.2.4"),
		test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3"})
	CheckForUpdates(app)
	assert.NoError(t, UpdateAll(app))
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "1.2.4", top.InstalledVersion)

	update_all_running(app).Store(true)
	defer update_all_running(app).Store(false)
	assert.ErrorIs(t, UpdateAll(app), ErrUpdatesInProgress)
}

// clj: `addon_test.clj/test-re-installable?`, `test-find-release`
func Test_ReinstallAddons(t *testing.T) {
	routes := everyaddon_routes(t, "1.2.3")
	app, path, _ := app_with_installed(t, routes,
		test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.3"})
	CheckForUpdates(app)
	os.Remove(filepath.Join(path, "EveryAddon", "EveryAddon.lua"))

	assert.NoError(t, ReinstallAddons(app, []string{addon_results(app)[0].ID}))
	assert.FileExists(t, filepath.Join(path, "EveryAddon", "EveryAddon.lua"), "restored")
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "1.2.3", top.InstalledVersion)
}

func Test_reinstall_release(t *testing.T) {
	ad := retail_strict
	a := updateable_addon(ad, "1.2.3", nil, su("1.2.4", GAMETRACK_RETAIL), su("1.2.3", GAMETRACK_RETAIL))
	release, same, ok := reinstall_release(a)
	assert.True(t, ok)
	assert.True(t, same)
	assert.Equal(t, "1.2.3", release.Version)

	a = updateable_addon(ad, "1.0.0", nil, su("1.2.4", GAMETRACK_RETAIL))
	release, same, _ = reinstall_release(a)
	assert.False(t, same)
	assert.Equal(t, "1.2.4", release.Version)

	assert.False(t, re_installable(updateable_addon(ad, "1.0", func(n *NFO) { n.Ignored = new(true) })))
	assert.False(t, re_installable(updateable_addon(ad, "1.0", func(n *NFO) { n.Source = ""; n.SourceID = "" })))
	assert.True(t, re_installable(updateable_addon(ad, "1.0", nil)))
}

func Test_InstallAddonRelease(t *testing.T) {
	routes := everyaddon_routes(t, "1.2.4")
	old_zip, _ := os.ReadFile(zip_of(t, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "1.2.3"}))
	routes[github_release_list_url("a/b")] = http_utils.Fixture{Body: []byte(`[` +
		strings.TrimSuffix(strings.TrimPrefix(github_releases_json("1.2.4", "retail"), "["), "]") + `,` +
		strings.TrimSuffix(strings.TrimPrefix(github_releases_json("1.2.3", "retail"), "["), "]") + `]`)}
	routes["https://github.com/a/b/releases/download/1.2.3/EveryAddon-1.2.3-retail.zip"] = http_utils.Fixture{Body: old_zip}
	app, path, _ := app_with_installed(t, routes,
		test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "1.2.4"})
	CheckForUpdates(app)
	id := addon_results(app)[0].ID

	assert.NoError(t, InstallAddonRelease(app, id, "1.2.3"))
	top, _ := read_nfo(t, MakeAddonsDir(path), "EveryAddon").Top()
	assert.Equal(t, "1.2.3", top.InstalledVersion)
	assert.True(t, Updateable(addon_results(app)[0].Item.(Addon)), "the newer release is offered again")

	assert.Error(t, InstallAddonRelease(app, id, "9.9.9"))
}

// --- removing

// returns the addon result labelled `label`.
func addon_result_by_label(t *testing.T, app *core.App, label string) *core.Result {
	t.Helper()
	for _, r := range addon_results(app) {
		if r.Item.(Addon).Label == label {
			return &r
		}
	}
	t.Fatalf("no addon %q", label)
	return nil
}

func Test_RemoveAddon__grouped(t *testing.T) {
	app, path, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_WOWI, SourceID: "1"})
	assert.NoError(t, RemoveAddon(app, addon_result_by_label(t, app, "EveryAddon")))
	assert.Empty(t, test_dir_contents(t, path))
	assert.Empty(t, addon_results(app))
}

// clj: `core_test.clj/uninstall-addon`
func Test_RemoveAddon__unmanaged(t *testing.T) {
	app, path, _ := app_with_installed(t, nil,
		test_addon_spec{DirList: []string{"EveryAddon"}},
		test_addon_spec{DirList: []string{"EveryAddon-BundledAddon"}})
	assert.NoError(t, RemoveAddon(app, addon_result_by_label(t, app, "EveryAddon")))
	assert.Equal(t, []string{"EveryAddon-BundledAddon"}, test_dir_contents(t, path))
}

// clj: `core_test.clj/uninstall-addons-with-mutual-dependencies--overwrote`, `--overwritten`
func Test_RemoveAddon__shared_directory(t *testing.T) {
	setup := func(t *testing.T) (*core.App, AddonsDir) {
		app, path, _ := app_with_installed(t, nil)
		ad := MakeAddonsDir(path)
		assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Version: "0.1.2"}, InstallOpts{}))
		assert.NoError(t, install_spec(t, ad, test_addon_spec{DirList: []string{"EveryOtherAddon", "EveryAddon-BundledAddon"}, Version: "5.6.7"}, InstallOpts{}))
		assert.NoError(t, ReloadAddonsDir(app, derive_addons_dir(ad, FindSettings(app), core.DirExists)))
		return app, ad
	}

	t.Run("newest owner removed", func(t *testing.T) {
		app, ad := setup(t)
		assert.NoError(t, RemoveAddon(app, addon_result_by_label(t, app, "https://example.org/EveryOtherAddon (group)")))
		assert.Equal(t, []string{"EveryAddon", "EveryAddon-BundledAddon"}, test_dir_contents(t, ad.Path))
		f := read_nfo(t, ad, "EveryAddon-BundledAddon")
		assert.Len(t, f.Stack, 1)
		assert.Equal(t, "https://example.org/EveryAddon", f.Stack[0].GroupID)
	})

	t.Run("older owner removed", func(t *testing.T) {
		app, ad := setup(t)
		assert.NoError(t, RemoveAddon(app, addon_result_by_label(t, app, "EveryAddon")))
		assert.Equal(t, []string{"EveryAddon-BundledAddon", "EveryOtherAddon"}, test_dir_contents(t, ad.Path))
		f := read_nfo(t, ad, "EveryAddon-BundledAddon")
		assert.Len(t, f.Stack, 1)
		assert.Equal(t, "https://example.org/EveryOtherAddon", f.Stack[0].GroupID)
	})
}

// clj: `core_test.clj/uninstall-ignored-addon`, `uninstall-ignored-bundled-addon`
func Test_RemoveAddons__ignored(t *testing.T) {
	app, path, _ := app_with_installed(t, nil,
		test_addon_spec{DirList: []string{"Ignored", "Ignored-Bundled"}, Source: SOURCE_WOWI, SourceID: "1"},
		test_addon_spec{DirList: []string{"Removed"}})
	os.MkdirAll(filepath.Join(path, "Ignored-Bundled", ".git"), 0o755)
	assert.NoError(t, ReloadAddonsDir(app, derive_addons_dir(MakeAddonsDir(path), FindSettings(app), core.DirExists)))

	ignored := addon_result_by_label(t, app, "Ignored")
	removed := addon_result_by_label(t, app, "Removed")
	err := RemoveAddons(app, []*core.Result{ignored, removed})
	assert.ErrorIs(t, err, ErrIgnored)
	assert.Equal(t, []string{"Ignored", "Ignored-Bundled"}, test_dir_contents(t, path))
}

// clj: `addon_test.clj/remove-addon--malign-addon-data`
func Test_check_removable(t *testing.T) {
	root := t.TempDir()
	ad := MakeAddonsDir(filepath.Join(root, "AddOns"))
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon"), 0o755)
	os.MkdirAll(filepath.Join(root, "Elsewhere"), 0o755)
	os.Symlink(filepath.Join(root, "Elsewhere"), filepath.Join(ad.Path, "Link"))
	os.WriteFile(filepath.Join(ad.Path, "file.txt"), []byte("x"), 0o644)

	_, err := check_removable(ad, "EveryAddon")
	assert.NoError(t, err)
	for _, given := range []string{"./", "../", "../../", "/root", "~/Desktop", "", ".", "Link", "file.txt", "EveryAddon/../../Elsewhere"} {
		_, err := check_removable(ad, given)
		assert.Error(t, err, given)
	}
	assert.DirExists(t, filepath.Join(root, "Elsewhere"))
}
