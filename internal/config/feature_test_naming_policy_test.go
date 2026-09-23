package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_TestNamingPolicyPattern(t *testing.T) {
	t.Parallel()

	// Pattern defined in scripts/check-test-names.sh
	prohibitedPattern := `(improvement|expansion|refinement|comprehensive|additional|extra|more|deep|jules)`
	re, err := regexp.Compile(prohibitedPattern)
	require.NoError(t, err, "Prohibited regex pattern must compile successfully")

	prohibitedCases := []string{
		"feature_test_improvement_test.go",
		"expansion_test.go",
		"refinement_scenarios_test.go",
		"comprehensive_coverage_test.go",
		"additional_cases_test.go",
		"extra_helper_test.go",
		"more_scenarios_test.go",
		"deep_analysis_test.go",
		"jules_feature_test.go",
		"jules_test_refinement_extra_test.go",
	}

	for _, tc := range prohibitedCases {
		t.Run("prohibited_"+tc, func(t *testing.T) {
			assert.Truef(t, re.MatchString(tc), "Filename %q must be matched as prohibited", tc)
		})
	}

	validCases := []string{
		"feature_shm_size_test.go",
		"bugfix_issue42_test.go",
		"resolver_robustness_test.go",
		"command_scenario_boundaries_test.go",
		"feature_test_naming_policy_test.go",
		"feature_oci_runtime_test.go",
		"feature_prune_option_test.go",
	}

	for _, tc := range validCases {
		t.Run("valid_"+tc, func(t *testing.T) {
			assert.Falsef(t, re.MatchString(tc), "Filename %q must NOT be matched as prohibited", tc)
		})
	}
}

func TestUnit_CheckTestNamesScriptExecution(t *testing.T) {
	// Find root dir relative to internal/config
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "Failed to resolve repository root directory")

	scriptPath := filepath.Join(repoRoot, "scripts", "check-test-names.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Skip("check-test-names.sh not found, skipping script execution test")
	}

	t.Run("CleanWorkingTree", func(t *testing.T) {
		// Run script against current repository state
		cmd := exec.Command(scriptPath)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err, "check-test-names.sh should pass on clean working tree: %s", string(out))
	})

	t.Run("ControlledTemporaryRepoCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Initialize temporary git repo
		initCmd := exec.Command("git", "init")
		initCmd.Dir = tempDir
		require.NoError(t, initCmd.Run(), "git init in tempDir must succeed")

		// Create scripts directory and copy check-test-names.sh
		scriptsDir := filepath.Join(tempDir, "scripts")
		require.NoError(t, os.MkdirAll(scriptsDir, 0755))
		tempScriptPath := filepath.Join(scriptsDir, "check-test-names.sh")

		scriptBytes, err := os.ReadFile(scriptPath)
		require.NoError(t, err, "Must read check-test-names.sh")
		require.NoError(t, os.WriteFile(tempScriptPath, scriptBytes, 0755))

		// Case 1: Prohibited test filename present
		prohibitedFile := filepath.Join(tempDir, "feature_improvement_test.go")
		require.NoError(t, os.WriteFile(prohibitedFile, []byte("package test"), 0644))

		cmdProhibited := exec.Command(tempScriptPath)
		cmdProhibited.Dir = tempDir
		outProhibited, errProhibited := cmdProhibited.CombinedOutput()
		assert.Error(t, errProhibited, "Script should return non-zero error for prohibited filename")
		assert.Contains(t, string(outProhibited), "Error: Test file naming policy violation detected")

		// Case 2: Remove prohibited file and create valid test filename
		require.NoError(t, os.Remove(prohibitedFile))
		validFile := filepath.Join(tempDir, "feature_shm_size_test.go")
		require.NoError(t, os.WriteFile(validFile, []byte("package test"), 0644))

		cmdValid := exec.Command(tempScriptPath)
		cmdValid.Dir = tempDir
		outValid, errValid := cmdValid.CombinedOutput()
		assert.NoError(t, errValid, "Script should succeed for valid filename: %s", string(outValid))
		assert.Contains(t, string(outValid), "Test file naming check passed successfully")
	})
}
