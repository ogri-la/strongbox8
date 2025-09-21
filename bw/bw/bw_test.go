package bw

import (
	"bw/core"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBWProvider_ID(t *testing.T) {
	provider := &BWProvider{}
	assert.Equal(t, "boardwalk", provider.ID())
}

func TestBWProvider_ServiceList(t *testing.T) {
	provider := &BWProvider{}
	services := provider.ServiceList()
	assert.NotEmpty(t, services)

	// Should have at least one service group
	assert.Greater(t, len(services), 0)

	// Check that the first service group has some services
	if len(services) > 0 {
		assert.NotEmpty(t, services[0].ServiceList)
	}
}

func TestBWProvider_ItemHandlerMap(t *testing.T) {
	provider := &BWProvider{}
	handlerMap := provider.ItemHandlerMap()
	assert.NotNil(t, handlerMap)
	// Empty map is fine for this provider
}

func TestBWProvider_Menu(t *testing.T) {
	provider := &BWProvider{}
	menu := provider.Menu()
	assert.NotNil(t, menu)
	// Empty menu is fine for this provider
}

func TestProvider(t *testing.T) {
	app := core.NewApp()
	bwProvider := Provider(app)
	assert.NotNil(t, bwProvider)
	assert.Equal(t, "boardwalk", bwProvider.ID())
}

func TestAnnotationStruct(t *testing.T) {
	annotation := Annotation{
		Annotation:  "test annotation",
		AnnotatedID: "test-id",
	}

	assert.Equal(t, "test annotation", annotation.Annotation)
	assert.Equal(t, "test-id", annotation.AnnotatedID)
}
