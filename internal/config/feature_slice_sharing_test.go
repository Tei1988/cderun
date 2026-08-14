package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Feature_SliceSharing_Regression(t *testing.T) {
	// 1. Create an input slice with spare capacity
	input := make([]string, 1, 10)
	input[0] = "original"

	// 2. Call resolveStringSliceOptWithVals (with nil resolver so needResolution is false)
	resolved := resolveStringSliceOptWithVals(input, nil)

	// 3. Verify that the returned slice contains the same elements
	assert.Equal(t, []string{"original"}, resolved)

	// 4. Append to the resolved slice
	resolved = append(resolved, "mutated")

	// 5. Verify that appending to the resolved slice did NOT change the original input slice
	assert.Equal(t, []string{"original"}, input)
	assert.Equal(t, []string{"original", "mutated"}, resolved)
}
