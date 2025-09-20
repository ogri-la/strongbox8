package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBunchEmpty(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{}
	expected := [][]string{}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunchAllUnique(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "a", "aa"}
	expected := [][]string{
		{"aaa"},
		{"a"},
		{"aa"},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunchUnbunched(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "a", "aa", "bbb", "b", "bb"}
	expected := [][]string{
		{"aaa"},
		{"a"},
		{"aa"},
		{"bbb"},
		{"b"},
		{"bb"},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunch(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "bbb", "a", "aa", "bb", "b", "c", "d", "ee", "ff", ""}
	expected := [][]string{
		{"aaa", "bbb"},
		{"a"},
		{"aa", "bb"},
		{"b", "c", "d"},
		{"ee", "ff"},
		{""},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestIntToString(t *testing.T) {
	assert.Equal(t, "0", IntToString(0))
	assert.Equal(t, "42", IntToString(42))
	assert.Equal(t, "-1", IntToString(-1))
	assert.Equal(t, "999999", IntToString(999999))
}

func TestStringToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"0", 0, false},
		{"42", 42, false},
		{"-1", -1, false},
		{"  123  ", 123, false}, // should trim whitespace
		{"abc", 0, true},
		{"", 0, true},
		{"12.34", 0, true},
	}

	for _, test := range tests {
		result, err := StringToInt(test.input)
		if test.hasError {
			assert.Error(t, err, "Expected error for input: %s", test.input)
		} else {
			assert.NoError(t, err, "Unexpected error for input: %s", test.input)
			assert.Equal(t, test.expected, result, "Wrong result for input: %s", test.input)
		}
	}
}

func TestQuickJSON(t *testing.T) {
	// Test simple struct
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	testObj := TestStruct{Name: "Alice", Age: 30}
	result := QuickJSON(testObj)

	// Should contain formatted JSON
	assert.Contains(t, result, `"name": "Alice"`)
	assert.Contains(t, result, `"age": 30`)

	// Test unserialisable object
	unserialisable := make(chan int) // channels can't be JSON marshalled
	result = QuickJSON(unserialisable)
	assert.Equal(t, `{"bw/error": "unserialisable"}`, result)
}

func TestPathExists(t *testing.T) {
	// Test with current directory (should exist)
	assert.True(t, PathExists("."), "Current directory should exist")

	// Test with non-existent path
	assert.False(t, PathExists("/this/path/should/not/exist/at/all"), "Non-existent path should return false")

	// Test with parent directory (should exist)
	assert.True(t, PathExists(".."), "Parent directory should exist")
}

func TestFileExists(t *testing.T) {
	// Test with current directory (should return false - it's a directory, not a file)
	assert.False(t, FileExists("."), "Current directory should return false for FileExists")

	// Test with this test file (should exist)
	assert.True(t, FileExists("utils_test.go"), "This test file should exist")

	// Test with non-existent file
	assert.False(t, FileExists("this_file_does_not_exist.txt"), "Non-existent file should return false")
}

func TestDirExists(t *testing.T) {
	// Test with current directory (should exist)
	assert.True(t, DirExists("."), "Current directory should exist")

	// Test with parent directory (should exist)
	assert.True(t, DirExists(".."), "Parent directory should exist")

	// Test with this test file (should return false - it's a file, not a directory)
	assert.False(t, DirExists("utils_test.go"), "Test file should return false for DirExists")

	// Test with non-existent directory
	assert.False(t, DirExists("/this/directory/should/not/exist"), "Non-existent directory should return false")
}

func TestMakeDirs(t *testing.T) {
	// Test with current directory (should not error)
	err := MakeDirs(".")
	assert.NoError(t, err, "MakeDirs with current directory should not error")

	// We won't test creating actual directories in unit tests to avoid side effects
}

func TestMakeParents(t *testing.T) {
	// Test with current directory path
	err := MakeParents("./test_file.txt")
	assert.NoError(t, err, "MakeParents should not error for existing directory")

	// Test with empty string
	err = MakeParents("")
	assert.NoError(t, err, "MakeParents with empty string should not error")
}

func TestUniqueID(t *testing.T) {
	// Test that UniqueID generates different IDs each time
	id1 := UniqueID()
	id2 := UniqueID()
	id3 := UniqueID()

	assert.NotEqual(t, id1, id2, "UniqueID should generate different IDs")
	assert.NotEqual(t, id2, id3, "UniqueID should generate different IDs")
	assert.NotEqual(t, id1, id3, "UniqueID should generate different IDs")

	// Test that IDs are the expected length (5 bytes = 10 hex chars)
	assert.Len(t, id1, 10, "UniqueID should be 10 characters (5 bytes in hex)")
	assert.Len(t, id2, 10, "UniqueID should be 10 characters (5 bytes in hex)")
	assert.Len(t, id3, 10, "UniqueID should be 10 characters (5 bytes in hex)")

	// Test that IDs contain only valid hex characters
	for _, char := range id1 {
		assert.True(t, (char >= '0' && char <= '9') || (char >= 'A' && char <= 'F'),
			"UniqueID should contain only hex characters, got: %c", char)
	}
}

func TestUniqueIDN(t *testing.T) {
	tests := []struct {
		n              int
		expectedLength int
	}{
		{1, 2},   // 1 byte = 2 hex chars
		{2, 4},   // 2 bytes = 4 hex chars
		{5, 10},  // 5 bytes = 10 hex chars
		{10, 20}, // 10 bytes = 20 hex chars
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("n_%d", test.n), func(t *testing.T) {
			id := UniqueIDN(test.n)
			assert.Len(t, id, test.expectedLength, "UniqueIDN(%d) should be %d characters", test.n, test.expectedLength)

			// Test that multiple calls generate different IDs
			id2 := UniqueIDN(test.n)
			assert.NotEqual(t, id, id2, "UniqueIDN should generate different IDs")

			// Test that IDs contain only valid hex characters
			for _, char := range id {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'A' && char <= 'F'),
					"UniqueIDN should contain only hex characters, got: %c", char)
			}
		})
	}

	// Test uniqueness with many IDs
	t.Run("uniqueness_test", func(t *testing.T) {
		generated := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id := UniqueIDN(5)
			assert.False(t, generated[id], "UniqueIDN generated duplicate ID: %s", id)
			generated[id] = true
		}
	})
}

