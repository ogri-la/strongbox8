package strongbox

import (
	"bw/core"
	"bw/http_utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// a zip can be installed into an empty addons dir,
// creating an addon
func Test_install_addon__minimal(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	zipfile := test_fixture_everyaddon_minimal_zip

	a, err := MakeAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	err = install_addon(ad, a, zipfile)
	assert.Nil(t, err)

	// assertions about addons dir state

	dir_list, err := core.DirList(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(dir_list))

	file_list, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 3, len(file_list))

	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon/"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/.strongbox.json"))

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(toc_map))
	assert.Equal(t, "1.2.3", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into an empty addons dir,
// creating an addon
func Test_install_addon__maximal(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	zipfile := test_fixture_everyaddon_maximal_zip

	a, err := MakeAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	err = install_addon(ad, a, zipfile)
	assert.Nil(t, err)

	// assertions about addons dir state

	dir_list, err := core.DirList(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(dir_list))

	file_list, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon/"))

	assert.Equal(t, 7, len(file_list))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Vanilla.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_TBC.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Wrath.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Cata.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/.strongbox.json"))

	file_list2, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon_Config/"))
	assert.Nil(t, err)
	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon_Config/"))

	assert.Equal(t, 3, len(file_list2))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/EveryAddon_Config.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/EveryAddon_Config.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/.strongbox.json"))

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))

	assert.Nil(t, err)
	assert.Equal(t, 5, len(toc_map))
	assert.Equal(t, "7.8.9", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into a populated, unmanaged, addons dir,
// updating an addon.
func Test_install_addon__update(t *testing.T) {
	ad := MakeAddonsDir(t.TempDir())

	zipfile := test_fixture_everyaddon_minimal_zip

	// 'install' EveryAddon v1.2.3
	// *no* strongbox nfo
	_, err := unzip_file(zipfile, ad.Path)
	assert.Nil(t, err)

	// install EveryAddon 1.2.4 patch update
	zipfile_update := test_fixture_everyaddon_minimal_update_zip
	a, err := MakeAddonFromZipfile(ad, zipfile_update)
	assert.Nil(t, err)

	err = install_addon(ad, a, zipfile_update)
	assert.Nil(t, err)

	// assertions about addons dir state

	dir_list, err := core.DirList(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(dir_list))

	file_list, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 3, len(file_list))

	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon/"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/.strongbox.json"))

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	// future: read addon data (if it exists) before installing update to ensure nfo data contains 'installedversion'
	// future: support a 'filesystem' source and get rid of the minimal-nfo of just a 'group-id' and 'primary?'

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(toc_map))
	assert.Equal(t, "1.2.4", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into a populated, unmanaged, addons dir,
// completely replacing an existing addon.
func Test_install_addon__completely_replace(t *testing.T) {
	// this covers the 'completely replace' installation behaviour that would otherwise create a mutual dependency

}

// a zip can be installed into a populated, unmanaged, addons dir,
// partial replacing an existing addon and creating a mutual dependency.
func Test_install_addon__mutual_dependency(t *testing.T) {
	// this covers the 'completely replace' installation behaviour
}

// ---

// an Addon derived from a CatalogueAddon + zipfile can be installed
func Test_install_addon__with_catalogue_addon(t *testing.T) {
	ad := MakeAddonsDir(t.TempDir())

	zipfile := test_fixture_everyaddon_minimal_zip

	ca := test_fixture_catalogue.AddonSummaryList[0]
	sul := []SourceUpdate{}

	a := MakeAddonFromCatalogueAddon(ad, ca, sul)

	err := install_addon(ad, a, zipfile)
	assert.Nil(t, err)

	addon_list, err := LoadAllInstalledAddons(ad)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(addon_list))

	expected_nfo := &NFO{
		GroupID: "https://github.com/ogri-la/everyaddon",
		Primary: true,
	}
	actual_nfo := addon_list[0].NFO
	assert.Equal(t, expected_nfo, actual_nfo)

	// ... more!
}

// ---

// all of the additional checks to installing an addon can be tested
func Test_install_addon_guard(t *testing.T) {
	app := DummyApp()
	ad := MakeAddonsDir(t.TempDir())

	zipfile := test_fixture_everyaddon_minimal_zip
	a, err := MakeAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	opts := InstallOpts{}
	err = install_addon_guard(app, ad, a, zipfile, opts)
	assert.Nil(t, err)
}

