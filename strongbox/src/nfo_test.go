package strongbox

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// a nfo file path is generated correctly
func Test_nfo_path(t *testing.T) {
	var cases = []struct {
		given    string
		expected string
	}{
		{"", ".strongbox.json"}, // _probably_ shouldn't allow this
		{"EveryAddon", "EveryAddon/.strongbox.json"},
		{"../EveryAddon", "../EveryAddon/.strongbox.json"}, // desirable?
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, nfo_path(c.given))
	}
}

// a vcs directory is detected and returned
func Test_version_control(t *testing.T) {
	addon_dir := t.TempDir()
	os.Mkdir(filepath.Join(addon_dir, ".hg"), 0744)
	expected := ".hg"
	actual, err := version_control(addon_dir)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// a directory that exists but does not contain a telltale vcs dir is not version controlled
func Test_version_controlled(t *testing.T) {
	addon_dir := t.TempDir()
	expected := false
	actual := version_controlled(addon_dir)
	assert.Equal(t, expected, actual)
}

// a directory that does not exist is not version controlled
func Test_version_controlled__dne(t *testing.T) {
	addon_dir := "/foo/bar"
	expected := false
	actual := version_controlled(addon_dir)
	assert.Equal(t, expected, actual)
}

// returns a complete nfo for `group_id` installed from wowinterface.
func full_nfo(group_id string, name string, version string) NFO {
	return NFO{
		InstalledVersion:     version,
		Name:                 name,
		GroupID:              group_id,
		Primary:              true,
		Source:               SOURCE_WOWI,
		SourceID:             "321",
		InstalledGameTrackID: GAMETRACK_RETAIL,
		SourceMapList:        []SourceMap{{Source: SOURCE_WOWI, SourceID: "321"}},
	}
}

// returns the nfo file contents written into a new addon dir, for reading back.
func addon_dir_with_nfo(t *testing.T, contents string) string {
	t.Helper()
	addon_dir := t.TempDir()
	os.WriteFile(nfo_path(addon_dir), []byte(contents), 0o644)
	return addon_dir
}

func Test_read_nfo_file__dne(t *testing.T) {
	_, err := read_nfo_file(t.TempDir())
	assert.ErrorIs(t, err, ErrNFODNE)
}

// strongbox 7 wrote source IDs as integers, strongbox 8 as strings.
func Test_read_nfo_file__single(t *testing.T) {
	for _, fixture := range []string{"nfofiles/single_with_ints.json", "nfofiles/single_with_strs.json"} {
		addon_dir := addon_dir_with_nfo(t, string(test_fixture_bytes(fixture)))
		actual, err := read_nfo_file(addon_dir)
		assert.NoError(t, err, fixture)
		assert.Equal(t, NFOFile{Stack: []NFO{test_fixture_nfo_single}}, actual, fixture)
		assert.False(t, actual.IsMutualDependency())
	}
}

func Test_read_nfo_file__shared_directory(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, string(test_fixture_nfo_multi_mixed_json))
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	assert.Equal(t, NFOFile{Stack: test_fixture_nfo_multi}, actual)
	assert.True(t, actual.IsMutualDependency())
	top, _ := actual.Top()
	assert.Equal(t, "https://bar.baz", top.GroupID)
}

// a strongbox 8 pre-release always wrote a list, even for a single addon.
func Test_read_nfo_file__list_of_one(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, `[{"group-id": "https://foo.bar", "primary?": true}]`)
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	assert.False(t, actual.IsMutualDependency())

	// and the next write stores a single object
	assert.NoError(t, write_nfo_file(addon_dir, actual))
	b, _ := os.ReadFile(nfo_path(addon_dir))
	assert.Equal(t, `{"group-id":"https://foo.bar","primary?":true}`, string(b))
}

// clj: `nfo_test.clj/read-nfo--v1`
func Test_read_nfo_file__v1_gains_source_map_list(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, `{"installed-version": "1.0", "name": "someaddon", "group-id": "blah", "primary?": true,
		"source": "wowinterface", "source-id": 123, "installed-game-track": "retail"}`)
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	top, _ := actual.Top()
	assert.Equal(t, []SourceMap{{Source: SOURCE_WOWI, SourceID: "123"}}, top.SourceMapList)
}