func TestPrefixedUniqueId(t *testing.T) {
	tests := []string{"test-", "user_", "item:", ""}

	for _, prefix := range tests {
		t.Run("prefix_"+prefix, func(t *testing.T) {
			id1 := PrefixedUniqueId(prefix)
			id2 := PrefixedUniqueId(prefix)

			// Should start with the prefix
			assert.True(t, strings.HasPrefix(id1, prefix), "ID should start with prefix '%s', got: %s", prefix, id1)
			assert.True(t, strings.HasPrefix(id2, prefix), "ID should start with prefix '%s', got: %s", prefix, id2)

			// Should be different
			assert.NotEqual(t, id1, id2, "PrefixedUniqueId should generate different IDs")

			// Should have the right total length (prefix + 10 chars for UniqueID)
			expectedLen := len(prefix) + 10
			assert.Len(t, id1, expectedLen, "PrefixedUniqueId should be %d characters total", expectedLen)
		})
	}
}

func TestIsDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test with actual directory
	assert.True(t, IsDir(tempDir), "IsDir should return true for actual directory")

	// Test with current directory
	assert.True(t, IsDir("."), "IsDir should return true for current directory")

	// Create a temporary file
	tempFile := filepath.Join(tempDir, "test_file.txt")
	err := os.WriteFile(tempFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test with file (should return false)
	assert.False(t, IsDir(tempFile), "IsDir should return false for file")

	// Test with non-existent path
	assert.False(t, IsDir(filepath.Join(tempDir, "nonexistent")), "IsDir should return false for non-existent path")
}

