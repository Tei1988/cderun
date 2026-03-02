package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of cderun.
	Version = "dev"
	// Revision is the git commit SHA.
	Revision = "unknown"
	// BuildDate is the date when the binary was built.
	BuildDate = "unknown"
)

// Info returns a formatted string containing version information.
func Info() string {
	return fmt.Sprintf("cderun version %s (rev: %s, built at: %s, %s/%s)", Version, Revision, BuildDate, runtime.GOOS, runtime.GOARCH)
}
