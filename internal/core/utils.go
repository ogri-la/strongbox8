package core

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func PathExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
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
