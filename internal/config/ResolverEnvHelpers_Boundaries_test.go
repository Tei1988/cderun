package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnit_Config_EnvHelpers_Boundaries validates environment helpers addEnv, deduplicateEnv, and mergeEnv
// across small (<= 8) and large (> 8) boundaries and with/without duplicates.
func TestUnit_Config_EnvHelpers_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("addEnv handles fresh and duplicate keys", func(t *testing.T) {
		m := map[string]string{
			"A": "A=1",
		}
		keys := []string{"A"}

		addEnv(m, &keys, []string{"A=2", "B=3"})
		assert.Equal(t, "A=2", m["A"])
		assert.Equal(t, "B=3", m["B"])
		assert.Equal(t, []string{"A", "B"}, keys)
	})

	t.Run("deduplicateEnv length <= 1", func(t *testing.T) {
		assert.Nil(t, deduplicateEnv(nil))
		assert.Equal(t, []string{"A=1"}, deduplicateEnv([]string{"A=1"}))
	})

	t.Run("deduplicateEnv small list <= 8 with duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "A=3", "C=4", "B=5"}
		expected := []string{"A=3", "B=5", "C=4"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv small list <= 8 without duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "C=3", "D=4"}
		res := deduplicateEnv(input)
		// Should return the exact same slice (same reference) to avoid allocation
		assert.Equal(t, input, res)
	})

	t.Run("deduplicateEnv large list > 8 with duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"A=9", "I=10", "B=11",
		}
		expected := []string{"A=9", "B=11", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=10"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv large list > 8 without duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"I=9", "J=10",
		}
		res := deduplicateEnv(input)
		assert.Equal(t, input, res)
	})

	t.Run("mergeEnv empty arguments", func(t *testing.T) {
		assert.Nil(t, mergeEnv(nil, nil, nil))
	})

	t.Run("mergeEnv optimizations: only one slice is populated", func(t *testing.T) {
		base := []string{"A=1", "A=2"}
		assert.Equal(t, []string{"A=2"}, mergeEnv(base, nil, nil))
		assert.Equal(t, []string{"A=2"}, mergeEnv(nil, base, nil))
		assert.Equal(t, []string{"A=2"}, mergeEnv(nil, nil, base))
	})

	t.Run("mergeEnv small list <= 8 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2"}
		p2 := []string{"C=3", "A=4"}
		p1 := []string{"D=5", "B=6"}
		// Total strings = 6 (<= 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=4", "B=6", "C=3", "D=5"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv large list > 8 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "F=6"}
		p1 := []string{"G=7", "H=8", "A=9", "D=10"}
		// Total strings = 10 (> 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=9", "B=2", "C=3", "D=10", "E=5", "F=6", "G=7", "H=8"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv exact cutover limit 8 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "F=6"}
		p1 := []string{"A=7", "D=8"}
		// Total strings = 8 (<= 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=7", "B=2", "C=3", "D=8", "E=5", "F=6"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv exact cutover limit 9 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "F=6"}
		p1 := []string{"A=7", "D=8", "G=9"}
		// Total strings = 9 (> 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=7", "B=2", "C=3", "D=8", "E=5", "F=6", "G=9"}
		assert.Equal(t, expected, res)
	})
}
