package strongbox

import (
	"bw/core"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// an addon can be installed into an empty addons dir
func Test_install_addon(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	zipfile := test_fixture_everyaddon_minimal_zip

	a, err := NewAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	opts := InstallOpts{}
	_, err = install_addon(a, ad, zipfile, opts)
	assert.Nil(t, err)

	// assertions about the state of the addons dir after unzipping

	dir_list, err := core.DirList(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(dir_list))

	file_list, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 3, len(file_list))

	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon/"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/.strongbox.json"))

	// assertions about the basic nfo file generated from a .zip file

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)
}
