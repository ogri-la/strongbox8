package strongbox

import (
	"bw/core"
	"bw/http_utils"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the service `id` offered by the strongbox provider.
func find_provider_service(t *testing.T, id string) core.Service {
	t.Helper()
	for _, group := range provider() {
		for _, service := range group.ServiceList {
			if service.ID == id {
				return service
			}
		}
	}
	t.Fatalf("no such service: %s", id)
	return core.Service{}
}

// every service the menus refer to exists.
func TestMenuServicesExist(t *testing.T) {
	for _, menu := range (&StrongboxProvider{}).Menu() {
		for _, item := range menu.MenuItemList {
			if item.ServiceID != "" {
				find_provider_service(t, item.ServiceID)
			}
		}
	}
}

func TestParseZipsToKeep(t *testing.T) {
	three := 3
	zero := 0
	cases := []struct {
		given    string
		expected *int
	}{
		{"", nil},
		{"  ", nil},
		{"3", &three},
		{" 0 ", &zero},
	}
	for _, c := range cases {
		actual, err := parse_zips_to_keep(nil, c.given)
		assert.NoError(t, err)
		assert.Equal(t, c.expected, actual)
	}
	for _, given := range []string{"-1", "two", "1.5"} {
		_, err := parse_zips_to_keep(nil, given)
		assert.Error(t, err, given)
	}
}

// changed preferences are saved straight away.
func TestPreferencesService(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	two := 2
	given := core.ServiceFnArgs{ArgList: []core.KeyVal{
		{Key: "addon-zips-to-keep", Val: &two},
		{Key: "keep-user-catalogue-updated", Val: false},
		{Key: "check-for-update", Val: false},
	}}
	result := find_provider_service(t, SERVICE_ID_PREFERENCES).Fn(app, given)
	assert.NoError(t, result.Err)

	actual := saved_settings(t, app).Preferences
	assert.Equal(t, &two, actual.AddonZipsToKeep)
	assert.False(t, actual.KeepUserCatalogueUpdated)
	assert.False(t, actual.CheckForUpdate)
	assert.Equal(t, &two, FindSettings(app).Preferences.AddonZipsToKeep)
}

// with no available addons dir, the menu items that install are disabled.
func TestMenuServices__no_addons_dir(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	for _, id := range []string{SERVICE_ID_INSTALL_FROM_FILE, SERVICE_ID_IMPORT_ADDON, SERVICE_ID_UPDATE_ALL} {
		assert.False(t, find_provider_service(t, id).IsApplicable(app, nil), id)
	}
	assert.True(t, find_provider_service(t, SERVICE_ID_NEW_ADDONS_DIR).IsApplicable(app, nil))
}

// an installed addon with no catalogue entry cannot be starred, and says why.
func TestStarService__unmatched(t *testing.T) {
	app, _, _ := app_with_installed(t, nil, test_addon_spec{DirList: []string{"EveryAddon"}})
	r, _ := only_addon(t, app)
	service := find_provider_service(t, SERVICE_ID_STAR_ADDON)
	assert.False(t, service.IsApplicable(app, []core.Result{*r}))

	result := service.Fn(app, core.MakeServiceFnArgs("selected", r))
	assert.ErrorContains(t, result.Err, "has no catalogue entry")
	assert.NoFileExists(t, get_paths(app)["strongbox.paths.user-catalogue-file"])
}

// the menu services that need no input do what they say without failing.
func TestMenuServices__refresh_user_catalogue(t *testing.T) {
	app, _ := test_app_with_routes(t, map[string]http_utils.Fixture{
		CAT_FULL.Source: {Body: []byte(catalogue_json(catalogue_entry("github", "a/b", "EveryAddon Renamed")))},
	})
	assert.NoError(t, StarCatalogueAddon(app, everyaddon_ca))
	actual := capture_log(func() {
		result := find_provider_service(t, SERVICE_ID_REFRESH_USER_CAT).Fn(app, core.NewServiceFnArgs())
		assert.NoError(t, result.Err)
	})
	assert.NotContains(t, actual, "not implemented")
	user := read_user_catalogue(get_paths(app)["strongbox.paths.user-catalogue-file"])
	assert.Equal(t, "EveryAddon Renamed", user.AddonSummaryList[0].Label)
}
