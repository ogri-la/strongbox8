package strongbox

// loading installed addons from an addons dir: grouping, primaries, implicit ignore.
// clj: `addon_test.clj/group-addons`, `load-installed-addons-*`

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the labels of `addon_list`, in order.
func addon_labels(addon_list []Addon) []string {
	labels := []string{}
	for _, a := range addon_list {
		labels = append(labels, a.Label)
	}
	return labels
}

// returns the addon labelled `label` in `addon_list`.
func addon_by_label(t *testing.T, addon_list []Addon, label string) Addon {
	t.Helper()
	for _, a := range addon_list {
		if a.Label == label {
			return a
		}
	}
	t.Fatalf("no addon labelled %q in %v", label, addon_labels(addon_list))
	return Addon{}
}

func Test_LoadAllInstalledAddons__blizzard_skipped(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"Blizzard_AuctionUI"}})
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})

	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Equal(t, []string{"EveryAddon"}, addon_labels(actual))
}

func Test_LoadAllInstalledAddons__dir_without_toc_skipped(t *testing.T) {
	ad := test_addons_dir(t)
	test_write_tree(t, ad.Path, test_file_tree{"NotAnAddon/readme.txt": "hi"})
	os.MkdirAll(filepath.Join(ad.Path, "EmptyDir"), 0o755)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})

	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Equal(t, []string{"EveryAddon"}, addon_labels(actual))
}

// directories sharing a group ID are one addon, described by the primary.
func Test_LoadAllInstalledAddons__grouped_with_primary(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{
		DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Title: "EveryAddon 1.2.3",
		Source: SOURCE_WOWI, SourceID: "321",
	})

	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Equal(t, "EveryAddon", actual[0].Label)
	assert.Equal(t, "EveryAddon", actual[0].DirName)
	assert.True(t, actual[0].IsGrouped)
	assert.Len(t, actual[0].InstalledAddonGroup, 2)
	assert.Equal(t, SOURCE_WOWI, actual[0].Source)
	assert.Equal(t, "321", actual[0].SourceID)
}

// the primary named by the nfo data represents the group even when no .toc in the group
// supports the strict addons dir's game track. found comparing strongbox 7 and 8 on a real
// addons dir: strongbox 7 chose 'BigWigs_Core' over the primary 'BigWigs'.
func Test_LoadAllInstalledAddons__primary_without_matching_toc(t *testing.T) {
	ad := test_addons_dir(t)
	ad.GameTrackID = GAMETRACK_FOREVER
	ad.Strict = true
	test_gen_addon(t, ad.Path, test_addon_spec{
		DirList: []string{"BigWigs", "BigWigs_Core"}, InterfaceVersion: "110005", Version: "v370",
		Source: SOURCE_GITHUB, SourceID: "BigWigsMods/BigWigs", GroupID: "https://github.com/BigWigsMods/BigWigs",
	})

	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Equal(t, "BigWigs", actual[0].Primary.DirName)
	assert.Equal(t, "v370", actual[0].InstalledVersion)
	assert.Nil(t, actual[0].TOC, "no .toc supports the game track")
}

// a group with no primary is named for its group ID, and chosen deterministically.
func Test_LoadAllInstalledAddons__grouped_without_primary(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddonOne", "EveryAddonTwo", "EveryAddonThree"}, Source: SOURCE_WOWI, SourceID: "1", GroupID: "everyaddonthree"}
	test_gen_addon(t, ad.Path, spec)
	for _, dir := range spec.DirList {
		nfo := test_addon_nfo(spec, dir)
		nfo.Primary = false
		test_write_nfo(t, filepath.Join(ad.Path, dir), nfo)
	}

	for range 3 {
		actual, err := LoadAllInstalledAddons(ad)
		assert.NoError(t, err)
		assert.Len(t, actual, 1)
		assert.Equal(t, "everyaddonthree (group)", actual[0].Label)
		assert.Equal(t, "EveryAddonOne", actual[0].Primary.DirName, "lowest directory name represents the group")
	}
}

