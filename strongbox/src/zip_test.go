package strongbox

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func Test_inspect_zipfile__minimal(t *testing.T) {
	report, err := inspect_zipfile(test_fixture_everyaddon_minimal)
	assert.Nil(t, err)
	assert.NotNil(t, report)

	expected := ZipReport{
		Contents: []string{
			"EveryAddon/",
			"EveryAddon/EveryAddon.lua",
			"EveryAddon/EveryAddon.toc",
		},
		TopLevelDirs: mapset.NewSet(
			"EveryAddon",
		),
		TopLevelFiles:         mapset.NewSet[string](),
		CompressedSizeBytes:   188,
		DecompressedSizeBytes: 289,
	}
	assert.Equal(t, expected, report)
}

/*
func Test_inspect_zipfile__maximal(t *testing.T) {
	report, err := inspect_zipfile(test_fixture_everyaddon_maximal)
	assert.Nil(t, err)
	assert.NotNil(t, report)

	expected := ZipReport{
		Contents: []string{
			"EveryAddon/",
			"EveryAddon/EveryAddon.lua",
			"EveryAddon/EveryAddon.toc",
		},
		TopLevelDirs: mapset.NewSet(
			"EveryAddon/",
		),
		TopLevelFiles:         mapset.NewSet[string](),
		CompressedSizeBytes:   190,
		DecompressedSizeBytes: 289,
	}
	assert.Equal(t, expected, report)
}
*/
