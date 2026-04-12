//go:build runtime

package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	diagnosisOnce sync.Once
)

func runDiagnosisOnFailure(stderr string, exitCode int, err error) {
	if exitCode == 0 && err == nil {
		return
	}

	diagnosisOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\n--- Runtime Test Failure Detected. Running Diagnosis ---\n")
		// Diagnosis mode runs without requiring a subcommand.
		diagStdout, diagStderr, diagExitCode, diagErr := runCderun("--diagnosis")
		if diagErr != nil {
			fmt.Fprintf(os.Stderr, "Diagnosis failed: %v\n", diagErr)
		} else {
			fmt.Fprintf(os.Stderr, "Diagnosis Exit Code: %d\n", diagExitCode)
			fmt.Fprintf(os.Stderr, "Diagnosis Stdout:\n%s\n", diagStdout)
			fmt.Fprintf(os.Stderr, "Diagnosis Stderr:\n%s\n", diagStderr)
		}
		fmt.Fprintf(os.Stderr, "--- End of Diagnosis ---\n\n")
	})
}

// runCderunE2E is a helper for runtime tests.
// It strictly requires a subcommand which acts as the tool name or image mapping key.
func runCderunE2E(cderunFlags []string, subCommand string, commandOptions []string) (stdout, stderr string, exitCode int, err error) {
	args := append([]string{}, cderunFlags...)
	args = append(args, subCommand)
	args = append(args, commandOptions...)
	stdout, stderr, exitCode, err = runCderun(args...)
	runDiagnosisOnFailure(stderr, exitCode, err)
	return
}

// runCderunWithStdinE2E is a helper for runtime tests with stdin.
// It strictly requires a subcommand.
func runCderunWithStdinE2E(stdin io.Reader, cderunFlags []string, subCommand string, commandOptions []string) (stdout, stderr string, exitCode int, err error) {
	args := append([]string{}, cderunFlags...)
	args = append(args, subCommand)
	args = append(args, commandOptions...)
	stdout, stderr, exitCode, err = runCderunWithStdin(stdin, args...)
	runDiagnosisOnFailure(stderr, exitCode, err)
	return
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
			// If already absolute, return directly. Otherwise call filepath.Abs.
			if filepath.IsAbs(path) {
				return path, nil
			}
			return filepath.Abs(path)
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return "", fmt.Errorf("cderun binary not found in %q or its ancestors", wd)
}

func runCderunWithStdin(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(stdin, args...)
}

// checkRuntimeResult is a helper to standardize result checking in E2E tests.
// It uses skipIfDockerBroken to handle environmental limitations reported via error or stderr.
func checkRuntimeResult(t *testing.T, stdout, stderr string, exitCode int, err error) {
	t.Helper()
	skipIfDockerBroken(t, err)
	if exitCode != 0 {
		// If the command failed but it might be due to environmental mount issues reported in stderr
		skipIfDockerBroken(t, fmt.Errorf("exit code %d: %s", exitCode, stderr))
		t.Fatalf("command failed with exit code %d: %s", exitCode, stderr)
	}
}
