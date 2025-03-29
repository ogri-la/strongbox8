package strongbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsBeforeClassic(t *testing.T) {
	// note: classic was "2019-08-26T00:00:00Z"
	var cases = []struct {
		given    time.Time
		expected bool
	}{
		// one second before
		{time.Date(2019, 8, 25, 23, 59, 59, 0, time.UTC), true},
		// exactly the same is not _before_
		{WOWClassicReleaseDate(), false},
		// one second after
		{time.Date(2019, 8, 26, 0, 0, 1, 0, time.UTC), false},
		// much later
		{time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC), false},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, IsBeforeClassic(c.given), i)
	}
}
