package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: Sticky Error Pattern
// Once the expression resolver encounters an error, it is retained,
// and subsequent resolutions immediately abort, preserving the first error.
func TestUnit_Config_Resolver_StickyError(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VALID":   "ok",
			"INVALID": "",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// 1. Trigger the first error
	_ = r.resolveString("{{NOT_EXIST_MAGIC_WORD}}")
	firstErr := r.Error()
	require.Error(t, firstErr)
	assert.Contains(t, firstErr.Error(), "unknown directive or magic word")

	// 2. Perform a successful operation on a fresh string
	res := r.resolveString("{{env:VALID}}")
	// The operation must return the input or unchanged string, or simply abort, and the resolver must still have the first error
	assert.Equal(t, "{{env:VALID}}", res)
	assert.Equal(t, firstErr, r.Error())

	// 3. Trigger a different error and verify it didn't overwrite the first error
	_ = r.resolveString("{{unknown_directive:param}}")
	assert.Equal(t, firstErr, r.Error())
}

// docs/features/value-resolution.md: Recursive Value Resolution
// Value resolution must handle deeply nested mixed structures (slices of maps of slices of maps) correctly.
func TestUnit_Config_Resolver_DeepRecursiveResolve(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VAR1": "val1",
			"VAR2": "val2",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	deepInput := []any{
		"{{HOME}}",
		map[string]any{
			"key1": "{{PWD}}",
			"key2": []any{
				"{{env:VAR1}}",
				map[string]any{
					"nested_key": "{{env:VAR2}}",
				},
				42,
			},
		},
	}

	resolved := r.Resolve(deepInput)
	require.NoError(t, r.Error())

	expected := []any{
		"/home/user",
		map[string]any{
			"key1": "/work",
			"key2": []any{
				"val1",
				map[string]any{
					"nested_key": "val2",
				},
				42,
			},
		},
	}
	assert.Equal(t, expected, resolved)
}

// docs/features/value-resolution.md: Double-Brace Escaping
// Verify that double-brace escaping completely preserves expressions literally,
// even inside deeply nested collections, without triggering "unknown directive or magic word" failures.
func TestUnit_Config_Resolver_DoubleBraceEscaping_DeepNested(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	input := map[string]any{
		"escaped_magic":     "{{ {{MY_MAGIC_WORD}} }}",
		"escaped_directive": "{{ {{my:directive:param}} }}",
		"nested_list": []any{
			"{{ {{ANOTHER_ONE}} }}",
			"normal_text",
		},
	}

	resolved := r.Resolve(input)
	require.NoError(t, r.Error())

	expected := map[string]any{
		"escaped_magic":     "{{MY_MAGIC_WORD}}",
		"escaped_directive": "{{my:directive:param}}",
		"nested_list": []any{
			"{{ANOTHER_ONE}}",
			"normal_text",
		},
	}
	assert.Equal(t, expected, resolved)
}

// docs/features/value-resolution.md: Multiple Anchors Evaluation
// A path containing multiple anchors must satisfy boundary verification checks for every active anchor.
func TestUnit_Config_Resolver_MultipleAnchorsBoundaryValidation_Complex(t *testing.T) {
	t.Parallel()

	pwd := "/work"
	home := "/home/user"

	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
		Env: map[string]string{
			"PATH_SUFFIX": "/subdir",
		},
	}

	t.Run("consecutive active anchors with boundary traversal", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Evaluates to "/home/user/subdir/../../work" -> "/work" which escapes the boundaries.
		_, err = ResolvePath("{{HOME}}{{env:PATH_SUFFIX}}/../../work", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("empty environment variable anchor with trailing parent traversal", func(t *testing.T) {
		mfsWithEnv := &MockFileSystem{
			WD:      pwd,
			HomeDir: home,
			Env: map[string]string{
				"EMPTY_ANCHOR": "",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfsWithEnv)
		require.NoError(t, err)

		// Anchors resolving to empty strings should immediately fail boundary checks
		_, err = ResolvePath("{{env:EMPTY_ANCHOR}}/../etc", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})
}

// docs/features/value-resolution.md: Unrecognized and Unknown Expressions
// Literal preservation of lowercase, camelCase, spacing, and symbols inside expressions,
// while verifying that all uppercase and colon-containing forms are strictly rejected as unknown.
func TestUnit_Config_Resolver_StrictVsLiteral_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	cases := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{"lowercase simple", "{{myvar}}", false},
		{"camelCase simple", "{{myCamelCaseVar}}", false},
		{"spacing simple", "{{some template var}}", false},
		{"symbols allowed", "{{my-var-123_abc}}", false},
		{"all uppercase fails", "{{MYVAR}}", true},
		{"all uppercase with underscores fails", "{{MY_VAR_123}}", true},
		{"colon fails", "{{my:var}}", true},
		{"multiple colons fails", "{{my:var:val}}", true},
		{"leading/trailing spaces in uppercase fails", "{{  MYVAR  }}", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := NewExpressionResolverWithFS(nil, mfs)
			require.NoError(t, err)

			res := r.resolveString(tc.input)
			if tc.expectErr {
				require.Error(t, r.Error())
				assert.Contains(t, r.Error().Error(), "unknown directive or magic word")
			} else {
				require.NoError(t, r.Error())
				assert.Equal(t, tc.input, res)
			}
		})
	}
}