func Test_read_nfo_file__ignore_only(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, `{"ignore?": true}`)
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	assert.Equal(t, NFOFile{IgnoreFlag: new(true)}, actual)
	assert.Equal(t, new(true), actual.Ignored())
}

// invalid nfo data is reported and treated as absent, and the file is left alone.
// clj: `nfo_test.clj/read-nfo--invalid`, changed: the file is no longer deleted
func Test_read_nfo_file__invalid(t *testing.T) {
	for _, given := range []string{`{}`, `[]`, `1`, `{"foo": "bar"}`, `null`, `{"group-id": ""}`, `{"ignore?": "yes"}`,
		`{"group-id": "x", "installed-game-track": "classic-bfa"}`, `[{"group-id": "x"}, {"name": "no group"}]`, `{not json`} {
		addon_dir := addon_dir_with_nfo(t, given)
		_, err := read_nfo_file(addon_dir)
		assert.ErrorIs(t, err, ErrNFOInvalid, given)
		assert.FileExists(t, nfo_path(addon_dir), given)
	}
}

// an addon installed long ago from a host that has gone still reads.
func Test_read_nfo_file__dead_host(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, string(test_fixture_nfo_single_ints_json))
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	top, _ := actual.Top()
	assert.Equal(t, SOURCE_CURSEFORGE, top.Source)
}

func Test_write_nfo_file__single_owner_is_an_object(t *testing.T) {
	addon_dir := t.TempDir()
	given := NFOFile{Stack: []NFO{full_nfo("https://example.org/everyaddon", "everyaddon", "1.2.3")}}
	assert.NoError(t, write_nfo_file(addon_dir, given))
	b, _ := os.ReadFile(nfo_path(addon_dir))
	assert.Equal(t, byte('{'), b[0])

	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	assert.Equal(t, given, actual)
}

func Test_write_nfo_file__shared_is_an_array(t *testing.T) {
	addon_dir := t.TempDir()
	given := NFOFile{Stack: []NFO{
		full_nfo("https://example.org/everyaddon", "everyaddon", "0.1.2"),
		full_nfo("https://example.org/everyotheraddon", "everyotheraddon", "5.6.7"),
	}}
	assert.NoError(t, write_nfo_file(addon_dir, given))
	b, _ := os.ReadFile(nfo_path(addon_dir))
	assert.Equal(t, byte('['), b[0])
	actual, err := read_nfo_file(addon_dir)
	assert.NoError(t, err)
	assert.Equal(t, given, actual)
}

func Test_write_nfo_file__ignore_only(t *testing.T) {
	addon_dir := t.TempDir()
	assert.NoError(t, write_nfo_file(addon_dir, NFOFile{IgnoreFlag: new(true)}))
	b, _ := os.ReadFile(nfo_path(addon_dir))
	assert.Equal(t, `{"ignore?":true}`, string(b))
}

func Test_write_nfo_file__empty_deletes(t *testing.T) {
	addon_dir := addon_dir_with_nfo(t, `{"ignore?": true}`)
	assert.NoError(t, write_nfo_file(addon_dir, NFOFile{}))
	assert.NoFileExists(t, nfo_path(addon_dir))
	assert.NoError(t, write_nfo_file(addon_dir, NFOFile{}), "deleting a missing file is fine")
}

// strongbox only writes live hosts and game tracks, and never writes invalid data.
func Test_write_nfo_file__invalid_refused(t *testing.T) {
	cases := []NFO{
		{GroupID: ""},
		test_fixture_nfo_single, // curseforge
		func() NFO { n := full_nfo("x", "x", "1"); n.InstalledGameTrackID = "classic-bfa"; return n }(),
		func() NFO { n := full_nfo("x", "x", "1"); n.InstalledGameTrackID = GAMETRACK_RETAIL_CLASSIC; return n }(),
	}
	for _, given := range cases {
		addon_dir := t.TempDir()
		err := write_nfo_file(addon_dir, NFOFile{Stack: []NFO{given}})
		assert.Error(t, err)
		assert.NoFileExists(t, nfo_path(addon_dir))
	}
}

