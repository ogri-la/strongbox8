package strongbox

// test data generators: addon directories, `.toc` files, nfo files and zip files built
// from in-memory data, so tests describe the addons they need rather than relying on a
// growing set of fixture files.
// clj: `test_helper.clj/gen-addon!`, `gen-addon-data`

import (
	"archive/zip"
	"bw/core"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// a file tree, path => contents. directories are implied by file paths.
type test_file_tree = map[string]string

// returns the contents of a `.toc` file with the given `title`, `version` and
// `interface_version`, plus any `extra` keys, sorted for deterministic output.
func test_gen_toc(title string, version string, interface_version string, extra map[string]string) string {
	lines := []string{
		"## Interface: " + interface_version,
		"## Title: " + title,
		"## Version: " + version,
	}
	extra_keys := []string{}
	for key := range extra {
		extra_keys = append(extra_keys, key)
	}
	slices.Sort(extra_keys)
	for _, key := range extra_keys {
		lines = append(lines, fmt.Sprintf("## %s: %s", key, extra[key]))
	}
	return strings.Join(lines, "\n") + "\n"
}

// writes the given file `tree` beneath `root`, creating directories as needed.
func test_write_tree(t *testing.T, root string, tree test_file_tree) {
	t.Helper()
	for rel_path, contents := range tree {
		full_path := filepath.Join(root, rel_path)
		if err := os.MkdirAll(filepath.Dir(full_path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full_path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writes a zip file at `zip_path` containing the given file `tree`, with explicit
// directory entries for every directory, the way most addon zips are built.
func test_write_zip(t *testing.T, zip_path string, tree test_file_tree) {
	t.Helper()
	fh, err := os.Create(zip_path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zw := zip.NewWriter(fh)

	path_list := []string{}
	for rel_path := range tree {
		path_list = append(path_list, rel_path)
	}
	slices.Sort(path_list)

	seen_dirs := map[string]bool{}
	for _, rel_path := range path_list {
		// directory entries first, "A/B/c.lua" => "A/", "A/B/"
		bits := strings.Split(rel_path, "/")
		for i := 1; i < len(bits); i++ {
			dir := strings.Join(bits[:i], "/") + "/"
			if !seen_dirs[dir] {
				seen_dirs[dir] = true
				if _, err := zw.Create(dir); err != nil {
					t.Fatal(err)
				}
			}
		}
		w, err := zw.Create(rel_path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(tree[rel_path])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// writes nfo data into `addon_dir`: a single nfo as an object, several as an array.
func test_write_nfo(t *testing.T, addon_dir string, nfo_list ...NFO) {
	t.Helper()
	var data any = nfo_list
	if len(nfo_list) == 1 {
		data = nfo_list[0]
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(addon_dir, NFO_FILENAME), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// a description of a generated addon spanning one or more directories.
type test_addon_spec struct {
	DirList          []string // "EveryAddon", "EveryAddon-BundledAddon"
	Title            string   // the primary .toc title, defaults to the first dir name
	Version          string   // defaults to "1.2.3"
	InterfaceVersion string   // defaults to "100000", retail
	Source           Source   // when set with SourceID, nfo data is written
	SourceID         string
	GroupID          string // defaults to "https://example.org/<first dir>"
	GameTrackID      GameTrackID
}

// returns the addon file tree for `spec`, one `.toc` and one `.lua` per directory.
func test_addon_tree(spec test_addon_spec) test_file_tree {
	tree := test_file_tree{}
	for i, dir := range spec.DirList {
		title := dir
		if i == 0 && spec.Title != "" {
			title = spec.Title
		}
		version := spec.Version
		if version == "" {
			version = "1.2.3"
		}
		iface := spec.InterfaceVersion
		if iface == "" {
			iface = "100000"
		}
		tree[dir+"/"+dir+".toc"] = test_gen_toc(title, version, iface, nil)
		tree[dir+"/"+dir+".lua"] = "-- " + dir
	}
	return tree
}

// returns the nfo data a strongbox install of `spec` would write for `dir`.
func test_addon_nfo(spec test_addon_spec, dir string) NFO {
	group_id := spec.GroupID
	if group_id == "" {
		group_id = "https://example.org/" + spec.DirList[0]
	}
	version := spec.Version
	if version == "" {
		version = "1.2.3"
	}
	game_track := spec.GameTrackID
	if game_track == "" {
		game_track = GAMETRACK_RETAIL
	}
	return NFO{
		InstalledVersion:     version,
		Name:                 strings.ToLower(spec.DirList[0]),
		GroupID:              group_id,
		Primary:              dir == spec.DirList[0],
		Source:               spec.Source,
		SourceID:             FlexString(spec.SourceID),
		InstalledGameTrackID: game_track,
		SourceMapList:        []SourceMap{{Source: spec.Source, SourceID: FlexString(spec.SourceID)}},
	}
}

// installs the generated addon described by `spec` directly into `addons_dir`, as though
// strongbox had installed it, writing nfo data when `spec` has a source.
// clj: `gen-addon!` with `:strongbox-installed`
func test_gen_addon(t *testing.T, addons_dir string, spec test_addon_spec) {
	t.Helper()
	test_write_tree(t, addons_dir, test_addon_tree(spec))
	if spec.Source == "" {
		return
	}
	for _, dir := range spec.DirList {
		test_write_nfo(t, filepath.Join(addons_dir, dir), test_addon_nfo(spec, dir))
	}
}

// writes a zip of the generated addon described by `spec` into `dir`, named the way
// strongbox names downloads, and returns its path.
func test_gen_addon_zip(t *testing.T, dir string, spec test_addon_spec) string {
	t.Helper()
	version := spec.Version
	if version == "" {
		version = "1.2.3"
	}
	zip_path := filepath.Join(dir, downloaded_addon_fname(strings.ToLower(spec.DirList[0]), version))
	test_write_zip(t, zip_path, test_addon_tree(spec))
	return zip_path
}

// returns a new, empty addons dir in a temporary directory.
func test_addons_dir(t *testing.T) AddonsDir {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AddOns")
	if err := core.MakeDirs(path); err != nil {
		t.Fatal(err)
	}
	return MakeAddonsDir(path)
}

// returns the sorted base names of the entries in `dir`.
// clj: `install-dir-contents`
func test_dir_contents(t *testing.T, dir string) []string {
	t.Helper()
	entry_list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	name_list := []string{}
	for _, entry := range entry_list {
		name_list = append(name_list, entry.Name())
	}
	slices.Sort(name_list)
	return name_list
}

// sets the modification time of `path` to `t`.
func chtimes(path string, when time.Time) {
	os.Chtimes(path, when, when)
}
