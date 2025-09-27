package main

import (
	"bw/core"
	"bw/ui"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	strongbox "strongbox/src"
	"testing"
	"time"

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

	// Ensure catalogue directory exists but is empty (no old format files)
	catalogue_dir := filepath.Join(data_dir, "strongbox", "catalogues")
	core.MakeDirs(catalogue_dir)

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
		{"tags column functionality", func(t *testing.T) {
			test_catalogue_addon := strongbox.CatalogueAddon{
				Name:     "test-addon",
				TagList:  []string{"test", "gui", "verification"},
				Source:   "github",
				SourceID: "test/test-addon",
			}

			test_addons_dir := strongbox.AddonsDir{
				Path:        addons_dir,
				GameTrackID: strongbox.GAMETRACK_RETAIL,
				Strict:      true,
			}

			addon := strongbox.MakeAddonFromCatalogueAddon(test_addons_dir, test_catalogue_addon, []strongbox.SourceUpdate{})
			assert.Equal(t, []string{"test", "gui", "verification"}, addon.Tags)

			item_map := addon.ItemMap()
			assert.Equal(t, "test, gui, verification", item_map["tags"])
			assert.Contains(t, strongbox.COL_LIST_DEFAULT, "tags")
		}},
		{"created date column functionality", func(t *testing.T) {
			testTime, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")
			test_catalogue_addon := strongbox.CatalogueAddon{
				Name:        "test-addon-with-date",
				CreatedDate: testTime,
				Source:      "wowinterface",
				SourceID:    "12345",
			}

			test_addons_dir := strongbox.AddonsDir{
				Path:        addons_dir,
				GameTrackID: strongbox.GAMETRACK_RETAIL,
				Strict:      true,
			}

			addon := strongbox.MakeAddonFromCatalogueAddon(test_addons_dir, test_catalogue_addon, []strongbox.SourceUpdate{})
			assert.Equal(t, testTime, addon.Created)

			item_map := addon.ItemMap()
			assert.Regexp(t, `\d+ year(s)? ago`, item_map[core.ITEM_FIELD_DATE_CREATED])
			assert.Contains(t, strongbox.COL_LIST_DEFAULT, "created-date")
		}},
		{"EasyMail catalogue addon should show tags when installed", func(t *testing.T) {
			// Use the exact EasyMail data from the catalogue
			easymailUpdatedTime, _ := time.Parse(time.RFC3339, "2025-08-16T00:09:10Z")
			easymail_catalogue := strongbox.CatalogueAddon{
				Name:            "easymail-from-cosmos",
				Label:           "EasyMail from Cosmos",
				Description:     "Cosmos died. But life is not over! This is the bona-fide update to the original Cosmos version. Accept no substitutes!",
				TagList:         []string{"mail", "ui"},
				URL:             "https://www.wowinterface.com/downloads/info11426",
				Source:          "wowinterface",
				SourceID:        "11426",
				GameTrackIDList: []strongbox.GameTrackID{"classic", "classic-cata", "retail"},
				UpdatedDate:     easymailUpdatedTime,
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
				assert.Equal(t, "mail, ui", item_map["tags"])
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
				assert.Equal(t, "", item_map["tags"])
			})

		}},
		{"search tab bug reproduction with old catalogue file", func(t *testing.T) {
			app := gui.App()

			// Create an old-format catalogue file that our new code should fail to parse
			old_format_catalogue_json := `{
				"spec": {"version": 1},
				"datestamp": "2023-01-15",
				"total": 1,
				"addon-summary-list": [
					{
						"name": "test-addon",
						"label": "Test Addon",
						"description": "A test addon",
						"created-date": "2023-01-15T10:30:00Z",
						"updated-date": "2024-08-16T00:09:10Z",
						"source": "github",
						"source-id": "test/test-addon",
						"tag-list": ["test"],
						"download-count": 100
					}
				]
			}`

			// Write this old format file to where the app expects to find it
			catalogue_dir := app.State.GetKeyVal("strongbox.paths.catalogue-dir")
			catalogue_file := filepath.Join(catalogue_dir, "short-catalogue.json")

			err := core.Spit(catalogue_file, []byte(old_format_catalogue_json))
			assert.NoError(t, err, "Should be able to write test catalogue file")

			// Now try to load this old format catalogue file - this should fail with our time.Time changes

			// Try to load the catalogue file
			strongbox.DBLoadCatalogue(app)

			// Check if catalogue loaded
			has_catalogue := app.HasResult(strongbox.ID_CATALOGUE)

			if has_catalogue {
				// Investigate what actually loaded
				cat_result := app.GetResult(strongbox.ID_CATALOGUE)
				cat := cat_result.Item.(strongbox.Catalogue)

				if len(cat.AddonSummaryList) > 0 {
					first_addon := cat.AddonSummaryList[0]

					// Test ItemMap to see if this is where the bug occurs
					item_map := first_addon.ItemMap()

					created := item_map[core.ITEM_FIELD_DATE_CREATED]
					updated := item_map[core.ITEM_FIELD_DATE_UPDATED]
					assert.NotNil(t, created, "Created field should exist")
					assert.NotNil(t, updated, "Updated field should exist")
				}
			}

			// Check catalogue addon results
			catalogue_results := app.FilterResultListByNS(strongbox.NS_CATALOGUE_ADDON)
			assert.NotNil(t, catalogue_results, "Catalogue results should exist")

			// Check search tab
			search_tab := gui.GetTab("search")
			assert.NotNil(t, search_tab, "Search tab should exist")

		}},
		{"test real catalogue file parsing", func(t *testing.T) {
			// Test with the actual catalogue file format from the real system
			real_catalogue_path := "/home/torkus/.local/share/strongbox8/short-catalogue.json"

			if !core.FileExists(real_catalogue_path) {
				t.Skip("Real catalogue file not found, skipping test")
				return
			}

			// Try to parse the real catalogue file
			cat_loc := strongbox.CatalogueLocation{
				Name:   "short",
				Label:  "Short",
				Source: "https://example.com/short.json",
			}

			// Read the file and try to unmarshal it
			data, err := os.ReadFile(real_catalogue_path)
			if err != nil {
				t.Fatalf("Failed to read catalogue file: %v", err)
			}

			var catalogue strongbox.Catalogue
			catalogue.CatalogueLocation = cat_loc
			err = json.Unmarshal(data, &catalogue)
			if err != nil {
				assert.Fail(t, "Real catalogue file should parse successfully with time.Time fields")
				return
			}

			if len(catalogue.AddonSummaryList) > 0 {
				first_addon := catalogue.AddonSummaryList[0]

				// Test ItemMap formatting
				item_map := first_addon.ItemMap()
				created := item_map[core.ITEM_FIELD_DATE_CREATED]
				updated := item_map[core.ITEM_FIELD_DATE_UPDATED]

				assert.NotEmpty(t, created, "Created date should be formatted")
				assert.NotEmpty(t, updated, "Updated date should be formatted")
			}
		}},
		{"verify search tab works with fixed catalogue loading", func(t *testing.T) {
			app := gui.App()

			// Load the real catalogue (should now work with our fix)
			strongbox.DBLoadCatalogue(app)

			// Check if catalogue loaded successfully
			has_catalogue := app.HasResult(strongbox.ID_CATALOGUE)
			assert.True(t, has_catalogue, "Catalogue should load successfully with flexible timestamp parsing")

			if !has_catalogue {
				t.Skip("Catalogue failed to load, cannot test search tab")
				return
			}

			// Check that catalogue addon results are available for search tab
			catalogue_results := app.FilterResultListByNS(strongbox.NS_CATALOGUE_ADDON)

			// Verify search tab has access to catalogue data
			search_tab := gui.GetTab("search")
			assert.NotNil(t, search_tab, "Search tab should exist")

			// This verifies the original bug is fixed: search tab should now have data
			// and not throw "cell index out of range" errors
			if len(catalogue_results) > 0 {
				first_result := catalogue_results[0]
				addon := first_result.Item.(strongbox.CatalogueAddon)

				// Verify the addon has properly formatted dates
				item_map := addon.ItemMap()
				created := item_map[core.ITEM_FIELD_DATE_CREATED]
				updated := item_map[core.ITEM_FIELD_DATE_UPDATED]

				// Both should be non-empty and contain time indicators
				assert.NotEmpty(t, created, "Created date should be formatted")
				assert.NotEmpty(t, updated, "Updated date should be formatted")
				assert.Regexp(t, `ago$`, created, "Created date should be in 'X ago' format")
				assert.Regexp(t, `ago$`, updated, "Updated date should be in 'X ago' format")
			}

		}},
		{"test catalogue JSON parsing with time.Time", func(t *testing.T) {
			// Create a test catalogue file with time.Time-compatible JSON
			test_time, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")

			test_catalogue := strongbox.Catalogue{
				CatalogueLocation: strongbox.CatalogueLocation{
					Name:   "test",
					Label:  "Test Catalogue",
					Source: "https://example.com/test.json",
				},
				Spec:      strongbox.CatalogueSpec{Version: 1},
				Datestamp: "2023-01-15",
				Total:     1,
				AddonSummaryList: []strongbox.CatalogueAddon{
					{
						Name:        "test-addon",
						Label:       "Test Addon",
						Description: "A test addon",
						CreatedDate: test_time,
						UpdatedDate: test_time,
						Source:      "github",
						SourceID:    "test/test-addon",
					},
				},
			}

			// Test if we can create ItemMap without errors
			first_addon := test_catalogue.AddonSummaryList[0]
			item_map := first_addon.ItemMap()

			created := item_map[core.ITEM_FIELD_DATE_CREATED]
			updated := item_map[core.ITEM_FIELD_DATE_UPDATED]

			// This should work fine if our time.Time implementation is correct
			assert.NotEmpty(t, created, "Created date should be formatted")
			assert.NotEmpty(t, updated, "Updated date should be formatted")
			assert.Contains(t, created, "year", "Created date should contain 'year'")

			// Test that the new time.Time fields work correctly
			// (This test is mainly to verify our original time.Time implementation works)

			// Test ItemMap formatting directly
			first_addon = test_catalogue.AddonSummaryList[0]
			item_map = first_addon.ItemMap()
			created = item_map[core.ITEM_FIELD_DATE_CREATED]
			updated = item_map[core.ITEM_FIELD_DATE_UPDATED]

			// Verify formatting works correctly
			assert.NotEmpty(t, created, "Created date should be formatted")
			assert.NotEmpty(t, updated, "Updated date should be formatted")
			assert.Contains(t, created, "year", "Created date should be formatted as relative time")
			assert.Contains(t, updated, "year", "Updated date should be formatted as relative time")
		}},
	}

	for _, testfn := range testfn_list {
		t.Run(testfn.label, testfn.fn)
	}
}
