package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirArgDef(t *testing.T) {
	argDef := DirArgDef()

	assert.Equal(t, "dir", argDef.ID)
	assert.Equal(t, "Directory", argDef.Label)
	assert.NotNil(t, argDef.Parser)
	assert.Len(t, argDef.ValidatorList, 1)
	assert.NotNil(t, argDef.Parser)
	assert.NotNil(t, argDef.ValidatorList[0])
}

func TestConfirmYesArgDef(t *testing.T) {
	argDef := ConfirmYesArgDef()

	assert.Equal(t, "confirm", argDef.ID)
	assert.Equal(t, "Are you sure?", argDef.Label)
	assert.Equal(t, "yes", argDef.Default)
	assert.NotNil(t, argDef.Parser)
	assert.Empty(t, argDef.ValidatorList)
}
