package main

import (
	"archive/zip"
	"bw/core"
	"bw/http_utils"
	"bw/ui"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	strongbox "strongbox/src"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtures for driving strongbox through its GUI the way a user does, with every HTTP
// request answered by a `FixtureTransport` and every confirmation by the test.

const (
	fx_catalogue_url  = "https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/short-catalogue.json"
	fx_everyaddon_gh  = "example/everyaddon"
	fx_everyaddon_url = "https://github.com/example/everyaddon"
	fx_everyaddon_wi  = "12345"
	fx_someaddon_gh   = "example/someaddon"
	fx_someaddon_url  = "https://github.com/example/someaddon"
)

// the GitHub releases API URL of the repository `source_id`.
func fx_github_releases_url(source_id string) string {
	return "https://api.github.com/repos/" + source_id + "/releases?per_page=100&page=1"
}

// the download URL of the retail asset of release `version` of the repository `source_id`.
func fx_github_asset_url(source_id, label, version string) string {
	return "https://github.com/" + source_id + "/releases/download/" + version + "/" + label + "-" + version + "-retail.zip"
}

// returns a GitHub releases response listing `version_list`, newest first, each with a
// retail asset.
func fx_github_releases(source_id, label string, version_list ...string) []byte {
	release_list := []map[string]any{}
	for _, version := range version_list {
		release_list = append(release_list, map[string]any{
			"name":         version,
			"tag_name":     version,
			"html_url":     "https://github.com/" + source_id + "/releases/tag/" + version,
			"published_at": "2026-01-01T00:00:00Z",
			"assets": []map[string]any{{
				"name":                 label + "-" + version + "-retail.zip",
				"state":                "uploaded",
				"content_type":         "application/zip",
				"browser_download_url": fx_github_asset_url(source_id, label, version),
			}},
		})
	}
	b, _ := json.Marshal(release_list)
	return b
}

// returns the bytes of an addon zip with a single directory `label` whose `.toc` declares
// `version`, a retail interface version and any extra `toc_lines`.
func fx_addon_zip(t *testing.T, label, version string, toc_lines ...string) []byte {
	t.Helper()
	toc := strings.Join(append([]string{
		"## Interface: 110200",
		"## Title: " + label,
		"## Version: " + version,
	}, toc_lines...), "\n") + "\n\n" + label + ".lua\n"

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		label + "/" + label + ".toc": toc,
		label + "/" + label + ".lua": "-- " + label + " " + version + "\n",
	} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// the selected catalogue: EveryAddon on both GitHub and WoWInterface, plus an addon the
// searches should not find.
func fx_catalogue() []byte {
	entry := func(name, label, source, source_id, url, description string) map[string]any {
		return map[string]any{
			"name": name, "label": label, "source": source, "source-id": source_id, "url": url,
			"description": description, "game-track-list": []string{"retail"},
			"updated-date": "2026-01-01T00:00:00Z", "tag-list": []string{}, "download-count": 1,
		}
	}
	b, _ := json.Marshal(map[string]any{
		"spec":      map[string]any{"version": 2},
		"datestamp": "2026-01-01",
		"total":     3,
		"addon-summary-list": []map[string]any{
			entry("everyaddon", "EveryAddon", "github", fx_everyaddon_gh, fx_everyaddon_url, "Does what no other addon does"),
			entry("everyaddon", "EveryAddon", "wowinterface", fx_everyaddon_wi, "https://www.wowinterface.com/downloads/info"+fx_everyaddon_wi, "Does what no other addon does"),
			entry("otheraddon", "OtherAddon", "github", "example/otheraddon", "https://github.com/example/otheraddon", "Something else entirely"),
		},
	})
	return b
}

// a WoWInterface file details response for EveryAddon at `version`.
func fx_wowinterface_details(version string) []byte {
	b, _ := json.Marshal([]map[string]any{{
		"UID": fx_everyaddon_wi, "UIVersion": version, "UIDate": 1767225600000,
		"UIFileName": "EveryAddon-" + version + ".zip", "UIName": "EveryAddon",
	}})
	return b
}

// strongbox 7's settings file, with one addons dir.
func fx_v7_config(v7_addons_dir string) []byte {
	b, _ := json.Marshal(map[string]any{
		"addon-dir-list": []map[string]any{
			{"addon-dir": v7_addons_dir, "game-track": "retail", "strict?": true},
		},
		"selected-addon-dir": v7_addons_dir,
		"selected-catalogue": "short",
		"preferences": map[string]any{
			"addon-zips-to-keep":  2,
			"ui-selected-columns": []string{"source", "name", "description", "combined-version", "game-version"},
		},
	})
	return b
}

// the routes answering every request the workflow makes, as they are before any new
// release is published.
func fx_routes(t *testing.T) map[string]http_utils.Fixture {
	return map[string]http_utils.Fixture{
		fx_catalogue_url:                 {Body: fx_catalogue()},
		strongbox.STRONGBOX_RELEASES_URL: {Body: []byte(`[{"tag_name": "0.0.1"}]`)},

		fx_github_releases_url(fx_everyaddon_gh):                      {Body: fx_github_releases(fx_everyaddon_gh, "EveryAddon", "1.2.3")},
		fx_github_asset_url(fx_everyaddon_gh, "EveryAddon", "1.2.3"):  {Body: fx_addon_zip(t, "EveryAddon", "1.2.3", "## X-WoWI-ID: "+fx_everyaddon_wi)},
		fx_github_releases_url(fx_someaddon_gh):                       {Body: fx_github_releases(fx_someaddon_gh, "SomeAddon", "2.0.0")},
		fx_github_asset_url(fx_someaddon_gh, "SomeAddon", "2.0.0"):    {Body: fx_addon_zip(t, "SomeAddon", "2.0.0")},
		"https://api.mmoui.com/v3/game/WOW/filedetails/12345.json":    {Body: fx_wowinterface_details("1.3.0")},
		"https://cdn.wowinterface.com/downloads/getfile.php?id=12345": {Body: fx_addon_zip(t, "EveryAddon", "1.3.0", "## X-WoWI-ID: "+fx_everyaddon_wi)},
	}
}

// answers the GUI's confirmations and records them, and records reported errors.
type fx_person struct {
	mu        sync.Mutex
	answer    bool
	asked     []string
	error_log []string
}

func (p *fx_person) confirm(_, message string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.asked = append(p.asked, message)
	return p.answer
}

func (p *fx_person) report_error(title, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.error_log = append(p.error_log, title+": "+message)
}

// sets the answer to the next confirmations and forgets those asked so far.
func (p *fx_person) will_answer(answer bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answer = answer
	p.asked = nil
}

func (p *fx_person) questions() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.asked)
}

