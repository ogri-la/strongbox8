package strongbox

import (
	"bw/core"
	"bw/http_utils"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	app.AddReplaceResults(MakeAddonsDirResult(ad)).Wait()

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

	// mock GitHub API response with a valid release.
	// the asset names its game track: an asset with no game track produces no update.
	github_response := `[{
		"name": "1.0.0",
		"tag_name": "v1.0.0",
		"published_at": "2024-01-01T00:00:00Z",
		"draft": false,
		"prerelease": false,
		"assets": [{
			"name": "Addon1-1.0.0-retail.zip",
			"state": "uploaded",
			"content_type": "application/zip",
			"browser_download_url": "https://example.com/Addon1-1.0.0-retail.zip"
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
	CheckAddons(app, []*core.Result{r1})

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
