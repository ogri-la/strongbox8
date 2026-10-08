package strongbox

import (
	"archive/zip"
	"bw/core"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

// zip files: inspected before anything is extracted, extracted only when safe.

type ZipReport struct {
	Contents              []string           // every entry's normalised path
	TopLevelDirs          mapset.Set[string] // directories at the top level, including those only implied by a file's path
	TopLevelFiles         mapset.Set[string] // files at the top level
	UnsafeEntries         []string           // entries that would be written outside the destination, or are links
	CompressedSizeBytes   int64
	DecompressedSizeBytes int64
}

// returns the zip entry path `name` normalised: forward slashes, no leading './'.
// a Windows-built zip may separate with backslashes.
func normalise_entry_name(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	return strings.TrimPrefix(name, "./")
}

// returns `true` when the normalised entry path `name` would stay inside the directory it
// is extracted into: not absolute, no drive letter, no '..' segment.
func safe_entry_name(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':') {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return false
		}
	}
	clean := path.Clean(name)
	return clean != "." && !strings.HasPrefix(clean, "../")
}

// returns the paths, top-level entries and sizes of the given `zipfile`.
// returns an error when the file is missing or cannot be opened as a .zip.
// reports on the .zip without extracting it, so the analysis can be tested separately
// from the extraction.
// a top-level directory without an entry of its own is still reported, implied by the
// paths of the files inside it, as many zip tools write no directory entries.
// clj: `zip.clj/zipfile-normal-entries`
func inspect_zipfile(zipfile string) (ZipReport, error) {
	if !core.FileExists(zipfile) {
		return ZipReport{}, fmt.Errorf("zipfile not found: %v", zipfile)
	}

	fh, err := zip.OpenReader(zipfile)
	if err != nil {
		return ZipReport{}, fmt.Errorf("failed to open .zip file for reading: %w", err)
	}
	defer fh.Close()

	report := ZipReport{
		Contents:      []string{},
		TopLevelDirs:  mapset.NewSet[string](),
		TopLevelFiles: mapset.NewSet[string](),
		UnsafeEntries: []string{},
	}

	for _, f := range fh.File {
		name := normalise_entry_name(f.Name)
		report.Contents = append(report.Contents, name)
		report.CompressedSizeBytes += int64(f.CompressedSize64)
		report.DecompressedSizeBytes += int64(f.UncompressedSize64)

		if !safe_entry_name(name) || f.Mode()&os.ModeSymlink != 0 {
			report.UnsafeEntries = append(report.UnsafeEntries, f.Name)
			continue
		}

		is_dir := f.FileInfo().IsDir() || strings.HasSuffix(name, "/")
		bits := strings.Split(strings.TrimSuffix(name, "/"), "/")
		switch {
		case len(bits) > 1, is_dir:
			report.TopLevelDirs.Add(bits[0]) // note: no trailing slash
		default:
			report.TopLevelFiles.Add(bits[0])
		}
	}

	return report, nil
}

// returns at most three of `names`, sorted, joined for an error message, with an ellipsis
// when there are more.
func first_three(names []string) string {
	names = slices.Clone(names)
	slices.Sort(names)
	joined := strings.Join(core.Take(3, names), ", ")
	if len(names) > 3 {
		joined += ", ..."
	}
	return joined
}

