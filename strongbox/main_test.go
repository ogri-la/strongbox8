package main

import (
	"bw/core"
	"bw/ui"
	"log/slog"
	"os"
	"path/filepath"
	strongbox "strongbox/src"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	exitCode := m.Run()
	os.Exit(exitCode)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// single test to prevent flashing
func Test_main_gui(t *testing.T) {

	// todo: the envvars above are not preventing the catalogue from loading

	tmpdir := t.TempDir()

	data_dir := filepath.Join(tmpdir, "xdg-data")     // "/tmp/rand/xdg-data"
	config_dir := filepath.Join(tmpdir, "xdg-config") // "/tmp/rand/xdg-config"

	os.Setenv("XDG_DATA_HOME", data_dir)
	os.Setenv("XDG_CONFIG_HOME", config_dir)

	addons_dir := filepath.Join(tmpdir, "addons") // "/tmp/rand/addons"
	core.MakeDirs(addons_dir)

	gui := main_gui()
	defer gui.Stop()
	defer gui.App().Stop() // urgh, this is all over the place. don't bundle app with gui?

	testfn_list := []struct {
		label string
		fn    func(t *testing.T)
	}{
		{"default tab count is correct", func(t *testing.T) {
			assert.True(t, len(gui.TabList) > 0)
		}},
		{"a form can be opened, filled, submitted", func(t *testing.T) {
			tab := gui.TabList[0] // addons dirs

			// find a service function to render
			service, err := gui.App().FindService("new-addons-dir")
			assert.Nil(t, err)

			assert.Nil(t, tab.GUIForm) // no form is set
			initial_data := []core.KeyVal{}
			tab.OpenForm(service, initial_data).Wait() // open details and set a form
			assert.NotNil(t, tab.GUIForm)              // form is now set

			// todo: test the gui rendered the form

			// bind form
			form_data := []core.KeyVal{
				{Key: "Foo", Val: "Bar"},
			}

			// hrm: not great. we have tab.OpenForm and tab.CloseForm, why not tab.FillForm(data) ? and tab.SubmitForm() ?
			// when we SubmitForm the form will need to be re-rendered with any errors, etc.
			gui.TkSync(func() {
				tab.GUIForm.Form.Update(form_data) // bind form data
				tab.GUIForm.Fill()                 // flush changes in form to gui
			}).Wait()

			// todo: test the gui rendered the data

			// we can submit the form directly and access the args to pass to the Service fn this way

			service_fn_args, form_err := tab.GUIForm.Submit()
			assert.NotNil(t, form_err.Error) // data was bad, we expect a validation error
			assert.Empty(t, service_fn_args) // because of bad data, the result should be empty

			// --- ok, try again with actual data

			// re-bind form with actual data
			form_data = []core.KeyVal{
				{Key: "addons-dir", Val: addons_dir},
			}

			gui.TkSync(func() {
				tab.GUIForm.Form.Update(form_data) // bind form data
				tab.GUIForm.Fill()                 // flush changes in form to gui
			}).Wait()

			// check the 'addons-dir' field in the gui form matches our temp dir
			addons_dir_field := tab.GUIForm.Fields[0]
			var field_value string
			gui.TkSync(func() {
				field_value = addons_dir_field.Input.Get()
			}).Wait()
			assert.Equal(t, addons_dir, field_value)

			service_fn_args, form_err = tab.GUIForm.Submit()
			assert.Nil(t, form_err) // data was good, form errors should be nil

			expected := core.KeyVal{Key: "addons-dir", Val: addons_dir}
			assert.Equal(t, expected, service_fn_args.ArgList[0])

			// clicking the submit button will actually call the bound service with the valid args

			submit_btn := tab.GUIForm.Fields[1].Input.(*ui.TKButton)

			gui.TkSync(func() {
				submit_btn.Invoke() // ... so, this is how the form should be invoked if we're testing gui behaviour.
			}).Wait()

			// a successful invocation this way (clicking submit button) closes the open form and discards the reference to the form

			// assert.Nil(t, tab.GUIForm) // hrm: this should be nil, we're passing a pointer around
			assert.Nil(t, gui.GetCurrentTab().GUIForm)

			// we now have one addons dir in the application state whose path is equal to the temp addons dir

			rl := gui.App().FilterResultListByNS(strongbox.NS_ADDONS_DIR)
			assert.Equal(t, 1, len(rl))

			r := rl[0]
			ad := r.Item.(strongbox.AddonsDir)
			assert.Equal(t, addons_dir, ad.Path)
		}},
		{"tags column functionality with proper data flow", func(t *testing.T) {
			// Create test addon with tags using proper data flow from catalogue
			test_catalogue_addon := strongbox.CatalogueAddon{
				Name:        "test-addon",
				Label:       "Test Addon",
				Description: "A test addon for verifying tags",
				TagList:     []string{"test", "gui", "verification"},
				URL:         "https://example.com/test-addon",
				Source:      "github",
				SourceID:    "test/test-addon",
			}

			// Create addon through proper data flow
			test_addons_dir := strongbox.AddonsDir{
				Path:        addons_dir,
				GameTrackID: strongbox.GAMETRACK_RETAIL,
				Strict:      true,
			}

			addon := strongbox.MakeAddonFromCatalogueAddon(test_addons_dir, test_catalogue_addon, []strongbox.SourceUpdate{})

			// Add addon to app state
			app := gui.App()
			addon_result := core.MakeResult(strongbox.NS_ADDON, addon, "test-addon-id")
			app.AddReplaceResults(addon_result).Wait()

			// Verify the data flow is correct at the model level
			addon_results := app.FilterResultListByNS(strongbox.NS_ADDON)
			assert.Equal(t, 1, len(addon_results), "Should have exactly one addon result")

			result := addon_results[0]
			found_addon := result.Item.(strongbox.Addon)

			// Test the complete data flow: CatalogueAddon.TagList -> Addon.Tags -> ItemInfo
			assert.Equal(t, "test-addon", found_addon.Name)
			assert.Equal(t, []string{"test", "gui", "verification"}, found_addon.Tags, "Tags should flow from CatalogueAddon.TagList to Addon.Tags")

			item_map := found_addon.ItemMap()
			expected_tags := "test, gui, verification"
			assert.Equal(t, expected_tags, item_map["tags"], "ItemMap should format tags correctly")

			// Verify tags column is in the default column list (this was our fix)
			assert.Contains(t, strongbox.COL_LIST_DEFAULT, "tags", "Tags should be in default column list")

			t.Logf("✓ Data flow test passed:")
			t.Logf("  CatalogueAddon.TagList: %v", test_catalogue_addon.TagList)
			t.Logf("  Addon.Tags: %v", found_addon.Tags)
			t.Logf("  ItemMap['tags']: %s", item_map["tags"])
			t.Logf("  Tags in default columns: %v", contains(strongbox.COL_LIST_DEFAULT, "tags"))

			// NOTE: Testing actual TK widget state would require access to unexported fields
			// The GUI state should correctly reflect the model state based on our architecture
		}},
		{"EasyMail catalogue addon should show tags when installed", func(t *testing.T) {
			// Use the exact EasyMail data from the catalogue
			easymail_catalogue := strongbox.CatalogueAddon{
				Name:            "easymail-from-cosmos",
				Label:           "EasyMail from Cosmos",
				Description:     "Cosmos died. But life is not over! This is the bona-fide update to the original Cosmos version. Accept no substitutes!",
				TagList:         []string{"mail", "ui"},
				URL:             "https://www.wowinterface.com/downloads/info11426",
				Source:          "wowinterface",
				SourceID:        "11426",
				GameTrackIDList: []strongbox.GameTrackID{"classic", "classic-cata", "retail"},
				UpdatedDate:     "2025-08-16T00:09:10Z",
				DownloadCount:   76373,
			}

			// Simulate this addon being installed
			test_addons_dir := strongbox.AddonsDir{
				Path:        addons_dir,
				GameTrackID: strongbox.GAMETRACK_RETAIL,
				Strict:      true,
			}

			// Test both scenarios: with and without catalogue match
			t.Run("with catalogue match", func(t *testing.T) {
				// Create addon through MakeAddonFromCatalogueAddon (simulates catalogue match)
				addon_with_match := strongbox.MakeAddonFromCatalogueAddon(test_addons_dir, easymail_catalogue, []strongbox.SourceUpdate{})

				// This should have tags
				assert.Equal(t, []string{"mail", "ui"}, addon_with_match.Tags, "Addon created from catalogue should have tags")

				item_map := addon_with_match.ItemMap()
				assert.Equal(t, "mail, ui", item_map["tags"], "ItemMap should format tags correctly")

				t.Logf("✓ With catalogue match - Tags: %v, Formatted: %s", addon_with_match.Tags, item_map["tags"])
			})

			t.Run("without catalogue match", func(t *testing.T) {
				// Create addon through MakeAddon without catalogue match (simulates installed addon without catalogue entry)
				nfo := strongbox.NFO{
					Name:    "easymail-from-cosmos",
					GroupID: "test-group",
				}

				toc := strongbox.NewTOC()
				toc.Name = "EasyMail"
				toc.Title = "EasyMail from Cosmos"
				toc.Notes = "Cosmos died. But life is not over!"

				installed_addon := strongbox.NewInstalledAddon()
				installed_addon.TOCMap = map[strongbox.PathToFile]strongbox.TOC{"EasyMail.toc": toc}
				installed_addon.NFOList = []strongbox.NFO{nfo}

				// MakeAddon without catalogue match
				addon_without_match := strongbox.MakeAddon(test_addons_dir, []strongbox.InstalledAddon{installed_addon}, installed_addon, &nfo, nil, []strongbox.SourceUpdate{})

				// This should NOT have tags (no catalogue match)
				assert.Nil(t, addon_without_match.Tags, "Addon without catalogue match should not have tags")

				item_map := addon_without_match.ItemMap()
				assert.Equal(t, "", item_map["tags"], "ItemMap should have empty tags")

				t.Logf("✓ Without catalogue match - Tags: %v, Formatted: '%s'", addon_without_match.Tags, item_map["tags"])
			})

			// The key question: In the real GUI, which scenario is happening?
			// If EasyMail shows no tags, it suggests the addon isn't matching with the catalogue
			t.Logf("💡 If EasyMail shows no tags in GUI, the addon likely isn't matching with catalogue data")
			t.Logf("   Check if installed addon name matches catalogue name exactly: 'easymail-from-cosmos'")
		}},
	}

	for _, testfn := range testfn_list {
		t.Run(testfn.label, testfn.fn)
	}
}
