package strongbox

import (
	"bw/core"
	"testing"

	"github.com/stretchr/testify/assert"
)

// an unloaded catalogue has no length to report and is distinct from an empty one.
func Test_db_catalogue_empty__not_loaded(t *testing.T) {
	app := DummyApp()

	assert.False(t, db_catalogue_loaded(app))

	_, err := db_catalogue_empty(app)
	assert.ErrorContains(t, err, "catalogue not loaded")
}

// returns an app with `item` in state under the catalogue ID, plus a cleanup fn.
// a real app has settings and paths by the time a catalogue is loaded, so this
// helper provides them too.
func app_with_catalogue(t *testing.T, item any) (*core.App, func()) {
	app, stopfn := DummyApp2(t.TempDir())
	app.AddReplaceResults(core.MakeResult(NS_CATALOGUE, item, ID_CATALOGUE)).Wait()
	return app, stopfn
}

// adding a catalogue to state realises its addons: the search tab lists them without
// the catalogue row being expanded.
func TestCatalogue__realised_on_insert(t *testing.T) {
	app, stopfn := app_with_catalogue(t, test_fixture_catalogue)
	defer stopfn()

	expected := len(test_fixture_catalogue.AddonSummaryList)
	assert.NotZero(t, expected)

	actual := app.FilterResultListByNS(NS_CATALOGUE_ADDON)
	assert.Len(t, actual, expected)
	for _, r := range actual {
		assert.Equal(t, ID_CATALOGUE, r.ParentID)
	}
	assert.True(t, app.FindResultByID(ID_CATALOGUE).ChildrenRealised)
}

// a catalogue's children are its own addons, not a fresh read from disk.
func TestCatalogue_ItemChildren(t *testing.T) {
	given := test_fixture_catalogue

	actual := given.ItemChildren(nil)

	assert.Len(t, actual, len(given.AddonSummaryList))
	for i, r := range actual {
		assert.Equal(t, NS_CATALOGUE_ADDON, r.NS)
		assert.Equal(t, given.AddonSummaryList[i], r.Item)
	}
}

// a loaded catalogue with no addons is empty.
func Test_db_catalogue_empty__loaded_and_empty(t *testing.T) {
	app, stopfn := app_with_catalogue(t, Catalogue{AddonSummaryList: []CatalogueAddon{}})
	defer stopfn()

	assert.True(t, db_catalogue_loaded(app))

	actual, err := db_catalogue_empty(app)
	assert.Nil(t, err)
	assert.True(t, actual)
}

// a loaded catalogue with addons is not empty.
func Test_db_catalogue_empty__loaded_and_populated(t *testing.T) {
	app, stopfn := app_with_catalogue(t, test_fixture_catalogue)
	defer stopfn()

	assert.True(t, db_catalogue_loaded(app))
	assert.NotEmpty(t, test_fixture_catalogue.AddonSummaryList)

	actual, err := db_catalogue_empty(app)
	assert.Nil(t, err)
	assert.False(t, actual)
}

// a result stored under the catalogue ID that isn't a catalogue is an error, not a panic.
func Test_db_catalogue_empty__not_a_catalogue(t *testing.T) {
	app, stopfn := app_with_catalogue(t, "not a catalogue")
	defer stopfn()

	_, err := db_catalogue_empty(app)
	assert.ErrorContains(t, err, "is not a catalogue")
}