// returns an error when the .zip described by `report` is not a usable addon archive.
// an addon archive must have only safe entries, no top-level files, at least one
// top-level directory, and a .toc file directly inside every top-level directory.
// at most three offending entries are named in the error.
// clj: `zip/valid-addon-zip-file?`
func valid_addon_zip_file(report ZipReport) error {
	if len(report.UnsafeEntries) > 0 {
		return fmt.Errorf("addon zip file contains entries that would be written outside the addons directory: %s", first_three(report.UnsafeEntries))
	}

	if report.TopLevelFiles.Cardinality() > 0 {
		return fmt.Errorf("addon zip file contains top level files: %s", first_three(report.TopLevelFiles.ToSlice()))
	}

	if report.TopLevelDirs.Cardinality() == 0 {
		return fmt.Errorf("addon zip file contains no directories")
	}

	dirs_with_toc := mapset.NewSet[string]()
	for _, name := range report.Contents {
		bits := strings.Split(name, "/") // "EveryAddon/EveryAddon.toc" => ["EveryAddon", "EveryAddon.toc"]
		if len(bits) == 2 && strings.HasSuffix(strings.ToLower(bits[1]), ".toc") {
			dirs_with_toc.Add(bits[0])
		}
	}

	if missing := report.TopLevelDirs.Difference(dirs_with_toc); missing.Cardinality() != 0 {
		return fmt.Errorf("addon zip file contains top level directories missing a .toc file: %s", first_three(missing.ToSlice()))
	}

	return nil
}

// returns the top-level directories of an addon zip that do not share its most common
// three-character prefix, when they look like extra addons bundled in: the directories
// fall into several prefix groups, one group is largest, and the smallest group has
// three or fewer members.
// returns nothing for a single group, several equally large groups, or only large groups
// (such as Altoholic's 'Altoholic*' and 'DataStore*'), to avoid false alarms.
// clj: `zip.clj/inconsistently-prefixed`
func inconsistently_prefixed(top_level_dirs mapset.Set[string]) []string {
	const magnitude = 3
	grouped := map[string][]string{} // prefix => dirs
	for _, dir := range top_level_dirs.ToSlice() {
		prefix := dir
		if len(dir) > 3 {
			prefix = dir[:3]
		}
		grouped[prefix] = append(grouped[prefix], dir)
	}
	if len(grouped) < 2 {
		return nil
	}

	group_list := [][]string{}
	for _, dirs := range grouped {
		slices.Sort(dirs)
		group_list = append(group_list, dirs)
	}
	slices.SortFunc(group_list, func(a, b []string) int {
		if c := cmp.Compare(len(b), len(a)); c != 0 {
			return c
		}
		return cmp.Compare(a[0], b[0])
	})

	if len(group_list[len(group_list)-1]) > magnitude {
		return nil
	}
	all_equal := true
	for _, g := range group_list {
		all_equal = all_equal && len(g) == len(group_list[0])
	}
	if all_equal {
		return nil
	}

	suspicious := []string{}
	for _, g := range group_list[1:] {
		suspicious = append(suspicious, g...)
	}
	slices.Sort(suspicious)
	return suspicious
}

// extracts the single zip entry `f` into `destination`, creating parent directories.
// directories are created readable by everyone and files keep only their permission bits,
// defaulting to readable by everyone: a zip's recorded modes can be empty or odd.
// refuses an entry that would be written outside `destination`.
func _unzip_file(destination string, f *zip.File) error {
	name := normalise_entry_name(f.Name)
	if !safe_entry_name(name) || f.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s: illegal file path", f.Name)
	}
	target := filepath.Join(destination, filepath.FromSlash(name))
	if !strings.HasPrefix(target, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("%s: illegal file path", f.Name)
	}

	if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
		return os.MkdirAll(target, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	mode := f.Mode().Perm() | 0o600 // always readable and writeable by the user
	if f.Mode().Perm() == 0 {
		mode = 0o644
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copy_err := io.Copy(out, rc)
	close_err := out.Close()
	return errors.Join(copy_err, close_err)
}

// extracts the .zip at `source` into `destination`, returning the names of the entries
// extracted.
// stops at the first failure, which may leave `destination` partly written: callers
// validate the zip first, see `valid_addon_zip_file`.
func unzip_file(source, destination string) ([]string, error) {
	r, err := zip.OpenReader(source)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return nil, err
	}

	extracted := []string{}
	for _, f := range r.File {
		if err := _unzip_file(destination, f); err != nil {
			return extracted, err
		}
		extracted = append(extracted, f.Name)
	}
	return extracted, nil
}