// clj: `core_test.clj/install-addons-with-mutual-dependencies-user-warning`
func Test_nfo_file_add(t *testing.T) {
	everyaddon := full_nfo("https://example.org/everyaddon", "everyaddon", "0.1.2")
	everyotheraddon := full_nfo("https://example.org/everyotheraddon", "everyotheraddon", "5.6.7")

	actual, msg := nfo_file_add(NFOFile{}, everyaddon, "EveryAddon-BundledAddon")
	assert.Equal(t, NFOFile{Stack: []NFO{everyaddon}}, actual)
	assert.Equal(t, "", msg)

	actual, msg = nfo_file_add(actual, everyotheraddon, "EveryAddon-BundledAddon")
	assert.Equal(t, NFOFile{Stack: []NFO{everyaddon, everyotheraddon}}, actual)
	assert.Equal(t, `"everyotheraddon" (5.6.7) replaced directory "EveryAddon-BundledAddon" of addon "everyaddon" (0.1.2)`, msg)

	// re-installing does not stack, the entry moves to the top
	actual, msg = nfo_file_add(actual, everyaddon, "EveryAddon-BundledAddon")
	assert.Equal(t, NFOFile{Stack: []NFO{everyotheraddon, everyaddon}}, actual)
	assert.NotEqual(t, "", msg)

	// an ignore-only flag is replaced
	actual, _ = nfo_file_add(NFOFile{IgnoreFlag: new(true)}, everyaddon, "EveryAddon")
	assert.Equal(t, NFOFile{Stack: []NFO{everyaddon}}, actual)
}

// clj: `nfo_test.clj/rm-nfo*`
func Test_nfo_file_rm(t *testing.T) {
	a := full_nfo("a", "a", "1")
	b := full_nfo("b", "b", "1")
	assert.Equal(t, NFOFile{}, nfo_file_rm(NFOFile{}, "a"))
	assert.Equal(t, NFOFile{}, nfo_file_rm(NFOFile{Stack: []NFO{a}}, "a"))
	assert.Equal(t, NFOFile{Stack: []NFO{b}}, nfo_file_rm(NFOFile{Stack: []NFO{a, b}}, "a"))
	assert.Equal(t, NFOFile{Stack: []NFO{a}}, nfo_file_rm(NFOFile{Stack: []NFO{a, b}}, "b"))
	assert.Equal(t, NFOFile{Stack: []NFO{a, b}}, nfo_file_rm(NFOFile{Stack: []NFO{a, b}}, "c"))
}

// clj: `nfo_test.clj/update-nfo-data-with-ignore-flags`
func Test_nfo_file_ignore_flags(t *testing.T) {
	a := full_nfo("a", "a", "1")

	// ignoring an unmanaged directory writes an ignore-only file
	assert.Equal(t, NFOFile{IgnoreFlag: new(true)}, nfo_file_ignore(NFOFile{}, false))

	// stopping ignoring an ignore-only file empties it, so it is deleted
	assert.Equal(t, NFOFile{}, nfo_file_stop_ignoring(NFOFile{IgnoreFlag: new(true)}, false))

	// stopping ignoring an implicitly ignored directory writes an explicit 'not ignored'
	assert.Equal(t, NFOFile{IgnoreFlag: new(false)}, nfo_file_stop_ignoring(NFOFile{}, true))

	// ignoring it again removes that flag, reverting to implicitly ignored
	assert.Equal(t, NFOFile{}, nfo_file_ignore(NFOFile{IgnoreFlag: new(false)}, true))

	// with full nfo data the flag lives on the top nfo
	ignored := nfo_file_ignore(NFOFile{Stack: []NFO{a}}, false)
	assert.Equal(t, new(true), ignored.Stack[0].Ignored)
	assert.Nil(t, nfo_file_stop_ignoring(ignored, false).Stack[0].Ignored)
	assert.Equal(t, new(false), nfo_file_stop_ignoring(ignored, true).Stack[0].Ignored)

	// the input is not modified
	assert.Nil(t, a.Ignored)
}

