package core

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func IntToString(val int) string {
	return strconv.Itoa(val)
}

func StringToInt(val string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(val))
}

func PathExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// returns `true` if given `file` exists and is not a directory.
// `file` may still be a symlink.
func FileExists(path string) bool {
	stat, err := os.Stat(path)
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

func MakeParents(path string) error {
	return MakeDirs(filepath.Dir(path))
}

// assumes the contents of `path` is *UTF-8 encoded text* and removes the BOM if it exists.
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

// thin wrapper around `os.WriteFile` to centralise file writing and mode setting.
// creates intermediate directories
func Spit(path string, data []byte) error {
	mode := os.FileMode(0644) // -rw-r--r--
	return os.WriteFile(path, data, mode)
}

// https://stackoverflow.com/questions/38418171/how-to-generate-unique-random-string-in-a-length-range-using-golang
func UniqueIDN(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%X", b)
}

func UniqueID() string {
	return UniqueIDN(5)
}

func PrefixedUniqueId(prefix string) string {
	return prefix + UniqueID()
}

// quick and dirty json serialisation of random data.
// returns a map with an error if input cannot be serialised.
func QuickJSON(val any) string {
	bytes, err := json.MarshalIndent(val, "", "    ")
	if err != nil {
		return `{"bw/error": "unserialisable"}`
	}
	return string(bytes)
}

// returns `path`, but rooted in the current user's home directory (~/)
// for example: `HomePath("/.config")` => `"/home/user/.config"`
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

// same as `os.ReadDir` but returns full paths
func ReadDir(path string) ([]string, error) {
	file_list, err := os.ReadDir(path)
	if err != nil {
		return []string{}, err
	}
	path_list := []string{}
	for _, file := range file_list {
		path_list = append(path_list, filepath.Join(path, file.Name()))
	}
	return path_list, nil
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

func GroupBy[T any](list_of_things []T, grouper func(T) string) map[string][]T {
	retval := map[string][]T{}
	for _, thing := range list_of_things {
		group_key := grouper(thing)
		group, present := retval[group_key]
		if !present {
			group = []T{}
		}
		group = append(group, thing)
		retval[group_key] = group
	}
	return retval
}

// similar to `GroupBy`, but the grouper can return any `comparable` value
func GroupBy2[T any, K comparable](list_of_things []T, grouper func(T) K) map[K][]T {
	retval := map[K][]T{}
	for _, thing := range list_of_things {
		group_key := grouper(thing)
		group, present := retval[group_key]
		if !present {
			group = []T{}
		}
		group = append(group, thing)
		retval[group_key] = group
	}
	return retval
}

// groups `list_of_things` by the value returned by `grouper`,
// preserving order
func Bunch[T any](list_of_things []T, grouper func(T) any) [][]T {
	final_group := [][]T{}
	current_group := []T{}
	var current_grouper any
	for _, thing := range list_of_things {
		thing_grouper := grouper(thing)
		if thing_grouper == current_grouper {
			current_group = append(current_group, thing)
			continue
		}

		if len(current_group) > 0 {
			final_group = append(final_group, current_group)
		}
		current_grouper = thing_grouper
		current_group = []T{thing}
	}
	if len(current_group) > 0 {
		final_group = append(final_group, current_group)
	}
	return final_group
}

// creates a map from the `list_of_things` by applying `keyfn` to each thing
func Index[T any](list_of_things []T, keyfn func(T) string) map[string]T {
	retval := map[string]T{}
	for _, thing := range list_of_things {
		retval[keyfn(thing)] = thing
	}
	return retval
}

func PanicBadType(thing any, expected string) {
	panic(fmt.Sprintf("programming error. expecting '%s' got '%s'", expected, reflect.TypeOf(thing)))
}

func PanicOnErr(err error) {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err.Error()))
	}
}

func FormatDateTime(dt time.Time) string {
	if dt.IsZero() {
		panic("programming error: given empty time.Time to format")
	}
	dtstr := dt.Format(time.RFC3339)
	return dtstr
}

func FormatTimeHumanOffset(t time.Time) (string, error) {
	if t.IsZero() {
		return "", errors.New("time is zero")
	}

	now := time.Now()
	if t.After(now) {
		return "", errors.New("time is in the future")
	}

	duration := now.Sub(t)
	totalSeconds := int(duration.Seconds())

	if totalSeconds < 60 {
		if totalSeconds == 1 {
			return "1 second ago", nil
		}
		return fmt.Sprintf("%d seconds ago", totalSeconds), nil
	}

	totalMinutes := totalSeconds / 60
	if totalMinutes < 60 {
		if totalMinutes == 1 {
			return "1 minute ago", nil
		}
		return fmt.Sprintf("%d minutes ago", totalMinutes), nil
	}

	totalHours := totalMinutes / 60
	if totalHours < 24 {
		if totalHours == 1 {
			return "1 hour ago", nil
		}
		return fmt.Sprintf("%d hours ago", totalHours), nil
	}

	totalDays := totalHours / 24
	if totalDays < 7 {
		if totalDays == 1 {
			return "1 day ago", nil
		}
		return fmt.Sprintf("%d days ago", totalDays), nil
	}

	totalWeeks := totalDays / 7
	if totalWeeks < 4 {
		if totalWeeks == 1 {
			return "1 week ago", nil
		}
		return fmt.Sprintf("%d weeks ago", totalWeeks), nil
	}

	totalMonths := totalDays / 30
	if totalMonths < 12 {
		if totalMonths == 1 {
			return "1 month ago", nil
		}
		return fmt.Sprintf("%d months ago", totalMonths), nil
	}

	totalYears := totalMonths / 12
	if totalYears == 1 {
		return "1 year ago", nil
	}
	return fmt.Sprintf("%d years ago", totalYears), nil
}

// a safer slice, (take n [...])
func Take[T any](n int, slice []T) []T {
	if n > len(slice) {
		n = len(slice)
	}
	if n < 0 {
		n = 0
	}
	return slice[:n]
}
