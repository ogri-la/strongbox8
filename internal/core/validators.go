package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func IsDirValidator(val string) error {
	stat, err := os.Stat(val)
	if err == nil {
		// file, dir, symlink ... we need a dir
		if !stat.IsDir() {
			return errors.New("not a directory")
		}
		return nil
	}
	return err
}

// returns an error if the given `val` doesn't *look* like a valid file name.
// doesn't actually check if `val` exists
func IsFilenameValidator(val string) error {
	if val == "" {
		return errors.New("empty")
	}
	_, err := filepath.Abs(val)
	if err != nil {
		return err
	}
	idx := len(val) - 1
	if val[idx] == '/' {
		return errors.New("filenames cannot end with a slash '/'")
	}
	return nil
}

// returns an error if the given `val` is not writeable.
// assumes `val` is a directory.
func DirIsWriteableValidator(val string) error {
	stat, err := os.Stat(val)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return errors.New("not a directory")
	}

	// Check if the user bit is enabled in file permission
	// - https://stackoverflow.com/questions/20026320/how-to-tell-if-folder-exists-and-is-writable
	if stat.Mode().Perm()&(1<<(uint(7))) == 0 {
		// Write permission bit is not set on this file for user
		return errors.New("directory is not writeable")
	}
	return nil
}

// returns an error if the directory of the given `val` is not writeable.
// assumes `val` is a file.
func FileDirIsWriteableValidator(val string) error {
	dir := filepath.Dir(val)
	return DirIsWriteableValidator(dir)
}

// returns an error if the given `val` is not writeable.
// assumes `val` is a file in a directory that exists.
func FileIsWriteableValidator(val string) error {
	stat, err := os.Stat(val)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return errors.New("path is a directory")
	}

	// Check if the user bit is enabled in file permission
	if stat.Mode().Perm()&(1<<(uint(7))) == 0 {
		return errors.New("file is not writeable")
	}

	return nil
}

func IsYesOrNoValidator(val string) error {
	val = strings.TrimSpace(strings.ToLower(val))
	if val[0] == 'y' || val[0] == 'n' {
		return nil
	}
	return fmt.Errorf("must be 'yes' or 'no'")
}

// a validator that always returns `true`.
func AlwaysTrueValidator(_ string) error {
	return nil
}

// returns true if the given `val` matches the ID of a result in the current state
func HasResultValidator(val string) error {
	result, err := ResultIDToResult(val)
	if err != nil {
		return errors.New("not found")
	}
	if EmptyResult(result.(Result)) {
		return errors.New("found, but empty") // totally unhelpful
	}
	return nil
}

func FileExistsValidator(file string) error {
	_, err := os.Stat(file)
	if err != nil {
		return err
	}
	return nil
}