func (p *fx_person) errors() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.error_log)
}

// ---

// the results with `id_list`, as a user selecting their rows has.
func selected_results(t *testing.T, gui *ui.GUIUI, id_list ...string) []core.Result {
	t.Helper()
	selected := []core.Result{}
	for _, id := range id_list {
		r := gui.App().GetResult(id)
		require.NotNil(t, r, "no result: %s", id)
		selected = append(selected, *r)
	}
	return selected
}

// returns the service `service_id` from the context menu of `selected`, and whether it is
// enabled there.
func context_menu_service(t *testing.T, gui *ui.GUIUI, selected []core.Result, service_id string) (core.Service, bool) {
	t.Helper()
	for _, group := range core.SelectionGroups(gui.App().TypeMap, gui.App(), selected) {
		for _, ss := range group.ServiceList {
			if ss.Service.ID == service_id {
				return ss.Service, ss.Enabled
			}
		}
	}
	t.Fatalf("service not in the context menu: %s", service_id)
	return core.Service{}, false
}

// returns `true` when `service_id` is enabled in the context menu of the results `id_list`.
func enabled_in_context_menu(t *testing.T, gui *ui.GUIUI, service_id string, id_list ...string) bool {
	t.Helper()
	_, enabled := context_menu_service(t, gui, selected_results(t, gui, id_list...), service_id)
	return enabled
}

// chooses `service_id` from the context menu of the results `id_list` and waits for it,
// and anything it started, to finish.
func choose_from_context_menu(t *testing.T, gui *ui.GUIUI, service_id string, id_list ...string) {
	t.Helper()
	selected := selected_results(t, gui, id_list...)
	service, enabled := context_menu_service(t, gui, selected, service_id)
	require.True(t, enabled, "service disabled in the context menu: %s", service_id)
	require.False(t, service.NeedsInput(), "service needs a form: %s", service_id)
	gui.TkSync(func() { gui.InvokeSelectionService(service, selected) })
	gui.WaitForIdle()
}