func TestPathIsWriteable(t *testing.T) {
	tempDir := t.TempDir()

	// Test with writable directory
	assert.True(t, PathIsWriteable(tempDir), "PathIsWriteable should return true for temp directory")

	// Test with current directory (should be writable in most cases)
	assert.True(t, PathIsWriteable("."), "PathIsWriteable should return true for current directory")

	// Test with non-existent path
	assert.False(t, PathIsWriteable(filepath.Join(tempDir, "nonexistent")), "PathIsWriteable should return false for non-existent path")
}

func TestLastWriteableDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test with existing writable directory - it should return the parent dir
	result := LastWriteableDir(tempDir)
	expectedParent := filepath.Dir(tempDir)
	assert.Equal(t, expectedParent, result, "LastWriteableDir should return the parent directory")

	// Test with nested path where parent exists
	nestedPath := filepath.Join(tempDir, "subdir", "nested", "path")
	result = LastWriteableDir(nestedPath)
	assert.Equal(t, tempDir, result, "LastWriteableDir should return the highest writable parent")

	// Test with empty string
	result = LastWriteableDir("")
	assert.Equal(t, "", result, "LastWriteableDir should return empty string for empty input")
}

func TestMakeDirsWithTempDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test creating nested directories
	nestedPath := filepath.Join(tempDir, "level1", "level2", "level3")
	err := MakeDirs(nestedPath)
	assert.NoError(t, err, "MakeDirs should not error when creating nested directories")

	// Verify the directories were created
	assert.True(t, DirExists(nestedPath), "Nested directories should be created")
	assert.True(t, DirExists(filepath.Join(tempDir, "level1")), "Level1 should be created")
	assert.True(t, DirExists(filepath.Join(tempDir, "level1", "level2")), "Level2 should be created")
}

func TestMakeParentsWithTempDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test creating parent directories for a file path
	filePath := filepath.Join(tempDir, "subdir1", "subdir2", "file.txt")
	err := MakeParents(filePath)
	assert.NoError(t, err, "MakeParents should not error")

	// Verify parent directories were created
	parentDir := filepath.Dir(filePath)
	assert.True(t, DirExists(parentDir), "Parent directories should be created")
	assert.True(t, DirExists(filepath.Join(tempDir, "subdir1")), "subdir1 should be created")
}

func TestSpit(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_file.txt")
	testData := []byte("Hello, World!\nThis is test data.")

	// Test writing file
	err := Spit(filePath, testData)
	assert.NoError(t, err, "Spit should not error when writing file")

	// Verify file exists and has correct content
	assert.True(t, FileExists(filePath), "File should exist after Spit")

	// Read back the content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err, "Should be able to read file back")
	assert.Equal(t, testData, content, "File content should match what was written")

	// Test overwriting existing file
	newData := []byte("Overwritten content")
	err = Spit(filePath, newData)
	assert.NoError(t, err, "Spit should not error when overwriting file")

	content, err = os.ReadFile(filePath)
	assert.NoError(t, err, "Should be able to read overwritten file")
	assert.Equal(t, newData, content, "File content should be overwritten")
}

func TestReadDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create some test files and directories
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	subdir := filepath.Join(tempDir, "subdir")

	err := os.WriteFile(file1, []byte("content1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	assert.NoError(t, err)
	err = os.Mkdir(subdir, 0755)
	assert.NoError(t, err)

	// Test ReadDir
	paths, err := ReadDir(tempDir)
	assert.NoError(t, err, "ReadDir should not error")
	assert.Len(t, paths, 3, "ReadDir should return 3 items")

	// Convert to map for easier checking
	pathMap := make(map[string]bool)
	for _, path := range paths {
		pathMap[path] = true
		// All paths should be absolute
		assert.True(t, filepath.IsAbs(path), "ReadDir should return absolute paths: %s", path)
	}

	assert.True(t, pathMap[file1], "ReadDir should include file1")
	assert.True(t, pathMap[file2], "ReadDir should include file2")
	assert.True(t, pathMap[subdir], "ReadDir should include subdir")

	// Test with non-existent directory
	_, err = ReadDir(filepath.Join(tempDir, "nonexistent"))
	assert.Error(t, err, "ReadDir should error for non-existent directory")
}

func TestDirList(t *testing.T) {
	tempDir := t.TempDir()

	// Create test structure
	subdir1 := filepath.Join(tempDir, "subdir1")
	subdir2 := filepath.Join(tempDir, "subdir2")
	file1 := filepath.Join(tempDir, "file1.txt")

	err := os.Mkdir(subdir1, 0755)
	assert.NoError(t, err)
	err = os.Mkdir(subdir2, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(file1, []byte("content"), 0644)
	assert.NoError(t, err)

	// Test DirList
	dirs, err := DirList(tempDir)
	assert.NoError(t, err, "DirList should not error")
	assert.Len(t, dirs, 2, "DirList should return only directories")

	// Convert to map for easier checking
	dirMap := make(map[string]bool)
	for _, dir := range dirs {
		dirMap[dir] = true
		// All paths should be absolute
		assert.True(t, filepath.IsAbs(dir), "DirList should return absolute paths: %s", dir)
		// All should be directories
		assert.True(t, IsDir(dir), "DirList should return only directories: %s", dir)
	}

	assert.True(t, dirMap[subdir1], "DirList should include subdir1")
	assert.True(t, dirMap[subdir2], "DirList should include subdir2")
	assert.False(t, dirMap[file1], "DirList should not include files")

	// Test with non-directory
	_, err = DirList(file1)
	assert.Error(t, err, "DirList should error when called on a file")
	assert.Contains(t, err.Error(), "not a directory")
}

func TestSlurpBytesUTF8(t *testing.T) {
	tempDir := t.TempDir()

	// Test with regular UTF-8 file (no BOM)
	t.Run("regular_utf8", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "regular.txt")
		testContent := "Hello, 世界! 🌍"
		err := os.WriteFile(filePath, []byte(testContent), 0644)
		assert.NoError(t, err)

		content, err := SlurpBytesUTF8(filePath)
		assert.NoError(t, err, "SlurpBytesUTF8 should not error")
		assert.Equal(t, testContent, string(content), "Content should match")
	})

	// Test with UTF-8 BOM file
	t.Run("utf8_with_bom", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "with_bom.txt")
		testContent := "Hello, 世界! 🌍"
		// UTF-8 BOM is 0xEF, 0xBB, 0xBF
		bomContent := []byte{0xEF, 0xBB, 0xBF}
		bomContent = append(bomContent, []byte(testContent)...)
		err := os.WriteFile(filePath, bomContent, 0644)
		assert.NoError(t, err)

		content, err := SlurpBytesUTF8(filePath)
		assert.NoError(t, err, "SlurpBytesUTF8 should not error")
		assert.Equal(t, testContent, string(content), "BOM should be stripped")
	})

	// Test with empty file
	t.Run("empty_file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "empty.txt")
		err := os.WriteFile(filePath, []byte{}, 0644)
		assert.NoError(t, err)

		content, err := SlurpBytesUTF8(filePath)
		// Empty files return EOF which is expected behavior
		if err != nil {
			assert.Equal(t, io.EOF, err, "Empty file should return EOF")
			assert.Empty(t, content)
		} else {
			assert.Empty(t, content)
		}
	})

	// Test with non-existent file
	t.Run("nonexistent_file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent.txt")

		_, err := SlurpBytesUTF8(filePath)
		assert.Error(t, err, "SlurpBytesUTF8 should error for non-existent file")
	})
}

