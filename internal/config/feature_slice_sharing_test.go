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

	// 6. Explicitly validate the rest of the input slice's backing array capacity
	// to detect and prove there is absolutely no backing-array aliasing
	inputFullBackingSlice := input[:cap(input)]
	assert.Empty(t, inputFullBackingSlice[1], "Backing-array aliasing detected! Element 1 in input backing-array was modified.")

	// 7. Validate nilness and non-nil empty slice preservation
	var nilSlice []string
	resolvedNil := resolveStringSliceOptWithVals(nilSlice, nil)
	assert.Nil(t, resolvedNil)

	emptySlice := []string{}
	resolvedEmpty := resolveStringSliceOptWithVals(emptySlice, nil)
	assert.NotNil(t, resolvedEmpty)
	assert.Len(t, resolvedEmpty, 0)
}