// several directories claiming to be primary resolve to the lowest directory name.
func Test_LoadAllInstalledAddons__several_primaries(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"Zed", "Alpha"}, Source: SOURCE_WOWI, SourceID: "1"}
	test_gen_addon(t, ad.Path, spec)
	for _, dir := range spec.DirList {
		nfo := test_addon_nfo(spec, dir)
		nfo.Primary = true
		test_write_nfo(t, filepath.Join(ad.Path, dir), nfo)
	}
	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Equal(t, "Alpha", actual[0].Primary.DirName)
}

// directories without group IDs are addons of their own.
func Test_LoadAllInstalledAddons__ungrouped(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryOtherAddon"}})
	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Len(t, actual, 2)
	for _, a := range actual {
		assert.False(t, a.IsGrouped)
		assert.Nil(t, a.NFO)
	}
}

// clj: `addon_test.clj/load-installed-addons--invalid-nfo-data-not-loaded`
func Test_LoadAllInstalledAddons__invalid_nfo(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	os.WriteFile(filepath.Join(ad.Path, "EveryAddon", NFO_FILENAME), []byte(`{}`), 0o644)

	actual, err := LoadAllInstalledAddons(ad)
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Nil(t, actual[0].NFO)
	assert.Equal(t, "EveryAddon", actual[0].Label)
	assert.FileExists(t, filepath.Join(ad.Path, "EveryAddon", NFO_FILENAME))
}

// clj: `addon_test.clj/load-installed-addons-2`. the nfo installed version wins over the
// .toc version.
func Test_LoadAllInstalledAddons__nfo_wins(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "321", Version: "1.2.3"}
	test_gen_addon(t, ad.Path, spec)
	nfo := test_addon_nfo(spec, "EveryAddon")
	nfo.InstalledVersion = "1.2.3-nfo"
	test_write_nfo(t, filepath.Join(ad.Path, "EveryAddon"), nfo)

	actual, _ := LoadAllInstalledAddons(ad)
	assert.Equal(t, "1.2.3-nfo", actual[0].InstalledVersion)
}

func Test_LoadAllInstalledAddons__sorted(t *testing.T) {
	ad := test_addons_dir(t)
	for _, title := range []string{"zeta", "Alpha", "beta"} {
		test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"Dir" + title}, Title: title})
	}
	actual, _ := LoadAllInstalledAddons(ad)
	assert.Equal(t, []string{"Alpha", "beta", "zeta"}, addon_labels(actual))
}

// --- implicit ignore

func Test_LoadAllInstalledAddons__git_checkout_ignored(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon", ".git"), 0o755)

	actual, _ := LoadAllInstalledAddons(ad)
	assert.True(t, actual[0].IsIgnored)
}

func Test_LoadAllInstalledAddons__unrendered_version_ignored(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}, Version: "@project-version@"})
	actual, _ := LoadAllInstalledAddons(ad)
	assert.True(t, actual[0].IsIgnored)
}

// clj: `addon_test.clj/load-installed-addons--explicit-nfo-ignore`
func Test_LoadAllInstalledAddons__explicit_unignore_wins(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon", ".git"), 0o755)
	test_write_tree(t, ad.Path, test_file_tree{"EveryAddon/" + NFO_FILENAME: `{"ignore?": false}`})

	actual, _ := LoadAllInstalledAddons(ad)
	assert.False(t, actual[0].IsIgnored)
}

func Test_LoadAllInstalledAddons__ignore_only_nfo(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon"}})
	test_write_tree(t, ad.Path, test_file_tree{"EveryAddon/" + NFO_FILENAME: `{"ignore?": true}`})

	actual, _ := LoadAllInstalledAddons(ad)
	assert.True(t, actual[0].IsIgnored)
}

