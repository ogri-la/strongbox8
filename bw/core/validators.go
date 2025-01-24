package core

import (
	"errors"
	"os"
	"path/filepath"
)

func IsDirValidator(_val interface{}) error {
	stat, err := os.Stat(_val.(string))
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
func IsFilenameValidator(_val interface{}) error {
	val, is_str := _val.(string)
	if !is_str {
		return errors.New("not a string")
	}
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
func DirIsWriteableValidator(_val interface{}) error {
	val, is_str := _val.(string)
	if !is_str {
		return errors.New("not a string")
	}
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
func FileDirIsWriteableValidator(_val interface{}) error {
	val, is_str := _val.(string)
	if !is_str {
		return errors.New("not a string")
	}
	dir := filepath.Dir(val)
	return DirIsWriteableValidator(dir)
}

// returns an error if the given `val` is not writeable.
// assumes `val` is a file in a directory that exists.
func FileIsWriteableValidator(_val interface{}) error {
	val, is_str := _val.(string)
	if !is_str {
		return errors.New("not a string")
	}
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

func AlwaysTrueValidator(_ interface{}) error {
	return nil
}

// returns true if the given `val` matches the ID of a result in the current state
func HasResultValidator(_val interface{}) error {
	if EmptyResult(_val.(Result)) {
		return errors.New("result not found")
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
