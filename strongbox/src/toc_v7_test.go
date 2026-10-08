package strongbox

// .toc parsing cases ported from strongbox 7's `toc_test.clj`.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

const v7_toc_file_contents = "## Version: 1.6.1\n" +
	"## Title: Addon Name\n" +
	"## Notes: Description of the addon here\n" +
	"## Description: Another description here??\n" +
	"## Author: John Doe\n" +
	"## X-WoWI-ID: 12345\n" +
	"## X-Curse-Project-ID: 54321\n" +
	"\n" +
	"## Multi: Colon: Madness:\n" +
	"\n" +
	"# Ignored: Regular comment\n" +
	"## Ignored, no colon\n" +
	" ## Ignored: Leading whitespace\n" +
	"\n" +
	"##Foo: Bar\n" +
	"## Bar:Baz\n" +
	"\n" +
	"############## Bup: Foo\n" +
	"\n" +
	"## Over: to dinner\n" +
	"## Over: written\n" +
	"\n" +
	"# # Comment1: ignored, must be bang-space-double-bang\n" +
	"# ## Comment2: comment2\n" +
	"# ##Comment3:comment3\n" +
	"\n" +
	"#@retail@\n" +
	"## Interface: 80205\n" +
	"#@end-retail@\n" +
	"#@non-retail@\n" +
	"# ## Interface: 11302\n" +
	"#@end-non-retail@\n" +
	"  \n" +
	"SomeAddon.lua"

// clj: `toc_test.clj/parse-toc-file`
func Test_parse_toc_file__v7(t *testing.T) {
	expected := map[string]string{
		"version":            "1.6.1",
		"title":              "Addon Name",
		"notes":              "Description of the addon here",
		"description":        "Another description here??",
		"author":             "John Doe",
		"x-curse-project-id": "54321",
		"x-wowi-id":          "12345",
		"multi":              "Colon: Madness:",
		"foo":                "Bar",
		"bar":                "Baz",
		"bup":                "Foo",
		"over":               "written",
		"#comment2":          "comment2",
		"#comment3":          "comment3",
		"interface":          "80205",
		"#interface":         "11302",
	}
	assert.Equal(t, expected, parse_toc_file(v7_toc_file_contents))
}

// clj: `toc_test.clj/parse-toc-file--empty`
func Test_parse_toc_file__empty(t *testing.T) {
	assert.Empty(t, parse_toc_file(""))
}

// clj: `toc_test.clj/parse-addon-toc-guard`
func Test_ParseAllAddonTocFiles__v7(t *testing.T) {
	addon_dir := filepath.Join(t.TempDir(), "SomeAddon")
	os.MkdirAll(addon_dir, 0o755)
	os.WriteFile(filepath.Join(addon_dir, "SomeAddon.toc"), []byte(v7_toc_file_contents), 0o644)

	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	assert.NoError(t, err)
	actual := toc_map["SomeAddon.toc"]
	assert.Equal(t, "addon-name", actual.Name)
	assert.Equal(t, "Addon Name", actual.Label)
	assert.Equal(t, "SomeAddon", actual.DirName)
	assert.Equal(t, "Description of the addon here", actual.Notes)
	assert.Equal(t, mapset.NewSet(80205, 11302), actual.InterfaceVersionSet)
	assert.Equal(t, mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC), actual.GameTrackIDSet)
	assert.Equal(t, "1.6.1", actual.InstalledVersion)
	assert.Equal(t, []SourceMap{{Source: SOURCE_WOWI, SourceID: "12345"}}, actual.SourceMapList)
}

// clj: `toc_test.clj/parse-addon-toc`
func Test_coerce_toc_data__defaults(t *testing.T) {
	given := map[string]string{}
	actual := coerce_toc_data(given, "/path/to/EveryAddon/EveryAddon.toc")
	assert.Equal(t, "EveryAddon *", actual.Label)
	assert.Equal(t, "everyaddon", actual.Name)
	assert.False(t, actual.Ignored)

	given = map[string]string{"title": "EveryAddon", "version": "@project-version@"}
	actual = coerce_toc_data(given, "/path/to/EveryAddon/EveryAddon.toc")
	assert.True(t, actual.Ignored)
}

// clj: `toc_test.clj/parse-addon-toc--x-source`
func Test_coerce_toc_data__x_source(t *testing.T) {
	cases := []struct {
		given    map[string]string
		expected []SourceMap
	}{
		{map[string]string{"x-wowi-id": "123"}, []SourceMap{{Source: SOURCE_WOWI, SourceID: "123"}}},
		{map[string]string{"x-wowi-id": "abc"}, []SourceMap{}},
		{map[string]string{"x-github": "https://github.com/a/b"}, []SourceMap{{Source: SOURCE_GITHUB, SourceID: "a/b"}}},
		{map[string]string{"x-website": "https://github.com/a/b"}, []SourceMap{{Source: SOURCE_GITHUB, SourceID: "a/b"}}},
		{map[string]string{"x-website": "https://example.org/a/b"}, []SourceMap{}},
		// wowinterface first, github second
		{map[string]string{"x-wowi-id": "123", "x-github": "https://github.com/a/b"},
			[]SourceMap{{Source: SOURCE_WOWI, SourceID: "123"}, {Source: SOURCE_GITHUB, SourceID: "a/b"}}},
	}
	for _, c := range cases {
		c.given["title"] = "EveryAddon"
		actual := coerce_toc_data(c.given, "/path/to/EveryAddon/EveryAddon.toc")
		assert.Equal(t, c.expected, actual.SourceMapList, c.given)
	}
}

