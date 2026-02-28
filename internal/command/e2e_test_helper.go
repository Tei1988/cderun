//go:build e2e

package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// runCderunE2E is a helper for E2E tests that ensures proper separation
// between cderun flags and the target command using the "--" separator.
func runCderunE2E(cderunFlags []string, targetCommand []string) (stdout, stderr string, exitCode int, err error) {
	args := append([]string{}, cderunFlags...)
	if len(targetCommand) > 0 {
		args = append(args, "--")
		args = append(args, targetCommand...)
	}
	return runCderun(args...)
}

// runCderunWithStdinE2E is a helper for E2E tests with stdin and proper argument separation.
func runCderunWithStdinE2E(stdin io.Reader, cderunFlags []string, targetCommand []string) (stdout, stderr string, exitCode int, err error) {
	args := append([]string{}, cderunFlags...)
	if len(targetCommand) > 0 {
		args = append(args, "--")
		args = append(args, targetCommand...)
	}
	return runCderunWithStdin(stdin, args...)
}

// findCderunBinary searches for the cderun binary in the project structure.
// It looks for "cderun" in the current directory and its ancestors.
func findCderunBinary() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	curr := wd
	for {
		path := filepath.Join(curr, "cderun")
		if _, err := os.Stat(path); err == nil {
			return filepath.Abs(path)
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return "", fmt.Errorf("cderun binary not found in %s or its ancestors", wd)
}
