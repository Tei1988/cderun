package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func TestFeature_SecurityHardeningValidationEnhancements(t *testing.T) {
	t.Run("ValidateAddHost empty host check", func(t *testing.T) {
		assert.NoError(t, ValidateAddHost("example.com:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("host.docker.internal:host-gateway"))

		err := ValidateAddHost(":127.0.0.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty host")
	})

	t.Run("ValidateImageName trailing and consecutive separators", func(t *testing.T) {
		validImages := []string{
			"ubuntu:22.04",
			"docker.io/library/nginx:latest",
			"myregistry.local:5000/app/service:v1.0.0",
			"alpine@sha256:e4001a1c9ed430ed6b14ed5eeb9c614b03657738f6580cf96bf1a3e83a45c36b",
		}
		for _, img := range validImages {
			assert.NoError(t, ValidateImageName(img), "image %q should be valid", img)
		}

		invalidImages := []string{
			"ubuntu/",
			"ubuntu:",
			"ubuntu@",
			"docker.io//ubuntu",
			"ubuntu::22.04",
			"ubuntu@@sha256:123",
		}
		for _, img := range invalidImages {
			assert.Error(t, ValidateImageName(img), "image %q should be invalid", img)
		}
	})

	t.Run("ValidateCpuset boundary and consecutive separator checks", func(t *testing.T) {
		validCpusets := []string{
			"0",
			"0-3",
			"0,2,4",
			"0-1,3-4",
		}
		for _, cs := range validCpusets {
			assert.NoError(t, ValidateCpuset(cs), "cpuset %q should be valid", cs)
		}

		invalidCpusets := []string{
			",0-3",
			"0-3,",
			"-0-3",
			"0-3-",
			"0,,3",
			"0--3",
			"0,-3",
		}
		for _, cs := range invalidCpusets {
			assert.Error(t, ValidateCpuset(cs), "cpuset %q should be invalid", cs)
		}
	})

	t.Run("ValidateGPUs boundary and consecutive separator checks", func(t *testing.T) {
		validGPUs := []string{
			"all",
			"0,1",
			"device=0,1",
			"count=2",
		}
		for _, g := range validGPUs {
			assert.NoError(t, ValidateGPUs(g), "gpus %q should be valid", g)
		}

		invalidGPUs := []string{
			",all",
			"all,",
			"=all",
			"all=",
			"-all",
			"all-",
			"0,,1",
			"device==0",
			"device=-0",
		}
		for _, g := range invalidGPUs {
			assert.Error(t, ValidateGPUs(g), "gpus %q should be invalid", g)
		}
	})

	t.Run("Resolver prefetch security validation", func(t *testing.T) {
		cliValid := &CLIOptions{
			Image: strPtr("alpine:latest"),
		}
		cliValid.CderunPrefetch = strPtr("python, node")
		res, err := ResolveWithFS("python", cliValid, nil, nil, RealFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "python, node", res.Prefetch)

		cliInvalid := &CLIOptions{
			Image: strPtr("alpine:latest"),
		}
		cliInvalid.CderunPrefetch = strPtr("python, ../malicious")
		_, err = ResolveWithFS("python", cliInvalid, nil, nil, RealFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prefetch")
	})
}
