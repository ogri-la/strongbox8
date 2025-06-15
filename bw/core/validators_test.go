package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTruthyFalsey(t *testing.T) {
	cases := []string{
		"true",
		"false",
		"yes",
		"no",
		"TRUE",
		"FALSE",
		"  TrUe  ",
		"  fAlSe  ",
	}
	for _, c := range cases {
		assert.Nil(t, IsTruthyFalsey(c))
	}

	bad_cases := []string{
		"false!",
		"maybe",
	}
	for _, c := range bad_cases {
		assert.Error(t, IsTruthyFalsey(c))
	}

}
