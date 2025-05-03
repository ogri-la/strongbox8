package strongbox

import (
	"bw/core"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// a zip can be installed into an empty addons dir,
// creating an addon
func Test_install_addon__minimal(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	zipfile := test_fixture_everyaddon_minimal_zip

	a, err := MakeAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	opts := InstallOpts{}
	_, err = install_addon(a, ad, zipfile, opts)
	assert.Nil(t, err)

	// assertions about addons dir state

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

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(toc_map))
	assert.Equal(t, "1.2.3", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into an empty addons dir,
// creating an addon
func Test_install_addon__maximal(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	zipfile := test_fixture_everyaddon_maximal_zip

	a, err := MakeAddonFromZipfile(ad, zipfile)
	assert.Nil(t, err)

	opts := InstallOpts{}
	_, err = install_addon(a, ad, zipfile, opts)
	assert.Nil(t, err)

	// assertions about addons dir state

	dir_list, err := core.DirList(ad.Path)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(dir_list))

	file_list, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon/"))

	assert.Equal(t, 7, len(file_list))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Vanilla.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_TBC.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Wrath.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/EveryAddon_Cata.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon/.strongbox.json"))

	file_list2, err := core.ListFiles(filepath.Join(ad.Path, "EveryAddon_Config/"))
	assert.Nil(t, err)
	assert.DirExists(t, filepath.Join(ad.Path, "EveryAddon_Config/"))

	assert.Equal(t, 3, len(file_list2))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/EveryAddon_Config.lua"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/EveryAddon_Config.toc"))
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon_Config/.strongbox.json"))

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))

	assert.Nil(t, err)
	assert.Equal(t, 5, len(toc_map))
	assert.Equal(t, "7.8.9", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into a populated, unmanaged, addons dir,
// updating an addon.
func Test_install_addon__update(t *testing.T) {
	ad := NewAddonsDir()
	ad.Path = t.TempDir()

	zipfile := test_fixture_everyaddon_minimal_zip

	// 'install' EveryAddon v1.2.3
	// *no* strongbox nfo
	_, err := unzip_file(zipfile, ad.Path)
	assert.Nil(t, err)

	// install EveryAddon 1.2.4 patch update
	zipfile_update := test_fixture_everyaddon_minimal_update_zip
	a, err := MakeAddonFromZipfile(ad, zipfile_update)
	assert.Nil(t, err)
	opts := InstallOpts{}
	_, err = install_addon(a, ad, zipfile_update, opts)
	assert.Nil(t, err)

	// assertions about addons dir state

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

	// assertions about file state

	nfo_list, err := read_nfo_file(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(nfo_list))
	assert.True(t, strings.HasPrefix(nfo_list[0].GroupID, "everyaddon"))
	assert.True(t, nfo_list[0].Primary)

	// future: read addon data (if it exists) before installing update to ensure nfo data contains 'installedversion'
	// future: support a 'filesystem' source and get rid of the minimal-nfo of just a 'group-id' and 'primary?'

	toc_map, err := ParseAllAddonTocFiles(filepath.Join(ad.Path, "EveryAddon/"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(toc_map))
	assert.Equal(t, "1.2.4", toc_map["EveryAddon.toc"].InstalledVersion)
}

// a zip can be installed into a populated, unmanaged, addons dir,
// completely replacing an existing addon.
func Test_install_addon__completely_replace(t *testing.T) {
	// this covers the 'completely replace' installation behaviour that would otherwise create a mutual dependency

}

// a zip can be installed into a populated, unmanaged, addons dir,
// partial replacing an existing addon and creating a mutual dependency.
func Test_install_addon__mutual_dependency(t *testing.T) {
	// this covers the 'completely replace' installation behaviour
}