// opens the form for `service` in the current tab with `initial`, sets the fields in
// `values` as a user typing or choosing does, submits it and waits for the service, and
// anything it started, to finish.
func submit_form(t *testing.T, gui *ui.GUIUI, service core.Service, initial []core.KeyVal, values map[string]string) {
	t.Helper()
	tab := gui.GetCurrentTab()
	tab.OpenForm(service, initial)
	require.NotNil(t, tab.GUIForm)
	gui.TkSync(func() {
		for _, field := range tab.GUIForm.Fields {
			if val, present := values[field.ID()]; present {
				field.Input.Set(val)
			}
		}
	})
	submit_btn := tab.GUIForm.Fields[len(tab.GUIForm.Fields)-1].Input.(*ui.TKButton)
	gui.TkSync(func() { submit_btn.Invoke() })
	gui.WaitForIdle()
}

// chooses `service_id` from the menu bar, as a user does, and waits for it to finish.
func choose_from_menu(t *testing.T, gui *ui.GUIUI, service_id string) {
	t.Helper()
	service, err := gui.App().FindService(service_id)
	require.NoError(t, err)
	require.False(t, service.NeedsInput())
	gui.TkSync(func() { gui.ConfirmAndRunService(service, core.NewServiceFnArgs(), nil) })
	gui.WaitForIdle()
}

// the installed addon result with the directory `dir_name`, nil when there is none.
func installed_addon(gui *ui.GUIUI, dir_name string) *core.Result {
	for _, r := range gui.App().FilterResultListByNS(strongbox.NS_ADDON) {
		for _, ia := range r.Item.(strongbox.Addon).InstalledAddonGroup {
			if ia.DirName == dir_name {
				return &r
			}
		}
	}
	return nil
}

// `true` when the result `id` has a row in the tab `tab_title`.
func has_row(gui *ui.GUIUI, tab_title, id string) bool {
	var present bool
	gui.TkSync(func() { _, present = gui.GetTab(tab_title).ItemFkeyIndex[id] })
	return present
}

// reads `option` of the row of `id` in the tab `tab_title`.
func row_option(gui *ui.GUIUI, tab_title, id, option string) string {
	var out string
	gui.TkSync(func() { out = gui.GetTab(tab_title).RowOption(id, option) })
	return out
}

// reads the `column` cell of the row of `id` in the tab `tab_title`.
func row_cell(gui *ui.GUIUI, tab_title, id, column string) string {
	var out string
	gui.TkSync(func() { out = gui.GetTab(tab_title).RowCell(id, column) })
	return out
}

// `true` when the row of `id` in the installed tab is marked as having an update.
func marked_for_update(gui *ui.GUIUI, id string) bool {
	return row_option(gui, strongbox.TAB_LABEL_INSTALLED, id, "-background") != ""
}

// the `.toc` version of the addon directory `dir`.
func toc_version(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.Base(dir)+".toc"))
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "## Version: "); ok {
			return v
		}
	}
	return ""
}

// ---

type workflow_env struct {
	gui           *ui.GUIUI
	ft            *http_utils.FixtureTransport
	person        *fx_person
	addons_dir    string // added through the GUI
	v7_addons_dir string // imported from strongbox 7's settings
	v7_cfg_file   string
	v7_cfg        []byte
	zip_file      string // an addon zip on disk, for installing from a file
}

