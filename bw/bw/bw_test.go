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

func TestConstants(t *testing.T) {
	// Test that our NS constants are properly formed
	assert.Equal(t, "bw/annotation/annotation", BW_NS_ANNOTATION_ANNOTATION.String())
	assert.Equal(t, "bw/core/result-list", BW_NS_RESULT_LIST.String())
	assert.Equal(t, "bw/core/error", BW_NS_ERROR.String())
	assert.Equal(t, "bw/core/state", BW_NS_STATE.String())
	assert.Equal(t, "bw/core/service", BW_NS_SERVICE.String())
	assert.Equal(t, "bw/fs/file", BW_NS_FS_FILE.String())
	assert.Equal(t, "bw/fs/dir", BW_NS_FS_DIR.String())
}
