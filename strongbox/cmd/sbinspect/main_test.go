package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	strongbox "strongbox/src"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// returns every path under `root` with its size, mode and modification time.
func tree_snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		snapshot[path] = info.Mode().String() + " " + info.ModTime().String() + " " + strconv.FormatInt(info.Size(), 10)
		return nil
	})
	require.NoError(t, err)
	return snapshot
}

// writes `files`, path => contents, under `root`.
func write_files(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0644))
		// an old modification time, so a rewrite within the same second still shows
		old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(full, old, old))
	}
}

// an addons dir: an addon installed by strongbox spanning two directories, an addon the
// user ignored, and one strongbox did not install.
func fixture_addons_dir(t *testing.T) string {
	addons := filepath.Join(t.TempDir(), "AddOns")
	nfo := `{"installed-version": "1.2.3", "name": "everyaddon", "group-id": "https://github.com/example/everyaddon",
		"primary?": %s, "source": "github", "source-id": "example/everyaddon", "installed-game-track": "retail"}`
	write_files(t, addons, map[string]string{
		"EveryAddon/EveryAddon.toc":               "## Interface: 110200\n## Title: EveryAddon\n## Version: 1.2.3\n",
		"EveryAddon/.strongbox.json":              fmt.Sprintf(nfo, "true"),
		"EveryAddon_Config/EveryAddon_Config.toc": "## Interface: 110200\n## Title: EveryAddon Config\n## Version: 1.2.3\n",
		"EveryAddon_Config/.strongbox.json":       fmt.Sprintf(nfo, "false"),
		"Ignored/Ignored.toc":                     "## Interface: 110200\n## Title: Ignored\n## Version: 0.1\n",
		"Ignored/.strongbox.json":                 `{"ignore?": true}`,
		"Unknown/Unknown.toc":                     "## Interface: 110200\n## Title: Unknown\n## Version: 9.9\n",
	})
	return addons
}

func TestRun__addons(t *testing.T) {
	addons := fixture_addons_dir(t)
	given := tree_snapshot(t, addons)

	var out bytes.Buffer
	require.NoError(t, run([]string{"addons", addons, "--game-track", "retail", "--strict=false"}, &out))

	expected := given
	actual := tree_snapshot(t, addons)
	assert.Equal(t, expected, actual, "the inspected dir is not written to")

	var report []strongbox.AddonReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	by_primary := map[string]strongbox.AddonReport{}
	for _, r := range report {
		by_primary[r.Primary] = r
	}
	require.Len(t, by_primary, 3)

	every := by_primary["EveryAddon"]
	assert.Equal(t, []string{"EveryAddon", "EveryAddon_Config"}, every.DirList)
	assert.Equal(t, "https://github.com/example/everyaddon", every.GroupID)
	assert.Equal(t, "github", every.NFOSource)
	assert.Equal(t, "1.2.3", every.InstalledVersion)
	assert.Equal(t, "EveryAddon.toc", every.TOCFile)
	assert.False(t, every.Ignored)

	ignored := by_primary["Ignored"]
	assert.True(t, ignored.Ignored)
	assert.Equal(t, "Ignored: ignored by the user", ignored.IgnoredReason)

	unknown := by_primary["Unknown"]
	assert.Empty(t, unknown.GroupID)
	assert.Equal(t, "9.9", unknown.InstalledVersion)
}

func TestRun__settings(t *testing.T) {
	dir := t.TempDir()
	write_files(t, dir, map[string]string{
		"config.json": `{"addon-dir-list": [{"addon-dir": "/does/not/exist", "game-track": "classic", "strict?": true}], "bogus": 1}`,
	})
	given := tree_snapshot(t, dir)

	var out bytes.Buffer
	require.NoError(t, run([]string{"settings", filepath.Join(dir, "config.json")}, &out))
	assert.Equal(t, given, tree_snapshot(t, dir), "the settings file is not written to")

	var report strongbox.SettingsReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, "strongbox 7", report.Format)
	assert.NotEmpty(t, report.Issues)
	require.Len(t, report.Settings.AddonsDirList, 1)
	assert.Equal(t, "/does/not/exist", report.Settings.AddonsDirList[0].Path, "a missing addons dir is kept")
	assert.Equal(t, strongbox.GAMETRACK_CLASSIC, report.Settings.AddonsDirList[0].GameTrackID)
}

func TestRun__bad_input(t *testing.T) {
	cases := [][]string{
		{},
		{"bogus"},
		{"addons"},
		{"addons", "/does/not/exist"},
		{"addons", t.TempDir(), "--game-track", "nope"},
		{"settings"},
		{"settings", "/does/not/exist.json"},
	}
	for _, given := range cases {
		var out bytes.Buffer
		assert.Error(t, run(given, &out), given)
		assert.Empty(t, out.String())
	}
}