// the subtests driving strongbox through a user's workflow. each depends on the ones
// before it and they run in order.
func workflow_subtests(env *workflow_env) []struct {
	label string
	fn    func(t *testing.T)
} {
	gui := env.gui
	app := gui.App()
	installed := strongbox.TAB_LABEL_INSTALLED
	everyaddon_id := func() string {
		return "addon:" + env.addons_dir + "#group:" + fx_everyaddon_url
	}
	everyaddon := func() strongbox.Addon {
		r := app.GetResult(everyaddon_id())
		if r == nil {
			return strongbox.Addon{}
		}
		return r.Item.(strongbox.Addon)
	}
	everyaddon_dir := filepath.Join(env.addons_dir, "EveryAddon")

	return []struct {
		label string
		fn    func(t *testing.T)
	}{
		{"first run imports strongbox 7's settings and leaves them unchanged", func(t *testing.T) {
			gui.WaitForIdle()
			settings := strongbox.FindSettings(app)
			require.Len(t, settings.AddonsDirList, 1)
			assert.Equal(t, env.v7_addons_dir, settings.AddonsDirList[0].Path)
			assert.Equal(t, env.v7_addons_dir, settings.Preferences.SelectedAddonsDir)
			assert.Equal(t, "short", settings.Preferences.SelectedCatalogue)

			actual, err := os.ReadFile(env.v7_cfg_file)
			require.NoError(t, err)
			assert.Equal(t, env.v7_cfg, actual)

			// the imported column preference picks the installed tab's columns
			col_list := []string{}
			for _, col := range installed_columns(settings.Preferences.SelectedColumns) {
				if !col.Hidden {
					col_list = append(col_list, col.Title)
				}
			}
			assert.Equal(t, []string{"source", "name", "description", "combined-version", "game-version"}, col_list)
		}},
		{"the catalogue is downloaded and shown in the search tab", func(t *testing.T) {
			for _, id := range []string{"catalogue-addon:github/" + fx_everyaddon_gh, "catalogue-addon:wowinterface/" + fx_everyaddon_wi} {
				present := has_row(gui, "search", id)
				assert.True(t, present, id)
			}
		}},
		{"adding an addons dir selects it", func(t *testing.T) {
			service, err := app.FindService(strongbox.SERVICE_ID_NEW_ADDONS_DIR)
			require.NoError(t, err)
			gui.SetActiveTab(installed)
			submit_form(t, gui, service, nil, map[string]string{"addons-dir": env.addons_dir})

			settings := strongbox.FindSettings(app)
			assert.Len(t, settings.AddonsDirList, 2)
			assert.Equal(t, env.addons_dir, settings.Preferences.SelectedAddonsDir)
			present := has_row(gui, installed, env.addons_dir)
			assert.True(t, present, "addons dir row")
		}},
		{"searching shows only matching catalogue addons", func(t *testing.T) {
			gui.SetActiveTab("search")
			gui.TkSync(func() { gui.GetTab("search").ApplySearchFilter("everyaddon") })
			assert.Equal(t, "0", row_option(gui, "search", "catalogue-addon:github/"+fx_everyaddon_gh, "-hide"))
			assert.Equal(t, "0", row_option(gui, "search", "catalogue-addon:wowinterface/"+fx_everyaddon_wi, "-hide"))
			assert.Equal(t, "1", row_option(gui, "search", "catalogue-addon:github/example/otheraddon", "-hide"))
		}},
		{"installing from the search tab installs into the selected addons dir", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_INSTALL_CAT_ADDON, "catalogue-addon:github/"+fx_everyaddon_gh)
			gui.TkSync(func() { gui.GetTab("search").ApplySearchFilter("") })

			assert.Equal(t, "1.2.3", toc_version(t, everyaddon_dir))
			assert.NoDirExists(t, filepath.Join(env.v7_addons_dir, "EveryAddon"))
			a := everyaddon()
			assert.Equal(t, "1.2.3", a.InstalledVersion)
			assert.NotNil(t, a.CatalogueAddon, "matched against the catalogue")
			assert.Equal(t, "1.2.3", row_cell(gui, installed, everyaddon_id(), "combined-version"))
			assert.False(t, marked_for_update(gui, everyaddon_id()))
		}},
		{"a new release is found on refresh and the row is marked", func(t *testing.T) {
			env.ft.Set(fx_github_releases_url(fx_everyaddon_gh), http_utils.Fixture{Body: fx_github_releases(fx_everyaddon_gh, "EveryAddon", "1.2.4", "1.2.3")})
			env.ft.Set(fx_github_asset_url(fx_everyaddon_gh, "EveryAddon", "1.2.4"), http_utils.Fixture{Body: fx_addon_zip(t, "EveryAddon", "1.2.4", "## X-WoWI-ID: "+fx_everyaddon_wi)})
			choose_from_menu(t, gui, strongbox.SERVICE_ID_REFRESH)

			assert.Equal(t, "1.2.4", everyaddon().AvailableVersion)
			assert.True(t, marked_for_update(gui, everyaddon_id()))
			assert.Equal(t, "1.2.4", row_cell(gui, installed, everyaddon_id(), "combined-version"))
			assert.Equal(t, "1.2.3", toc_version(t, everyaddon_dir), "nothing is installed until the user asks")
		}},
		{"updating installs the new release and clears the mark", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_UPDATE_ADDON, everyaddon_id())
			assert.Equal(t, "1.2.4", toc_version(t, everyaddon_dir))
			assert.Equal(t, "1.2.4", everyaddon().InstalledVersion)
			assert.False(t, marked_for_update(gui, everyaddon_id()))
		}},
		{"a pinned addon is not updated", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_PIN_ADDON, everyaddon_id())
			assert.Equal(t, "(pinned) 1.2.4", row_cell(gui, installed, everyaddon_id(), "combined-version"))

			env.ft.Set(fx_github_releases_url(fx_everyaddon_gh), http_utils.Fixture{Body: fx_github_releases(fx_everyaddon_gh, "EveryAddon", "1.2.5", "1.2.4", "1.2.3")})
			choose_from_menu(t, gui, strongbox.SERVICE_ID_REFRESH)
			assert.Equal(t, "1.2.4", everyaddon().AvailableVersion, "the pinned release is chosen")
			assert.Len(t, everyaddon().SourceUpdateList, 3, "the new release was found")
			assert.False(t, marked_for_update(gui, everyaddon_id()))
			assert.False(t, enabled_in_context_menu(t, gui, strongbox.SERVICE_ID_UPDATE_ADDON, everyaddon_id()))
		}},
		{"unpinning allows the update again", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_UNPIN_ADDON, everyaddon_id())
			assert.True(t, marked_for_update(gui, everyaddon_id()))
			assert.Equal(t, "1.2.5", row_cell(gui, installed, everyaddon_id(), "combined-version"))
		}},
		{"an ignored addon cannot be updated or uninstalled", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_IGNORE_ADDON, everyaddon_id())
			assert.True(t, everyaddon().IsIgnored)
			assert.False(t, marked_for_update(gui, everyaddon_id()))
			assert.Equal(t, "(ignored) 1.2.4", row_cell(gui, installed, everyaddon_id(), "combined-version"))
			assert.False(t, enabled_in_context_menu(t, gui, strongbox.SERVICE_ID_UPDATE_ADDON, everyaddon_id()))
			assert.False(t, enabled_in_context_menu(t, gui, strongbox.SERVICE_ID_UNINSTALL_ADDON, everyaddon_id()))

			choose_from_menu(t, gui, strongbox.SERVICE_ID_UPDATE_ALL)
			assert.Equal(t, "1.2.4", toc_version(t, everyaddon_dir))
		}},
		{"an addon is managed again once it is no longer ignored", func(t *testing.T) {
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_STOP_IGNORING_ADDON, everyaddon_id())
			assert.False(t, everyaddon().IsIgnored)
			assert.True(t, enabled_in_context_menu(t, gui, strongbox.SERVICE_ID_UNINSTALL_ADDON, everyaddon_id()))
			assert.True(t, marked_for_update(gui, everyaddon_id()))
		}},
		{"an addon is installed from a zip file", func(t *testing.T) {
			service, err := app.FindService(strongbox.SERVICE_ID_INSTALL_FROM_FILE)
			require.NoError(t, err)
			gui.SetActiveTab(installed)
			submit_form(t, gui, service, []core.KeyVal{{Key: "zipfiles", Val: env.zip_file}}, nil)

			assert.Equal(t, "0.1.0", toc_version(t, filepath.Join(env.addons_dir, "LocalAddon")))
			r := installed_addon(gui, "LocalAddon")
			require.NotNil(t, r)
			present := has_row(gui, installed, r.ID)
			assert.True(t, present, "LocalAddon row")
		}},
		{"an addon is imported by URL and added to the user catalogue", func(t *testing.T) {
			service, err := app.FindService(strongbox.SERVICE_ID_IMPORT_ADDON)
			require.NoError(t, err)
			submit_form(t, gui, service, nil, map[string]string{"url": "  " + fx_someaddon_url + "  "})

			assert.Equal(t, "2.0.0", toc_version(t, filepath.Join(env.addons_dir, "SomeAddon")))
			r := installed_addon(gui, "SomeAddon")
			require.NotNil(t, r)
			b, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "strongbox8", "user-catalogue.json"))
			require.NoError(t, err)
			assert.Contains(t, string(b), fx_someaddon_gh)
		}},
		{"switching source checks the new host for updates", func(t *testing.T) {
			selected := selected_results(t, gui, everyaddon_id())
			service, enabled := context_menu_service(t, gui, selected, strongbox.SERVICE_ID_SWITCH_SOURCE)
			require.True(t, enabled)
			gui.SetActiveTab(installed)
			submit_form(t, gui, service, core.SelectionArgs(service, selected).ArgList, map[string]string{"source": "wowinterface " + fx_everyaddon_wi})

			a := everyaddon()
			assert.Equal(t, "wowinterface", a.Source)
			assert.Equal(t, fx_everyaddon_wi, a.SourceID)
			assert.Equal(t, "1.3.0", a.AvailableVersion)
			assert.Equal(t, "wowinterface", row_cell(gui, installed, everyaddon_id(), "source"))
			assert.Equal(t, "1.2.4", toc_version(t, everyaddon_dir), "switching source installs nothing")
		}},
		{"declining to uninstall removes nothing", func(t *testing.T) {
			env.person.will_answer(false)
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_UNINSTALL_ADDON, everyaddon_id())
			require.Len(t, env.person.questions(), 1)
			assert.Contains(t, env.person.questions()[0], "EveryAddon")
			assert.DirExists(t, everyaddon_dir)
			assert.NotNil(t, app.GetResult(everyaddon_id()))
		}},
		{"confirming uninstall removes the addon's directories and row", func(t *testing.T) {
			env.person.will_answer(true)
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_UNINSTALL_ADDON, everyaddon_id())
			assert.Len(t, env.person.questions(), 1)
			assert.NoDirExists(t, everyaddon_dir)
			assert.Nil(t, app.GetResult(everyaddon_id()))
			present := has_row(gui, installed, everyaddon_id())
			assert.False(t, present)
			assert.DirExists(t, filepath.Join(env.addons_dir, "LocalAddon"), "other addons are untouched")
		}},
		{"removing an addons dir leaves its files alone", func(t *testing.T) {
			env.person.will_answer(true)
			choose_from_context_menu(t, gui, strongbox.SERVICE_ID_REMOVE_ADDONS_DIR, env.addons_dir)
			assert.Len(t, env.person.questions(), 1)
			assert.DirExists(t, filepath.Join(env.addons_dir, "LocalAddon"))
			assert.DirExists(t, filepath.Join(env.addons_dir, "SomeAddon"))

			settings := strongbox.FindSettings(app)
			require.Len(t, settings.AddonsDirList, 1)
			assert.Equal(t, env.v7_addons_dir, settings.AddonsDirList[0].Path)
			assert.Nil(t, app.GetResult(env.addons_dir))
		}},
		{"no request went unanswered and no service failed", func(t *testing.T) {
			assert.Empty(t, env.ft.Unrouted())
			assert.Empty(t, env.person.errors())
		}},
	}
}

// writes the fixtures to disk under `tmpdir` and returns the environment the workflow
// runs in, without a GUI.
func workflow_fixtures(t *testing.T, tmpdir string) *workflow_env {
	env := &workflow_env{
		ft:            http_utils.NewFixtureTransport(fx_routes(t)),
		person:        &fx_person{answer: true},
		addons_dir:    filepath.Join(tmpdir, "addons"),
		v7_addons_dir: filepath.Join(tmpdir, "v7-addons"),
		v7_cfg_file:   filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "strongbox", "config.json"),
		zip_file:      filepath.Join(tmpdir, "downloads", "LocalAddon-0.1.0.zip"),
	}
	env.v7_cfg = fx_v7_config(env.v7_addons_dir)
	for _, dir := range []string{env.addons_dir, env.v7_addons_dir, filepath.Dir(env.v7_cfg_file), filepath.Dir(env.zip_file)} {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	require.NoError(t, os.WriteFile(env.v7_cfg_file, env.v7_cfg, 0644))
	require.NoError(t, os.WriteFile(env.zip_file, fx_addon_zip(t, "LocalAddon", "0.1.0"), 0644))
	return env
}
