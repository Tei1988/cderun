package version

import (
	"runtime"
	"testing"
)

func TestInfo(t *testing.T) {
	// Reset to default values
	Version = "dev"
	Revision = "unknown"
	BuildDate = "unknown"

	expected := "cderun version dev (rev: unknown, built at: unknown, " + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got := Info(); got != expected {
		t.Errorf("Info() = %v, want %v", got, expected)
	}

	// Test with values
	Version = "1.0.0"
	Revision = "abc1234"
	BuildDate = "2026-03-02T12:34:56Z"

	expected = "cderun version 1.0.0 (rev: abc1234, built at: 2026-03-02T12:34:56Z, " + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got := Info(); got != expected {
		t.Errorf("Info() = %v, want %v", got, expected)
	}
}
