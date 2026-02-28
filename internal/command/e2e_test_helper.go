//go:build e2e

package command

import (
	"io"
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
//
//nolint:unused
func runCderunWithStdinE2E(stdin io.Reader, cderunFlags []string, targetCommand []string) (stdout, stderr string, exitCode int, err error) {
	args := append([]string{}, cderunFlags...)
	if len(targetCommand) > 0 {
		args = append(args, "--")
		args = append(args, targetCommand...)
	}
	return runCderunWithStdin(stdin, args...)
}
