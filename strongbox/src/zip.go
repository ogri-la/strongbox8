package strongbox

import (
	"archive/zip"
	"bw/core"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

type ZipReport struct {
	Contents              []string
	TopLevelDirs          mapset.Set[string]
	TopLevelFiles         mapset.Set[string]
	CompressedSizeBytes   int64
	DecompressedSizeBytes int64
}

// returns a struct capturing every path within .zip,
// a set of top-level directories and filesizes in bytes..
// note: step is new strongbox 8.0, mostly for testing and separating raw data from analysis.
func inspect_zipfile(zipfile string) (ZipReport, error) {
	empty_response := ZipReport{}

	if !core.FileExists(zipfile) {
		return empty_response, fmt.Errorf("zipfile not found: %v", zipfile)
	}

	fh, err := zip.OpenReader(zipfile)
	if err != nil {
		return empty_response, fmt.Errorf("failed to open .zip file for reading: %w", err)
	}
	defer fh.Close()

	var compressed_size_bytes int64
	var decompressed_size_bytes int64

	zip_paths := []string{}
	top_level_zip_dirs := mapset.NewSet[string]()
	top_level_zip_files := mapset.NewSet[string]()

	for _, f := range fh.File {
		zip_paths = append(zip_paths, f.Name)

		finfo := f.FileInfo()

		compressed_size_bytes += int64(f.CompressedSize64)
		//decompressed_size_bytes := f.UncompressedSize64
		decompressed_size_bytes += int64(finfo.Size()) // prefer this, there are system-dependent calculations for non-files

		bits := strings.Split(f.Name, "/")

		if finfo.IsDir() {
			top_level_zip_dirs.Add(bits[0]) // note: no trailing slash
		} else if len(bits) == 1 {
			top_level_zip_files.Add(f.Name) // note: trailing slash.
		}
	}

	return ZipReport{
		Contents:              zip_paths,
		TopLevelDirs:          top_level_zip_dirs,
		TopLevelFiles:         top_level_zip_files,
		CompressedSizeBytes:   compressed_size_bytes,
		DecompressedSizeBytes: decompressed_size_bytes,
	}, nil
}
