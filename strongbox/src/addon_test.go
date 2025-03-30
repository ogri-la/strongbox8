package strongbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_load_installed_addon__empty_dir(t *testing.T) {
	empty_addon_dir := t.TempDir()
	_, err := load_installed_addon(empty_addon_dir)
	assert.NotNil(t, err)
}
