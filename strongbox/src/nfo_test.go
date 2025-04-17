package strongbox

import (
	"bw/core"
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

// attempting to read a nfo file that doesn't exist returns an error
func Test_read_nfo_file__dne(t *testing.T) {
	_, err := read_nfo_file(t.TempDir())
	assert.NotNil(t, err)
}

// 7.0 nfo files can be read and deserialised correctly into 8.0 structs
func Test_read_nfo_file__single_nfo_ints(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_single_ints_json)
	assert.Nil(t, err)

	expected := []NFO{test_fixture_nfo_single}
	actual, err := read_nfo_file(output_dir)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// 8.0 nfo files can be read and deserialised correctly into 8.0 structs
func Test_read_nfo_file__single_nfo_strs(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_single_strs_json)
	assert.Nil(t, err)

	expected := []NFO{test_fixture_nfo_single}
	actual, err := read_nfo_file(output_dir)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func Test_read_nfo_file__multi_nfo_mixed(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_multi_mixed_json)
	assert.Nil(t, err)

	expected := test_fixture_nfo_multi
	actual, err := read_nfo_file(output_dir)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// nfo data is correctly ignored
func Test_nfo_ignored(t *testing.T) {
	var cases = []struct {
		given    NFO
		expected bool
	}{
		{NFO{}, false},                    // neither explicitly ignored nor unignored
		{NFO{Ignored: Ptr(true)}, true},   // explicitly ignored
		{NFO{Ignored: Ptr(false)}, false}, // explicitly unignored
	}
	for _, c := range cases {
		actual := nfo_ignored(c.given)
		assert.Equal(t, c.expected, actual)
	}
}

// error returned if no nfo to pick
func Test_pick_nfo__dne(t *testing.T) {
	_, err := pick_nfo([]NFO{})
	assert.NotNil(t, err)
}

// latest nfo data is correctly picked
func Test_pick_nfo(t *testing.T) {
	var cases = []struct {
		given    []NFO
		expected NFO
	}{
		{[]NFO{{Name: "Foo"}}, NFO{Name: "Foo"}},
		{[]NFO{{Name: "Foo"}, {Name: "Bar"}, {Name: "Baz"}}, NFO{Name: "Baz"}},
	}
	for _, c := range cases {
		actual, err := pick_nfo(c.given)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func Test_is_mutual_dependency(t *testing.T) {
	var cases = []struct {
		given    []NFO
		expected bool
	}{
		{[]NFO{}, false},
		{[]NFO{{Name: "Foo"}}, false},
		{[]NFO{{Name: "Foo"}, {Name: "Bar"}}, true},
		{[]NFO{{Name: "Foo"}, {Name: "Foo"}}, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, is_mutual_dependency(c.given))
	}
}

// attempting to delete nfo data that does not exist is an error
func Test_rm_nfo__dne(t *testing.T) {
	addon_dir := t.TempDir()
	grpid := "foo"
	_, err := rm_nfo(addon_dir, grpid)
	assert.NotNil(t, err)
}

func Test_rm_nfo__single(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_single_strs_json)
	assert.Nil(t, err)

	expected := []NFO{} // hrm, grumble

	grpid := "https://foo.bar"
	actual, err := rm_nfo(output_dir, grpid)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func Test_rm_nfo(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_multi_mixed_json)
	assert.Nil(t, err)

	expected := []NFO{test_fixture_nfo_single}

	grpid := "https://bar.baz" // newest
	actual, err := rm_nfo(output_dir, grpid)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// any nfo from any position can be removed so long as the group id matches
// todo: is it possible for multiple nfo with identical group ids? it shouldn't be ...
func Test_rm_nfo__out_of_order(t *testing.T) {
	output_dir := t.TempDir()

	err := core.Spit(nfo_path(output_dir), test_fixture_nfo_multi_mixed_json)
	assert.Nil(t, err)

	expected := []NFO{test_fixture_nfo_multi[1]}

	grpid := "https://foo.bar" // oldest
	actual, err := rm_nfo(output_dir, grpid)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func Test_write_nfo(t *testing.T) {
	output_dir := t.TempDir()

	nfo := NFO{}
	nfo.GroupID = "foo"
	expected := []NFO{nfo}

	err := write_nfo(output_dir, expected)
	assert.Nil(t, err)

	actual, err := read_nfo_file(output_dir)
	assert.Nil(t, err)

	assert.Equal(t, expected, actual)
}

func Test_write_nfo__bad_cases(t *testing.T) {
	output_dir := t.TempDir()
	var cases = []struct {
		given []NFO
	}{
		{[]NFO{}},   // empty
		{[]NFO{{}}}, // contains empty
		// todo: invalid data cases
	}
	for i, c := range cases {
		err := write_nfo(output_dir, c.given)
		assert.NotNil(t, err, i)
	}
}