func TestHomePath(t *testing.T) {
	// Test with empty path (should return home directory)
	homePath := HomePath("")
	assert.NotEmpty(t, homePath)
	assert.True(t, strings.HasPrefix(homePath, "/"))

	// Test with valid absolute path from home
	homePath = HomePath("/test")
	assert.True(t, strings.Contains(homePath, "test"))
	assert.True(t, strings.HasPrefix(homePath, "/"))

	// Test with invalid path (should panic)
	assert.Panics(t, func() {
		HomePath("relative/path")
	})
}

func TestListFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files and directories
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	subDir := filepath.Join(tempDir, "subdir")

	err := os.WriteFile(file1, []byte("test1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(file2, []byte("test2"), 0644)
	assert.NoError(t, err)
	err = os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	// Test listing files
	files, err := ListFiles(tempDir)
	assert.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Contains(t, files, file1)
	assert.Contains(t, files, file2)

	// Should not contain the subdirectory
	assert.NotContains(t, files, subDir)

	// Test with non-existent directory
	_, err = ListFiles(filepath.Join(tempDir, "nonexistent"))
	assert.Error(t, err)

	// Test with file instead of directory
	_, err = ListFiles(file1)
	assert.Error(t, err)
}

func TestGroupBy(t *testing.T) {
	input := []string{"a", "bb", "ccc", "dd", "e"}

	// Group by string length
	result := GroupBy(input, func(s string) string {
		return fmt.Sprintf("len_%d", len(s))
	})

	assert.Len(t, result, 3)
	assert.Equal(t, []string{"a", "e"}, result["len_1"])
	assert.Equal(t, []string{"bb", "dd"}, result["len_2"])
	assert.Equal(t, []string{"ccc"}, result["len_3"])
}

func TestGroupBy2(t *testing.T) {
	input := []string{"a", "bb", "ccc", "dd", "e"}

	// Group by string length using integer keys
	result := GroupBy2(input, func(s string) int {
		return len(s)
	})

	assert.Len(t, result, 3)
	assert.Equal(t, []string{"a", "e"}, result[1])
	assert.Equal(t, []string{"bb", "dd"}, result[2])
	assert.Equal(t, []string{"ccc"}, result[3])
}

func TestIndex(t *testing.T) {
	type TestItem struct {
		ID   string
		Name string
	}

	input := []TestItem{
		{ID: "1", Name: "First"},
		{ID: "2", Name: "Second"},
		{ID: "3", Name: "Third"},
	}

	result := Index(input, func(item TestItem) string {
		return item.ID
	})

	assert.Len(t, result, 3)
	assert.Equal(t, "First", result["1"].Name)
	assert.Equal(t, "Second", result["2"].Name)
	assert.Equal(t, "Third", result["3"].Name)
}

func TestFormatDateTime(t *testing.T) {
	// Test with a known time
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	result := FormatDateTime(testTime)

	// Should format as expected (check that it contains the basic components)
	assert.Contains(t, result, "2023")
	assert.Contains(t, result, "12")
	assert.Contains(t, result, "25")
	assert.NotEmpty(t, result)
}

func TestTake(t *testing.T) {
	input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Test taking first 3 elements
	result := Take(3, input)
	assert.Equal(t, []int{1, 2, 3}, result)

	// Test taking more than available
	result = Take(15, input)
	assert.Equal(t, input, result)

	// Test taking 0 elements
	result = Take(0, input)
	assert.Empty(t, result)

	// Test taking negative number (should return empty)
	result = Take(-5, input)
	assert.Empty(t, result)

	// Test with empty slice
	result = Take(5, []int{})
	assert.Empty(t, result)
}

func TestPanicOnErr(t *testing.T) {
	// Test with nil error (should not panic)
	assert.NotPanics(t, func() {
		PanicOnErr(nil)
	})

	// Test with actual error (should panic)
	assert.Panics(t, func() {
		PanicOnErr(fmt.Errorf("test error"))
	})
}

func TestPanicBadType(t *testing.T) {
	// Test that PanicBadType actually panics
	assert.Panics(t, func() {
		PanicBadType("string", "integer")
	})

	assert.Panics(t, func() {
		PanicBadType(42, "string")
	})
}
