package ui

import (
	"bw/core"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKVCliNoColorConstant(t *testing.T) {
	assert.Equal(t, "bw.cli.NO_COLOR", KV_CLI_NO_COLOR)
}

func TestFullyQualifiedFnName(t *testing.T) {
	// Test service without ServiceGroup
	service := core.Service{
		Label: "test-function",
	}

	result := FullyQualifiedFnName(service)
	assert.Equal(t, "test-function", result)

	// Test service with ServiceGroup
	serviceGroup := &core.ServiceGroup{
		NS: core.NS{
			Major: "github",
			Minor: "repos",
			Type:  "service",
		},
	}

	service = core.Service{
		Label:        "list-issues",
		ServiceGroup: serviceGroup,
	}

	result = FullyQualifiedFnName(service)
	assert.Equal(t, "github/repos/list-issues", result)

	// Test with different namespace
	serviceGroup = &core.ServiceGroup{
		NS: core.NS{
			Major: "os",
			Minor: "fs",
			Type:  "service",
		},
	}

	service = core.Service{
		Label:        "list-files",
		ServiceGroup: serviceGroup,
	}

	result = FullyQualifiedFnName(service)
	assert.Equal(t, "os/fs/list-files", result)
}

func TestPickIdxEdgeCases(t *testing.T) {
	// Test with no items
	_, err := pick_idx(0)
	assert.Error(t, err)
	assert.Equal(t, "no items to choose from", err.Error())
}

func TestPickKeyEdgeCases(t *testing.T) {
	// Test with empty menu - this will require input but we can test the logic
	// For now, just test the menu structure requirements
	menu := [][]string{}

	// We can't easily test this without mocking stdin, but we can test it indirectly
	// by ensuring the function exists and can be called (it will fail when trying to read input)
	_, err := pick_key(menu)
	assert.Error(t, err) // Should error when trying to read input or when no match found
}

func TestValidateIdxInput(t *testing.T) {
	tests := []struct {
		input       string
		numItems    int
		expectedIdx int
		expectError bool
		errorMsg    string
	}{
		{"1", 3, 0, false, ""},                                         // Valid input "1" should return index 0
		{"3", 3, 2, false, ""},                                         // Valid input "3" should return index 2
		{"0", 3, 0, true, "idx out of range: 1-3"},                     // Invalid: too small
		{"4", 3, 0, true, "idx out of range: 1-3"},                     // Invalid: too large
		{"", 3, 0, true, "no selection made"},                          // Empty input
		{"  2  ", 3, 1, false, ""},                                     // Should trim whitespace
		{"abc", 3, 0, true, "failed to convert selection to an index"}, // Non-numeric
		{"2.5", 3, 0, true, "failed to convert selection to an index"}, // Decimal
		{"1", 0, 0, true, "no items to choose from"},                   // No items available
	}

	for _, test := range tests {
		t.Run("input_"+test.input+"_items_"+fmt.Sprint(test.numItems), func(t *testing.T) {
			result, err := validate_idx_input(test.input, test.numItems)

			if test.expectError {
				assert.Error(t, err)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedIdx, result)
			}
		})
	}
}

func TestValidateKeyInput(t *testing.T) {
	menu := [][]string{
		{"a", "Option A"},
		{"b", "Option B"},
		{"quit", "Quit"},
	}

	tests := []struct {
		input       string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{"a", "a", false, ""},
		{"b", "b", false, ""},
		{"quit", "quit", false, ""},
		{"A", "a", false, ""},                     // Should be case insensitive
		{"B", "b", false, ""},                     // Should be case insensitive
		{"QUIT", "quit", false, ""},               // Should be case insensitive
		{"  a  ", "a", false, ""},                 // Should trim whitespace
		{"x", "", true, "unknown option 'x'"},     // Invalid option
		{"xyz", "", true, "unknown option 'xyz'"}, // Invalid option
		{"", "", true, "unknown option ''"},       // Empty input
	}

	for _, test := range tests {
		t.Run("input_"+test.input, func(t *testing.T) {
			result, err := validate_key_input(test.input, menu)

			if test.expectError {
				assert.Error(t, err)
				if test.errorMsg != "" {
					assert.Contains(t, err.Error(), test.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestValidateKeyInputEmptyMenu(t *testing.T) {
	menu := [][]string{}

	result, err := validate_key_input("anything", menu)
	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "unknown option 'anything'")
}
