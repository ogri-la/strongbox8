package main

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// INFO is the default, DEBUG is opt-in, and unknown levels are refused.
func Test_parse_logging_level(t *testing.T) {
	cases := []struct {
		given    []string
		expected slog.Level
	}{
		{nil, slog.LevelInfo},
		{[]string{"--verbosity", "debug"}, slog.LevelDebug},
		{[]string{"-verbosity=warn"}, slog.LevelWarn},
		{[]string{"--verbosity", "error"}, slog.LevelError},
	}
	for _, c := range cases {
		actual, err := parse_logging_level(c.given)
		assert.NoError(t, err, c.given)
		assert.Equal(t, c.expected, actual, c.given)
	}
	for _, given := range [][]string{{"--verbosity", "fatal"}, {"--bogus"}} {
		_, err := parse_logging_level(given)
		assert.Error(t, err, given)
	}
}
