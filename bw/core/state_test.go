package core

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	state := NewState()

	// Test Root is properly initialized
	assert.NotNil(t, state.Root)
	assert.Equal(t, NS{}, state.Root.NS)
	assert.NotNil(t, state.Root.Item)

	// Root.Item should be an empty []Result
	results, ok := state.Root.Item.([]Result)
	assert.True(t, ok, "Root.Item should be []Result")
	assert.Empty(t, results)

	// Test other fields are initialized
	assert.NotNil(t, state.index)
	assert.Empty(t, state.index)
	assert.NotNil(t, state.KeyVals)
	assert.Empty(t, state.KeyVals)
	assert.NotNil(t, state.ListenerList)
	assert.Empty(t, state.ListenerList)
}

func TestStateGetResults(t *testing.T) {
	state := NewState()

	// Initially empty
	results := state.GetResults()
	assert.Empty(t, results)

	// Add some results
	testResults := []Result{
		{ID: "test1", NS: MakeNS("test", "ns", "type")},
		{ID: "test2", NS: MakeNS("test", "ns", "type")},
	}
	state.SetRoot(testResults)

	results = state.GetResults()
	assert.Len(t, results, 2)
	assert.Equal(t, "test1", results[0].ID)
	assert.Equal(t, "test2", results[1].ID)
}

func TestStateSetRoot(t *testing.T) {
	state := NewState()

	testResults := []Result{
		{ID: "item1", NS: MakeNS("test", "ns", "item")},
		{ID: "item2", NS: MakeNS("test", "ns", "item")},
	}

	state.SetRoot(testResults)

	retrievedResults := state.GetResults()
	assert.Len(t, retrievedResults, 2)
	assert.Equal(t, testResults, retrievedResults)
}

