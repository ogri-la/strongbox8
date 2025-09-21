package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewForm(t *testing.T) {
	form := NewForm()

	assert.Equal(t, Service{}, form.Service) // Should be empty service
	assert.NotNil(t, form.input.ArgList)
	assert.Empty(t, form.input.ArgList)
}

func TestMakeForm(t *testing.T) {
	service := Service{
		ID:    "test-service",
		Label: "Test Service",
	}

	form := MakeForm(service)

	assert.Equal(t, service, form.Service)
	assert.NotNil(t, form.input.ArgList)
	assert.Empty(t, form.input.ArgList)
}

func TestNewFormError(t *testing.T) {
	formError := NewFormError()

	assert.Nil(t, formError.Error)
	assert.NotNil(t, formError.FieldErrorList)
	assert.Empty(t, formError.FieldErrorList)
}

func TestFormData(t *testing.T) {
	form := NewForm()

	// Test with empty form
	data := form.Data()
	assert.NotNil(t, data)
	assert.Empty(t, data)

	// Test with some data
	form.Update([]KeyVal{
		{Key: "field1", Val: "value1"},
		{Key: "field2", Val: 42},
	})

	data = form.Data()
	assert.Len(t, data, 2)
	assert.Equal(t, "value1", data["field1"])
	assert.Equal(t, 42, data["field2"])
}

func TestFormUpdate(t *testing.T) {
	form := NewForm()

	// Update with new data
	newArgs := []KeyVal{
		{Key: "test", Val: "value"},
		{Key: "number", Val: 123},
	}

	form.Update(newArgs)

	assert.Len(t, form.input.ArgList, 2)
	assert.Equal(t, "test", form.input.ArgList[0].Key)
	assert.Equal(t, "value", form.input.ArgList[0].Val)
	assert.Equal(t, "number", form.input.ArgList[1].Key)
	assert.Equal(t, 123, form.input.ArgList[1].Val)
}

func TestFormReset(t *testing.T) {
	service := Service{
		ID:    "test-service",
		Label: "Test Service",
	}

	form := MakeForm(service)

	// Add some data
	form.Update([]KeyVal{{Key: "test", Val: "value"}})
	assert.Len(t, form.input.ArgList, 1)

	// Reset the form
	form.Reset()

	// Should be back to initial state
	assert.Equal(t, service, form.Service)
	assert.Empty(t, form.input.ArgList)
}

func TestFormValidateEmpty(t *testing.T) {
	// Create a service with no arguments
	service := Service{
		ID:        "test-service",
		Interface: ServiceInterface{ArgDefList: []ArgDef{}},
	}

	form := MakeForm(service)
	formError := form.Validate()

	// Should be nil (no errors) for empty argument list
	assert.Nil(t, formError)
}

func TestFormSubmitEmpty(t *testing.T) {
	// Create a service with no arguments
	service := Service{
		ID:        "test-service",
		Interface: ServiceInterface{ArgDefList: []ArgDef{}},
	}

	form := MakeForm(service)
	args, formError := form.Submit()

	// Should succeed with empty args
	assert.Nil(t, formError)
	assert.NotNil(t, args.ArgList)
	assert.Empty(t, args.ArgList)
}
