package strongbox

import (
	"errors"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func Test_load_installed_addon__empty_dir(t *testing.T) {
	empty_addon_dir := t.TempDir()
	_, err := load_installed_addon(empty_addon_dir)
	assert.NotNil(t, err)
}

func Test_determine_primary_subdir(t *testing.T) {
	var cases = []struct {
		given    mapset.Set[string]
		expected string
	}{
		{mapset.NewSet("Foo"), "Foo"},
		{mapset.NewSet("Foo", "FooBar", "FooBarBaz"), "Foo"},
		{mapset.NewSet("FooBarBaz", "FooBar", "Foo"), "Foo"}, // maps have no order
	}
	for _, c := range cases {
		actual, err := determine_primary_subdir(c.given)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func Test_determine_primary_subdir__error_cases(t *testing.T) {
	var cases = []struct {
		given    mapset.Set[string]
		expected error
	}{
		{mapset.NewSet[string](), errors.New("empty set")},
		{mapset.NewSet("Foo", "Bar"), errors.New("no common directory prefix")},
		{mapset.NewSet("Foo", "Bar", "Baz"), errors.New("no common directory prefix")},
	}
	for _, c := range cases {
		_, err := determine_primary_subdir(c.given)
		assert.NotNil(t, err)
		assert.Equal(t, c.expected, err)
	}
}
