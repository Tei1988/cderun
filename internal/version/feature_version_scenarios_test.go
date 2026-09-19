package version

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Version_InfoScenarios(t *testing.T) {
	// Backup original global variables
	origVersion := Version
	origRevision := Revision
	origBuildDate := BuildDate

	t.Cleanup(func() {
		Version = origVersion
		Revision = origRevision
		BuildDate = origBuildDate
	})

	testCases := []struct {
		name      string
		ver       string
		rev       string
		buildDate string
	}{
		{
			name:      "Default development values",
			ver:       "dev",
			rev:       "unknown",
			buildDate: "unknown",
		},
		{
			name:      "Semantic version with prerelease tag",
			ver:       "v1.2.3-beta.1",
			rev:       "a1b2c3d4e5f67890",
			buildDate: "2026-03-20T12:00:00Z",
		},
		{
			name:      "Empty metadata string variables",
			ver:       "",
			rev:       "",
			buildDate: "",
		},
		{
			name:      "Custom vendor build tags",
			ver:       "2.0.0-custom+build123",
			rev:       "7890123",
			buildDate: "2026-01-01",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.ver
			Revision = tc.rev
			BuildDate = tc.buildDate

			info := Info()
			expected := fmt.Sprintf("cderun version %s (rev: %s, built at: %s, %s/%s)",
				tc.ver, tc.rev, tc.buildDate, runtime.GOOS, runtime.GOARCH)

			assert.Equal(t, expected, info)
		})
	}
}