func TestStateGetResult(t *testing.T) {
	state := NewState()

	// Test with empty state
	_, err := state.GetResult("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "result with id not present")

	// Set up index and results
	testResults := []Result{
		{ID: "test1", NS: MakeNS("test", "ns", "type")},
		{ID: "test2", NS: MakeNS("test", "ns", "type")},
	}
	state.SetRoot(testResults)
	state.index = map[string]int{
		"test1": 0,
		"test2": 1,
	}

	// Test successful retrieval
	result, err := state.GetResult("test1")
	assert.NoError(t, err)
	assert.Equal(t, "test1", result.ID)

	result, err = state.GetResult("test2")
	assert.NoError(t, err)
	assert.Equal(t, "test2", result.ID)

	// Test non-existent ID
	_, err = state.GetResult("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "result with id not present")
}

func TestStateGetIndex(t *testing.T) {
	state := NewState()

	// Initially empty
	index := state.GetIndex()
	assert.Empty(t, index)

	// Add some index entries
	state.index["test1"] = 0
	state.index["test2"] = 1

	index = state.GetIndex()
	assert.Len(t, index, 2)
	assert.Equal(t, 0, index["test1"])
	assert.Equal(t, 1, index["test2"])
}

func TestStateAddListener(t *testing.T) {
	state := NewState()

	// Initially empty
	assert.Empty(t, state.ListenerList)

	// Add a listener
	listener1 := Listener{
		ID: "test-listener-1",
		ReducerFn: func(r Result) bool { return true },
		CallbackFn: func(old, new []Result) {},
	}

	state.AddListener(listener1)
	assert.Len(t, state.ListenerList, 1)
	assert.Equal(t, "test-listener-1", state.ListenerList[0].ID)

	// Add another listener
	listener2 := Listener{
		ID: "test-listener-2",
		ReducerFn: func(r Result) bool { return false },
		CallbackFn: func(old, new []Result) {},
	}

	state.AddListener(listener2)
	assert.Len(t, state.ListenerList, 2)
	assert.Equal(t, "test-listener-1", state.ListenerList[0].ID)
	assert.Equal(t, "test-listener-2", state.ListenerList[1].ID)
}

func TestStateGetKeyVal(t *testing.T) {
	state := NewState()

	// Test non-existent key
	assert.Equal(t, "", state.GetKeyVal("nonexistent"))

	// Test with string value
	state.KeyVals["string_key"] = "test_value"
	assert.Equal(t, "test_value", state.GetKeyVal("string_key"))

	// Test with non-string value (should return empty string)
	state.KeyVals["int_key"] = 42
	assert.Equal(t, "", state.GetKeyVal("int_key"))

	state.KeyVals["bool_key"] = true
	assert.Equal(t, "", state.GetKeyVal("bool_key"))

	// Test with nil value
	state.KeyVals["nil_key"] = nil
	assert.Equal(t, "", state.GetKeyVal("nil_key"))
}

func TestStateGetKeyAnyVal(t *testing.T) {
	state := NewState()

	// Test non-existent key
	assert.Nil(t, state.GetKeyAnyVal("nonexistent"))

	// Test with various value types
	state.KeyVals["string_key"] = "test_value"
	assert.Equal(t, "test_value", state.GetKeyAnyVal("string_key"))

	state.KeyVals["int_key"] = 42
	assert.Equal(t, 42, state.GetKeyAnyVal("int_key"))

	state.KeyVals["bool_key"] = true
	assert.Equal(t, true, state.GetKeyAnyVal("bool_key"))

	state.KeyVals["slice_key"] = []string{"a", "b", "c"}
	assert.Equal(t, []string{"a", "b", "c"}, state.GetKeyAnyVal("slice_key"))

	// Test with nil value
	state.KeyVals["nil_key"] = nil
	assert.Nil(t, state.GetKeyAnyVal("nil_key"))
}

func TestStateSetKeyAnyVal(t *testing.T) {
	state := NewState()

	// Test setting various types
	state.SetKeyAnyVal("string_key", "test_value")
	assert.Equal(t, "test_value", state.KeyVals["string_key"])

	state.SetKeyAnyVal("int_key", 42)
	assert.Equal(t, 42, state.KeyVals["int_key"])

	state.SetKeyAnyVal("bool_key", true)
	assert.Equal(t, true, state.KeyVals["bool_key"])

	state.SetKeyAnyVal("nil_key", nil)
	assert.Nil(t, state.KeyVals["nil_key"])

	// Test overwriting values
	state.SetKeyAnyVal("string_key", "new_value")
	assert.Equal(t, "new_value", state.KeyVals["string_key"])
}

func TestStateSomeKeyVals(t *testing.T) {
	state := NewState()

	// Set up test data with mixed types
	state.KeyVals["app.name"] = "test_app"
	state.KeyVals["app.version"] = "1.0.0"
	state.KeyVals["app.debug"] = true // Not a string
	state.KeyVals["user.name"] = "john_doe"
	state.KeyVals["user.age"] = 30 // Not a string
	state.KeyVals["config.timeout"] = "30s"
	state.KeyVals["other.key"] = "other_value"

	// Test "app." prefix
	appKeys := state.SomeKeyVals("app.")
	expected := map[string]string{
		"app.name":    "test_app",
		"app.version": "1.0.0",
		// app.debug should be excluded (not a string)
	}
	assert.Equal(t, expected, appKeys)

	// Test "user." prefix
	userKeys := state.SomeKeyVals("user.")
	expected = map[string]string{
		"user.name": "john_doe",
		// user.age should be excluded (not a string)
	}
	assert.Equal(t, expected, userKeys)

	// Test non-existent prefix
	noKeys := state.SomeKeyVals("nonexistent.")
	assert.Empty(t, noKeys)

	// Test empty prefix (should return all string values)
	allStringKeys := state.SomeKeyVals("")
	expected = map[string]string{
		"app.name":       "test_app",
		"app.version":    "1.0.0",
		"user.name":      "john_doe",
		"config.timeout": "30s",
		"other.key":      "other_value",
	}
	assert.Equal(t, expected, allStringKeys)
}

func TestStateSomeKeyAnyVals(t *testing.T) {
	state := NewState()

	// Set up test data with mixed types
	state.KeyVals["app.name"] = "test_app"
	state.KeyVals["app.version"] = "1.0.0"
	state.KeyVals["app.debug"] = true
	state.KeyVals["user.name"] = "john_doe"
	state.KeyVals["user.age"] = 30
	state.KeyVals["config.timeout"] = "30s"
	state.KeyVals["other.key"] = "other_value"

	// Test "app." prefix (should include all types)
	appKeys := state.SomeKeyAnyVals("app.")
	expected := map[string]any{
		"app.name":    "test_app",
		"app.version": "1.0.0",
		"app.debug":   true,
	}
	assert.Equal(t, expected, appKeys)

	// Test "user." prefix
	userKeys := state.SomeKeyAnyVals("user.")
	expected = map[string]any{
		"user.name": "john_doe",
		"user.age":  30,
	}
	assert.Equal(t, expected, userKeys)

	// Test non-existent prefix
	noKeys := state.SomeKeyAnyVals("nonexistent.")
	assert.Empty(t, noKeys)

	// Test empty prefix (should return all values)
	allKeys := state.SomeKeyAnyVals("")
	expected = map[string]any{
		"app.name":       "test_app",
		"app.version":    "1.0.0",
		"app.debug":      true,
		"user.name":      "john_doe",
		"user.age":       30,
		"config.timeout": "30s",
		"other.key":      "other_value",
	}
	assert.Equal(t, expected, allKeys)
}

func TestStateIntegration(t *testing.T) {
	// Test that all state operations work together
	state := NewState()

	// Set up some data
	testResults := []Result{
		{
			ID:   "result1",
			NS:   MakeNS("test", "integration", "result"),
			Item: "test_data_1",
			Tags: mapset.NewSet[Tag](),
		},
		{
			ID:   "result2",
			NS:   MakeNS("test", "integration", "result"),
			Item: "test_data_2",
			Tags: mapset.NewSet[Tag](),
		},
	}

	state.SetRoot(testResults)
	state.index = map[string]int{"result1": 0, "result2": 1}

	// Set up some key-value pairs
	state.SetKeyAnyVal("test.setting1", "value1")
	state.SetKeyAnyVal("test.setting2", "value2")
	state.SetKeyAnyVal("other.setting", "other_value")

	// Add a listener
	listener := Listener{
		ID: "integration-test-listener",
		ReducerFn: func(r Result) bool {
			return r.NS.Major == "test"
		},
		CallbackFn: func(old, new []Result) {
			// Callback for testing - implementation not needed for this test
		},
	}
	state.AddListener(listener)

	// Verify everything works
	assert.Len(t, state.GetResults(), 2)

	result, err := state.GetResult("result1")
	assert.NoError(t, err)
	assert.Equal(t, "result1", result.ID)

	testSettings := state.SomeKeyVals("test.")
	assert.Len(t, testSettings, 2)
	assert.Equal(t, "value1", testSettings["test.setting1"])

	assert.Len(t, state.ListenerList, 1)
	assert.Equal(t, "integration-test-listener", state.ListenerList[0].ID)
}