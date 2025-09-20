package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServiceResult(t *testing.T) {
	result := NewServiceResult()

	assert.Nil(t, result.Err)
	assert.NotNil(t, result.Result)
	assert.Empty(t, result.Result)
	assert.True(t, result.IsEmpty())
}

func TestServiceResultIsEmpty(t *testing.T) {
	// Empty result
	result := ServiceResult{}
	assert.True(t, result.IsEmpty())

	// Result with error only
	result = ServiceResult{Err: errors.New("test error")}
	assert.False(t, result.IsEmpty())

	// Result with results only
	result = ServiceResult{Result: []Result{{ID: "test"}}}
	assert.False(t, result.IsEmpty())

	// Result with both error and results
	result = ServiceResult{
		Err:    errors.New("test error"),
		Result: []Result{{ID: "test"}},
	}
	assert.False(t, result.IsEmpty())
}

func TestMakeServiceResult(t *testing.T) {
	// Test with no results
	result := MakeServiceResult()
	assert.Nil(t, result.Err)
	assert.Empty(t, result.Result)

	// Test with one result
	testResult := Result{ID: "test1"}
	result = MakeServiceResult(testResult)
	assert.Nil(t, result.Err)
	assert.Len(t, result.Result, 1)
	assert.Equal(t, "test1", result.Result[0].ID)

	// Test with multiple results
	testResult2 := Result{ID: "test2"}
	result = MakeServiceResult(testResult, testResult2)
	assert.Nil(t, result.Err)
	assert.Len(t, result.Result, 2)
	assert.Equal(t, "test1", result.Result[0].ID)
	assert.Equal(t, "test2", result.Result[1].ID)
}

func TestMakeServiceResultError(t *testing.T) {
	// Test with error and message
	originalErr := errors.New("original error")
	result := MakeServiceResultError(originalErr, "context message")

	assert.NotNil(t, result.Err)
	assert.Empty(t, result.Result)
	assert.Contains(t, result.Err.Error(), "context message")
	assert.Contains(t, result.Err.Error(), "original error")

	// Test with nil error and message
	result = MakeServiceResultError(nil, "just a message")
	assert.NotNil(t, result.Err)
	assert.Empty(t, result.Result)
	assert.Equal(t, "just a message", result.Err.Error())
}

func TestNewServiceFnArgs(t *testing.T) {
	args := NewServiceFnArgs()

	assert.NotNil(t, args.ArgList)
	assert.Empty(t, args.ArgList)
}

func TestMakeServiceFnArgs(t *testing.T) {
	args := MakeServiceFnArgs("testkey", "testvalue")

	assert.Len(t, args.ArgList, 1)
	assert.Equal(t, "testkey", args.ArgList[0].Key)
	assert.Equal(t, "testvalue", args.ArgList[0].Val)
}

func TestArgExclusivityConstants(t *testing.T) {
	assert.Equal(t, ArgExclusivity("exclusive"), ArgChoiceExclusive)
	assert.Equal(t, ArgExclusivity("non-exclusive"), ArgChoiceNonExclusive)
}

func TestInputWidgetConstants(t *testing.T) {
	assert.Equal(t, InputWidget("text-field"), InputWidgetTextField)
	assert.Equal(t, InputWidget("text-box"), InputWidgetTextBox)
	assert.Equal(t, InputWidget("choice-list"), InputWidgetSelection)
	assert.Equal(t, InputWidget("multi-choice-list"), InputWidgetMultiSelection)
	assert.Equal(t, InputWidget("file-picker"), InputWidgetFileSelection)
	assert.Equal(t, InputWidget("dir-picker"), InputWidgetDirSelection)
}