// installing an addon from the catalogue is possible
func Test_install_addon_guard__with_catalogue_addon(t *testing.T) {
	tmpdir := t.TempDir()
	app, stopfn := DummyApp2(tmpdir)
	defer stopfn()

	ad := MakeAddonsDir(filepath.Join(tmpdir, "addons"))
	_, wg := app.AddItem(NS_ADDONS_DIR, ad)
	wg.Wait()

	ca := test_fixture_catalogue.AddonSummaryList[0]
	sul := []SourceUpdate{}
	a := MakeAddonFromCatalogueAddon(ad, ca, sul)

	zipfile := test_fixture_everyaddon_minimal_zip

	opts := InstallOpts{}
	err := install_addon_guard(app, ad, a, zipfile, opts)
	assert.Nil(t, err)

	addon_list, err := LoadAllInstalledAddons(ad)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(addon_list))

	expected_nfo := &NFO{
		GroupID: "https://github.com/ogri-la/everyaddon",
		Primary: true,
	}
	actual_nfo := addon_list[0].NFO
	assert.Equal(t, expected_nfo, actual_nfo)
}

// installing an addon from the catalogue over the top of an existing addon is the same as an addon updating itself
func Test_install_addon_guard__with_catalogue_addon_overwriting_existing(t *testing.T) {

}

// ---

func TestRemoveAddon(t *testing.T) {
	/*
		app := DummyApp()
		go app.ProcessUpdateLoop()
		defer core.Stop(app)

		strongbox := Provider(app)
		app.RegisterProvider(strongbox)
		app.StartProviders()
	*/

	tmpdir := t.TempDir()
	app, stopfn := DummyApp2(tmpdir)
	defer stopfn()

	ad := MakeAddonsDir(filepath.Join(tmpdir, "addons"))
	_, wg := app.AddItem(NS_ADDONS_DIR, ad)
	wg.Wait()

	assert.Nil(t, InstallAddonHelper(app, ad))

	r := app.FirstResult(func(r core.Result) bool {
		return r.NS == NS_ADDON
	})
	assert.NotNil(t, r)

	err := RemoveAddon(app, r)
	assert.Nil(t, err)

	// result no longer present in state
	r2 := app.FirstResult(func(r core.Result) bool {
		return r.NS == NS_ADDON
	})
	assert.Nil(t, r2)

	// addon contents no longer present on fs
	path_list, err := core.ReadDir(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, path_list)

}

func TestCheckAddon(t *testing.T) {
	tmpdir := t.TempDir()
	app, stopfn := DummyApp2(tmpdir)
	defer stopfn()

	// mock GitHub API response with a valid release
	github_response := `[{
		"name": "1.0.0",
		"tag_name": "v1.0.0",
		"published_at": "2024-01-01T00:00:00Z",
		"draft": false,
		"prerelease": false,
		"assets": [{
			"name": "Addon1-1.0.0.zip",
			"state": "uploaded",
			"content_type": "application/zip",
			"browser_download_url": "https://example.com/Addon1-1.0.0.zip"
		}]
	}]`
	app.Downloader = core.MakeDummyDownloader(&http_utils.ResponseWrapper{
		Bytes: []byte(github_response),
	})

	ad := MakeAddonsDir(filepath.Join(tmpdir, "addons"))

	// create two addons with sources - both could have updates available
	addon1 := Addon{
		AddonsDir: &ad,
		Label:     "Addon1",
		Source:    SOURCE_GITHUB,
		SourceID:  "test/addon1",
		InstalledAddonGroup: []InstalledAddon{
			{Name: "Addon1"},
		},
		Primary: InstalledAddon{Name: "Addon1"},
	}

	addon2 := Addon{
		AddonsDir: &ad,
		Label:     "Addon2",
		Source:    SOURCE_GITHUB,
		SourceID:  "test/addon2",
		InstalledAddonGroup: []InstalledAddon{
			{Name: "Addon2"},
		},
		Primary: InstalledAddon{Name: "Addon2"},
	}

	// add both to state
	r1, wg1 := app.AddItem(NS_ADDON, addon1)
	wg1.Wait()
	r2, wg2 := app.AddItem(NS_ADDON, addon2)
	wg2.Wait()

	// verify initial state - no source updates
	assert.Equal(t, 0, len(r1.Item.(Addon).SourceUpdateList))
	assert.Equal(t, 0, len(r2.Item.(Addon).SourceUpdateList))

	// check only addon1 for updates
	CheckAddon(app, r1)

	// verify addon1 now has updates available
	updated_r1 := app.FindResultByID(r1.ID)
	assert.NotNil(t, updated_r1)
	updated_addon1 := updated_r1.Item.(Addon)
	assert.Equal(t, 1, len(updated_addon1.SourceUpdateList), "addon1 should have 1 source update after CheckAddon")
	assert.Equal(t, "1.0.0", updated_addon1.SourceUpdateList[0].Version)
	assert.NotNil(t, updated_addon1.SourceUpdate, "addon1 should have a selected SourceUpdate")

	// verify addon2 is unchanged - still no updates
	updated_r2 := app.FindResultByID(r2.ID)
	assert.NotNil(t, updated_r2)
	updated_addon2 := updated_r2.Item.(Addon)
	assert.Equal(t, 0, len(updated_addon2.SourceUpdateList), "addon2 should still have 0 source updates")
}
