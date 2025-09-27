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
		{"tags column functionality", func(t *testing.T) {
			test_catalogue_addon := strongbox.CatalogueAddon{
				Name:    "test-addon",
				TagList: []string{"test", "gui", "verification"},
				Source:  "github",
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
			test_catalogue_addon := strongbox.CatalogueAddon{
				Name:        "test-addon-with-date",
				CreatedDate: "2023-01-15T10:30:00Z",
				Source:      "wowinterface",
				SourceID:    "12345",
			}

			test_addons_dir := strongbox.AddonsDir{
				Path:        addons_dir,
				GameTrackID: strongbox.GAMETRACK_RETAIL,
				Strict:      true,
			}

			addon := strongbox.MakeAddonFromCatalogueAddon(test_addons_dir, test_catalogue_addon, []strongbox.SourceUpdate{})
			assert.Equal(t, "2023-01-15T10:30:00Z", addon.Created)

			item_map := addon.ItemMap()
			assert.Equal(t, "2023-01-15T10:30:00Z", item_map[core.ITEM_FIELD_DATE_CREATED])
			assert.Contains(t, strongbox.COL_LIST_DEFAULT, "created-date")
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
	}

	for _, testfn := range testfn_list {
		t.Run(testfn.label, testfn.fn)
	}
}