// docs/features/value-resolution.md: File Directive Validation
// The file parameter of the file directive must be strictly validated.
func TestUnit_Config_Resolver_FileDirective_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("null byte in file name rejected", func(t *testing.T) {
		_, err := r.resolveFile("file\x00name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("control character in file name rejected", func(t *testing.T) {
		_, err := r.resolveFile("file\x01name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("non-local relative path rejected", func(t *testing.T) {
		_, err := r.resolveFile("../outside/config.yaml")
		require.Error(t, err)
	})
}

// docs/features/value-resolution.md: FindDir Directive Validation
// The folder parameter of the find_dir directive must be strictly validated.
func TestUnit_Config_Resolver_FindDirDirective_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("null byte in folder name rejected", func(t *testing.T) {
		_, err := r.resolveFindDir("folder\x00name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("control character in folder name rejected", func(t *testing.T) {
		_, err := r.resolveFindDir("folder\x07name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})
}

// docs/features/value-resolution.md: Env Directive Validation
// Environment variable keys are strictly validated.
func TestUnit_Config_Resolver_EnvDirective_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VAR_NAME": "value",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("null byte in env key rejected", func(t *testing.T) {
		_, err := r.resolveEnv("VAR\x00NAME")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("control character in env key rejected", func(t *testing.T) {
		_, err := r.resolveEnv("VAR\nNAME")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("invalid characters in env key rejected", func(t *testing.T) {
		_, err := r.resolveEnv("VAR-NAME")
		require.Error(t, err)
	})

	t.Run("empty env key rejected", func(t *testing.T) {
		_, err := r.resolveEnv("")
		require.Error(t, err)
	})

	t.Run("env with default value works", func(t *testing.T) {
		val, err := r.resolveEnv("NOT_EXIST_KEY:-fallback_val")
		require.NoError(t, err)
		assert.Equal(t, "fallback_val", val)
	})
}

// docs/features/value-resolution.md: Simulated fs.Abs Failure
// If fs.Abs fails under non-zero host context level (Level > 0), the resolver must handle it gracefully.
func TestUnit_Config_Resolver_AbsFailure_LevelGtZero(t *testing.T) {
	t.Parallel()

	pwd := "/work"
	home := "/home/user"

	errAbs := errors.New("simulated abs error")
	mfs := &julesAbsErrMockFS{
		errAbs: errAbs,
		MockFileSystem: MockFileSystem{
			WD:      pwd,
			HomeDir: home,
		},
	}

	hostCtx := &HostContext{
		Level: 1,
		Mounts: []MountMapping{
			{Source: "/host/dir", Target: "/work", Level: 1},
		},
	}

	r, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	// Since abs fails, applyReverseResolution should return an error
	_, err = r.applyReverseResolution("sub")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated abs error")
}

type julesAbsErrMockFS struct {
	MockFileSystem
	errAbs error
}

func (m *julesAbsErrMockFS) Abs(path string) (string, error) {
	if m.errAbs != nil {
		return "", m.errAbs
	}
	return m.MockFileSystem.Abs(path)
}