// clj: `nfo_test.clj/pin!`, `unpin!`
func Test_nfo_file_pin(t *testing.T) {
	a := full_nfo("a", "a", "1.2.3")
	b := full_nfo("b", "b", "4.5.6")
	pinned := nfo_file_pin(NFOFile{Stack: []NFO{a, b}}, "4.5.6")
	assert.Equal(t, "", pinned.Stack[0].PinnedVersion)
	assert.Equal(t, "4.5.6", pinned.Stack[1].PinnedVersion)
	assert.Equal(t, "", nfo_file_pin(pinned, "").Stack[1].PinnedVersion)
	assert.Equal(t, NFOFile{}, nfo_file_pin(NFOFile{}, "1"), "nothing to pin")
}

// property: any stack of valid nfo data survives a write and a read unchanged.
func Test_nfo_file__round_trip(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	hosts := []Source{SOURCE_GITHUB, SOURCE_GITLAB, SOURCE_WOWI}
	tracks := SUPPORTED_GAME_TRACKS.ToSlice()
	for i := range 200 {
		stack := []NFO{}
		for j := range 1 + r.Intn(4) {
			source := hosts[r.Intn(len(hosts))]
			source_id := FlexString(fmt.Sprintf("id-%d", r.Intn(1000)))
			nfo := NFO{
				GroupID:              fmt.Sprintf("https://example.org/%d-%d", i, j),
				Primary:              r.Intn(2) == 0,
				PinnedVersion:        []string{"", "1.0"}[r.Intn(2)],
				Ignored:              []*bool{nil, new(true), new(false)}[r.Intn(3)],
				InstalledVersion:     fmt.Sprintf("%d.%d", r.Intn(10), r.Intn(10)),
				Name:                 fmt.Sprintf("addon%d", j),
				Source:               source,
				SourceID:             source_id,
				InstalledGameTrackID: tracks[r.Intn(len(tracks))],
				SourceMapList:        []SourceMap{{Source: source, SourceID: source_id}},
			}
			if r.Intn(3) == 0 {
				// grouping only
				nfo = NFO{GroupID: nfo.GroupID, Primary: nfo.Primary, PinnedVersion: nfo.PinnedVersion, Ignored: nfo.Ignored}
			}
			stack = append(stack, nfo)
		}
		given := NFOFile{Stack: stack}
		addon_dir := t.TempDir()
		assert.NoError(t, write_nfo_file(addon_dir, given))
		actual, err := read_nfo_file(addon_dir)
		assert.NoError(t, err)
		assert.Equal(t, given, actual)
	}
}

func Test_derive_nfo__invalid(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	ial := []InstalledAddon{}
	pa := InstalledAddon{}
	sul := []SourceUpdate{}
	nfo := NFO{
		GroupID: "testing",
	}
	a := MakeAddon(ad, ial, pa, &nfo, nil, sul)

	// all three of these must be non-empty in order to skip the minimal nfo check
	a.Source = "dne"   // will trigger the validation error
	a.SourceID = "foo" //
	a.SourceUpdate = &SourceUpdate{}

	actual := derive_nfo(a, true)
	assert.NotNil(t, actual.Valid())
}

// the barest minimum data to get a valid nfo file
func Test_derive_nfo__minimum(t *testing.T) {
	ad := AddonsDir{
		Path: t.TempDir(),
	}
	ial := []InstalledAddon{}
	pa := InstalledAddon{}
	sul := []SourceUpdate{}
	nfo := NFO{
		GroupID: "testing",
	}
	a := MakeAddon(ad, ial, pa, &nfo, nil, sul)
	expected := NFO{
		GroupID: "testing",
		Primary: true,
	}
	actual := derive_nfo(a, true)
	assert.Equal(t, expected, actual)
	assert.Nil(t, actual.Valid())
}
