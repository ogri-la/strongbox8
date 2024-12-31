package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBunchEmpty(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{}
	expected := [][]string{}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunchAllUnique(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "a", "aa"}
	expected := [][]string{
		{"aaa"},
		{"a"},
		{"aa"},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunchUnbunched(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "a", "aa", "bbb", "b", "bb"}
	expected := [][]string{
		{"aaa"},
		{"a"},
		{"aa"},
		{"bbb"},
		{"b"},
		{"bb"},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}

func TestBunch(t *testing.T) {
	grouper := func(t string) any {
		return len(t)
	}

	things := []string{"aaa", "bbb", "a", "aa", "bb", "b", "c", "d", "ee", "ff", ""}
	expected := [][]string{
		{"aaa", "bbb"},
		{"a"},
		{"aa", "bb"},
		{"b", "c", "d"},
		{"ee", "ff"},
		{""},
	}

	actual := Bunch(things, grouper)
	assert.Equal(t, expected, actual)
}