// clj: `core_test.clj/clear-addon-ignore-flag--group-addons`
func Test_LoadAllInstalledAddons__ignored_member_ignores_group(t *testing.T) {
	ad := test_addons_dir(t)
	test_gen_addon(t, ad.Path, test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_WOWI, SourceID: "1"})
	os.MkdirAll(filepath.Join(ad.Path, "EveryAddon-BundledAddon", ".svn"), 0o755)

	actual, _ := LoadAllInstalledAddons(ad)
	assert.Len(t, actual, 1)
	assert.True(t, actual[0].IsIgnored)
}

func Test_LoadAllInstalledAddons__pinned(t *testing.T) {
	ad := test_addons_dir(t)
	spec := test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "1"}
	test_gen_addon(t, ad.Path, spec)
	nfo := test_addon_nfo(spec, "EveryAddon")
	nfo.PinnedVersion = "1.2.3"
	test_write_nfo(t, filepath.Join(ad.Path, "EveryAddon"), nfo)

	actual, _ := LoadAllInstalledAddons(ad)
	assert.True(t, actual[0].IsPinned)
	assert.Equal(t, "1.2.3", actual[0].PinnedVersion)
}

// a strongbox 7 shared directory is read as a mutual dependency and grouped by its top
// entry.
func Test_LoadAllInstalledAddons__shared_directory(t *testing.T) {
	ad := test_addons_dir(t)
	everyaddon := test_addon_spec{DirList: []string{"EveryAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_WOWI, SourceID: "1", Version: "0.1.2"}
	other := test_addon_spec{DirList: []string{"EveryOtherAddon", "EveryAddon-BundledAddon"}, Source: SOURCE_GITHUB, SourceID: "a/b", Version: "5.6.7"}
	test_gen_addon(t, ad.Path, everyaddon)
	test_gen_addon(t, ad.Path, other)
	test_write_nfo(t, filepath.Join(ad.Path, "EveryAddon-BundledAddon"), test_addon_nfo(everyaddon, "EveryAddon-BundledAddon"), test_addon_nfo(other, "EveryAddon-BundledAddon"))

	actual, _ := LoadAllInstalledAddons(ad)
	assert.Len(t, actual, 2)
	assert.Len(t, addon_by_label(t, actual, "EveryAddon").InstalledAddonGroup, 1)
	assert.Len(t, addon_by_label(t, actual, "EveryOtherAddon").InstalledAddonGroup, 2)
}

// clj: `addon_test.clj/load-installed-addon--multi-toc`
func Test_LoadAllInstalledAddons__multi_toc(t *testing.T) {
	ad := test_addons_dir(t)
	test_write_tree(t, ad.Path, test_file_tree{
		"EveryAddon/EveryAddon.toc":         test_gen_toc("EveryAddon", "1.2.3", "110000", map[string]string{"Notes": "retail notes"}),
		"EveryAddon/EveryAddon_Vanilla.toc": test_gen_toc("EveryAddon", "1.2.3", "11503", map[string]string{"Notes": "classic notes"}),
		"EveryAddon/EveryAddon_Mists.toc":   test_gen_toc("EveryAddon", "1.2.3", "50500", map[string]string{"Notes": "mists notes"}),
	})
	for game_track, expected := range map[GameTrackID]string{GAMETRACK_RETAIL: "retail notes", GAMETRACK_CLASSIC: "classic notes", GAMETRACK_CLASSIC_MISTS: "mists notes"} {
		ad.GameTrackID = game_track
		actual, _ := LoadAllInstalledAddons(ad)
		assert.Equal(t, expected, actual[0].Description, game_track)
		assert.Equal(t, 3, actual[0].Primary.GametrackIDSet.Cardinality())
	}

	// forever has no .toc and strict finds none
	ad.GameTrackID = GAMETRACK_FOREVER
	actual, _ := LoadAllInstalledAddons(ad)
	assert.Nil(t, actual[0].TOC)
	assert.Equal(t, "EveryAddon", actual[0].Label)
	assert.Equal(t, "1.2.3", actual[0].InstalledVersion)
}
