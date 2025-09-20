package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentity(t *testing.T) {
	tests := []string{"hello", "world", "", "123", "special chars !@#$%"}

	for _, input := range tests {
		result, err := Identity(nil, input)
		assert.NoError(t, err)
		assert.Equal(t, input, result)
	}
}

func TestParseStringAsInt(t *testing.T) {
	tests := []struct {
		input       string
		expected    int
		expectError bool
	}{
		{"0", 0, false},
		{"42", 42, false},
		{"-1", -1, false},
		{"999", 999, false},
		{"abc", 0, true},
		{"12.34", 0, true},
		{"", 0, true},
		{"  123  ", 0, true}, // No trimming in this function
	}

	for _, test := range tests {
		result, err := ParseStringAsInt(nil, test.input)
		if test.expectError {
			assert.Error(t, err, "Expected error for input: %s", test.input)
			assert.Equal(t, 0, result)
		} else {
			assert.NoError(t, err, "Unexpected error for input: %s", test.input)
			assert.Equal(t, test.expected, result)
		}
	}
}

func TestParseTruthyFalseyAsBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"yes", true},
		{"y", true},
		{"Y", true},
		{"YES", true},
		{"true", true},
		{"t", true},
		{"T", true},
		{"TRUE", true},
		{"  yes  ", true}, // Should trim whitespace
		{"no", false},
		{"n", false},
		{"false", false},
		{"f", false},
		{"", false},
		{"maybe", false},
		{"0", false},
		{"1", false},
	}

	for _, test := range tests {
		result, err := ParseTruthyFalseyAsBool(nil, test.input)
		assert.NoError(t, err, "Unexpected error for input: %s", test.input)
		assert.Equal(t, test.expected, result, "Wrong result for input: %s", test.input)
	}
}

func TestParseStringAsPath(t *testing.T) {
	tests := []struct {
		input    string
		hasError bool
	}{
		{".", false},
		{"..", false},
		{"/tmp", false},
		{"./test", false},
		{"../test", false},
		{"/home/user/documents", false},
		{"", false}, // Empty string should work
	}

	for _, test := range tests {
		result, err := ParseStringAsPath(nil, test.input)
		if test.hasError {
			assert.Error(t, err, "Expected error for input: %s", test.input)
		} else {
			assert.NoError(t, err, "Unexpected error for input: %s", test.input)
			assert.IsType(t, "", result, "Result should be string for input: %s", test.input)
			// Result should be an absolute path
			resultStr, ok := result.(string)
			assert.True(t, ok, "Result should be string")
			if len(resultStr) > 0 {
				assert.True(t, resultStr[0] == '/', "Result should be absolute path, got: %s", resultStr)
			}
		}
	}
}

func TestParseStringStripWhitespace(t *testing.T) {
	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"hello", "hello", false},
		{"  hello  ", "hello", false},
		{"\tworld\n", "world", false},
		{"test", "test", false},
		{"", "", true},     // Empty after trimming should error
		{"   ", "", true},  // Only whitespace should error
		{"\t\n", "", true}, // Only whitespace chars should error
	}

	for _, test := range tests {
		result, err := ParseStringStripWhitespace(nil, test.input)
		if test.expectError {
			assert.Error(t, err, "Expected error for input: %q", test.input)
			assert.Nil(t, result)
		} else {
			assert.NoError(t, err, "Unexpected error for input: %q", test.input)
			assert.Equal(t, test.expected, result)
		}
	}
}

func TestParseStringAsResultID(t *testing.T) {
	// Create a mock app with some results
	app := NewApp()

	// Test with empty app (no results)
	result, err := ParseStringAsResultID(app, "nonexistent")
	assert.NoError(t, err, "Should not error even if result not found")
	// Should return an empty Result struct
	assert.NotNil(t, result)
}