// clj: `toc_test.clj/rm-trailing-version`
func Test_rm_trailing_version(t *testing.T) {
	cases := map[string]string{
		"Grid 2":                         "Grid",
		"Carbonite Maps v8.2.0":          "Carbonite Maps",
		"Grid2":                          "Grid2",
		"Bartender4":                     "Bartender4",
		"|cFF00FF00EveryAddon|r":         "|cFF00FF00EveryAddon|r",
		"BigWigs [TBC Classic]":          "BigWigs [TBC Classic]",
		"foo ...":                        "foo",
		"foo v.0.":                       "foo",
		"foo v2019.01.01":                "foo",
		"Healbot Continued 9.2.0.12":     "Healbot Continued",
		"ElvUI_CustomTweaks 1.0.0-alpha": "ElvUI_CustomTweaks 1.0.0-alpha",
	}
	for given, expected := range cases {
		assert.Equal(t, expected, rm_trailing_version(given), given)
	}
}

// clj: `toc_test.clj/parse-interface-value`
func Test_parse_interface_value(t *testing.T) {
	cases := []struct {
		given    string
		expected mapset.Set[int]
	}{
		{"", mapset.NewSet[int]()},
		{"100206", mapset.NewSet(100206)},
		{"100206, 40400, 11502", mapset.NewSet(100206, 40400, 11502)},
		{" , 100206,,40400 , ", mapset.NewSet(100206, 40400)},
		{"100206, 100206", mapset.NewSet(100206)},
		{"foo", mapset.NewSet[int]()},
		{"1.2.3, 100206", mapset.NewSet(100206)},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, parse_interface_value(c.given), c.given)
	}
}

// clj: `toc_test.clj/parse-addon-toc--multiple-interface-versions`
func Test_coerce_toc_data__multiple_interface_versions(t *testing.T) {
	actual := coerce_toc_data(map[string]string{"title": "x", "interface": "100206, 40400, 11502"}, "/p/EveryAddon/EveryAddon.toc")
	assert.Equal(t, mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC_CATA, GAMETRACK_CLASSIC), actual.GameTrackIDSet)
}

// clj: `toc_test.clj/parse-addon-toc--invalid-toc-questie`
// an unsupported-client fallback .toc has an invalid interface version and no game track.
func Test_ParseTOCFile__questie_fallback(t *testing.T) {
	addon_dir := filepath.Join(t.TempDir(), "Questie")
	os.MkdirAll(addon_dir, 0o755)
	os.WriteFile(filepath.Join(addon_dir, "Questie.toc"), test_fixture_bytes("v7/questie--invalid.toc"), 0o644)
	toc, err := ParseTOCFile(filepath.Join(addon_dir, "Questie.toc"))
	assert.NoError(t, err)
	assert.Empty(t, toc.GameTrackIDSet.ToSlice())
}

// a leading byte order mark does not spoil the first key.
func Test_ParseTOCFile__bom(t *testing.T) {
	addon_dir := filepath.Join(t.TempDir(), "EveryAddon")
	os.MkdirAll(addon_dir, 0o755)
	toc_path := filepath.Join(addon_dir, "EveryAddon.toc")
	os.WriteFile(toc_path, []byte("\ufeff## Title: EveryAddon\n## Interface: 100000\n"), 0o644)
	toc, err := ParseTOCFile(toc_path)
	assert.NoError(t, err)
	assert.Equal(t, "EveryAddon", toc.Title)
}

// the parser never panics, and every key it returns is lower case and trimmed.
func FuzzParseTOC(f *testing.F) {
	f.Add(v7_toc_file_contents)
	f.Add("## Title: x\r\n## Interface: 1\r\n")
	f.Add("# ## : \n##:\n## :x")
	f.Fuzz(func(t *testing.T, given string) {
		actual := parse_toc_file(given)
		for key, val := range actual {
			if utf8.ValidString(key) && key != strings.ToLower(key) {
				t.Errorf("key not lower case: %q", key)
			}
			if val != strings.TrimSpace(val) || val == "" {
				t.Errorf("value not trimmed or empty: %q", val)
			}
		}
		coerce_toc_data(actual, "/path/to/EveryAddon/EveryAddon.toc")
	})
}
