package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

func StringToInt(val string) (int, error) {
	return strconv.Atoi(val)
}

func PathExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// returns `true` if given `file` exists and is not a directory.
// `file` may still be a symlink.
func FileExists(file string) bool {
	stat, err := os.Stat(file)
	if err == nil {
		return !stat.IsDir()
	}
	return false
}

// path exists and is a directory
func DirExists(val string) bool {
	stat, err := os.Stat(val)
	if err == nil {
		return stat.IsDir()
	}
	// doesn't exist or isn't a directory
	return false
}

func PathIsWriteable(val string) bool {
	stat, err := os.Stat(val)
	if err != nil {
		// doesn't exist, something else
		return false
	}
	// Check if the user bit is enabled in file permission
	// - https://stackoverflow.com/questions/20026320/how-to-tell-if-folder-exists-and-is-writable
	if stat.Mode().Perm()&(1<<(uint(7))) == 0 {
		// Write permission bit is not set on this file for user
		return false
	}
	return true
}

func LastWriteableDir(val string) string {
	if val == "" {
		return val
	}
	parent := filepath.Dir(val)
	if !DirExists(parent) {
		return LastWriteableDir(parent)
	}
	if !PathIsWriteable(parent) {
		return LastWriteableDir(parent)
	}
	return parent
}

func MakeDirs(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func SlurpBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// assumes the contents of `path` is text and removes the BOM if it exists.
// - https://en.wikipedia.org/wiki/Byte_order_mark
func SlurpBytesUTF8(path string) ([]byte, error) {
	empty_bytes := []byte{}

	// taken from:
	// - https://stackoverflow.com/questions/21371673/reading-files-with-a-bom-in-go#answer-21375405
	fh, err := os.Open(path)
	if err != nil {
		return empty_bytes, err
	}
	defer fh.Close()

	rdr := bufio.NewReader(fh)
	r, _, err := rdr.ReadRune()
	if err != nil {
		return empty_bytes, err
	}

	if r != '\uFEFF' {
		rdr.UnreadRune() // Not a BOM -- put the rune back
	}

	b, err := io.ReadAll(rdr)
	if err != nil {
		return empty_bytes, err
	}

	return b, nil
}

func Slurp(path string) (string, error) {
	b, err := SlurpBytes(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Spit(path string, data string) error {
	mode := int(0x0644) // -rw-r--r--
	return os.WriteFile(path, []byte(data), os.FileMode(mode))
}

/* unused
func LoadJSONFile(path string) (map[string]interface{}, error) {
	var err error
	var settings map[string]interface{}
	if PathExists(path) {
		data, err := SlurpBytes(path)
		if err == nil {
			json.Unmarshal(data, &settings)
		}
	}
	return settings, err
}
*/

// crude, fat and expensive
func UniqueID() string {
	// perhaps revisit: https://github.com/mebjas/timestamp_compression
	now := time.Now().UTC().UnixNano()
	r := rand.Intn(9)
	return fmt.Sprintf("%d%d", now, r)
}

// quick and dirty json serialisation of random data.
// returns a map with an error if input cannot be serialised.
func QuickJSON(val interface{}) string {
	bytes, err := json.MarshalIndent(val, "", "    ")
	if err != nil {
		return `{"bw/error": "unserialisable"}`
	}
	return string(bytes)
}

func HomePath(path string) string {
	user, err := user.Current()
	if err != nil {
		panic(fmt.Errorf("failed to find current user: %w", err))
	}
	if path == "" {
		return user.HomeDir
	}
	if path[0] != '/' {
		panic("programming error. path for user home must start with a forward slash")
	}
	return filepath.Join(user.HomeDir, path)
}

// returns `true` if given `path` is a directory.
// any errors are treated as `false`.
func IsDir(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// returns a list of absolute paths to directories found at the given `path`.
func DirList(path string) ([]string, error) {
	if !IsDir(path) {
		return []string{}, errors.New("not a directory")
	}
	file_list, err := os.ReadDir(path)
	if err != nil {
		return []string{}, err
	}
	dir_list := []string{}
	for _, dir := range file_list {
		full_path := filepath.Join(path, dir.Name())
		if IsDir(full_path) {
			dir_list = append(dir_list, full_path)
		}
	}
	return dir_list, nil
}

// returns a list of all *files* found at the given `path`.
func ListFiles(path string) ([]string, error) {
	if !IsDir(path) {
		return []string{}, errors.New("not a directory, cannot list files")
	}
	path_list, err := os.ReadDir(path)
	if err != nil {
		return []string{}, err
	}
	file_list := []string{}
	for _, file := range path_list {
		full_path := filepath.Join(path, file.Name())
		if !IsDir(full_path) {
			file_list = append(file_list, full_path)
		}
	}
	return file_list, nil
}
