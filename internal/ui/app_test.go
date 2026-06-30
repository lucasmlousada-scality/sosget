package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{
		0:          "0B",
		512:        "512B",
		1024:       "1.0KB",
		1536:       "1.5KB",
		1048576:    "1.0MB",
		1073741824: "1.0GB",
	}
	for in, want := range cases {
		assert.Equalf(t, want, formatSize(in), "formatSize(%d)", in)
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	assert.Equal(t, "just now", formatAge(now.Add(-30*time.Second)))
	assert.Equal(t, "5m ago", formatAge(now.Add(-5*time.Minute)))
	assert.Equal(t, "3h ago", formatAge(now.Add(-3*time.Hour)))

	// Older than a day falls back to an absolute date.
	old := now.Add(-48 * time.Hour)
	assert.Equal(t, old.Format("2006-01-02 15:04"), formatAge(old))
}
