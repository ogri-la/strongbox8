package strongbox

import (
	"path/filepath"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func Test_inspect_zipfile__minimal(t *testing.T) {
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

	actual, err := inspect_zipfile(test_fixture_everyaddon_minimal_zip)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func Test_unzip_file(t *testing.T) {
	tmp := t.TempDir()
	expected := []string{
		"EveryAddon/",
		"EveryAddon/EveryAddon.lua",
		"EveryAddon/EveryAddon.toc",
	}
	actual, err := unzip_file(test_fixture_everyaddon_minimal_zip, tmp)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)

	assert.DirExists(t, filepath.Join(tmp, "EveryAddon/"))
	assert.FileExists(t, filepath.Join(tmp, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(tmp, "EveryAddon/EveryAddon.toc"))
}
