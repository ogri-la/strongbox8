package strongbox

import (
	"bw/core"
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
