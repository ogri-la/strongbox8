package strongbox

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestCombinedVersion(t *testing.T) {
	cases := []struct {
		given    Addon
		expected string
	}{
		{Addon{InstalledVersion: "1.2.3"}, "1.2.3"},
		{Addon{InstalledVersion: "1.2.3", AvailableVersion: "1.2.4", SourceUpdateList: []SourceUpdate{{Version: "1.2.4"}}, SourceUpdate: &SourceUpdate{Version: "1.2.4"}}, "1.2.4"},
		{Addon{InstalledVersion: "1.2.3", IsPinned: true, PinnedVersion: "1.2.3"}, "(pinned) 1.2.3"},
		{Addon{InstalledVersion: "1.2.3", IsIgnored: true}, "(ignored) 1.2.3"},
		{Addon{IsIgnored: true}, "(ignored)"},
		{Addon{InstalledVersion: "1.2.3", AvailableVersion: "1.2.4", IsIgnored: true}, "(ignored) 1.2.3"},
	}
	for _, c := range cases {
		actual := combined_version(c.given)
		assert.Equal(t, c.expected, actual)
	}
}

// every column the installed tab offers is a key in an addon's item map.
func TestInstalledColumnKeys(t *testing.T) {
	item_map := Addon{}.ItemMap()
	for _, key := range InstalledColumnKeys() {
		_, present := item_map[key]
		assert.True(t, present, key)
	}
}

func TestSelectedColumnKeys(t *testing.T) {
	given := []string{"name", "tag-list", "dirsize", "uber-button", "browse-local", "bogus", "combined-version"}
	expected := mapset.NewSet("name", "tags", "size", "combined-version")
	actual := SelectedColumnKeys(given)
	assert.True(t, expected.Equal(actual), actual.String())
}

// the default columns are all shown, bar strongbox 7's 'uber-button'.
func TestSelectedColumnKeys__default(t *testing.T) {
	actual := SelectedColumnKeys(COL_LIST_DEFAULT)
	assert.Equal(t, len(COL_LIST_DEFAULT)-1, actual.Cardinality())
}
