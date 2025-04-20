package strongbox

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_find_toc_files(t *testing.T) {
	toc_map, err := find_toc_files(filepath.Join(test_fixture_everyaddon_maximal, "EveryAddon"))
	assert.Nil(t, err)
	expected := 5 // retail, vanilla, tbc, wrath, cata
	assert.Equal(t, expected, len(toc_map))

	toc_map2, err := find_toc_files(filepath.Join(test_fixture_everyaddon_maximal, "EveryAddon_Config"))
	assert.Nil(t, err)
	expected = 1 // toc with multiple interfaces
	assert.Equal(t, expected, len(toc_map2))
}

func Test_parse_all_addon_toc_files(t *testing.T) {

}
