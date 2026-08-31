package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnit_Config_EnvHelpers_Boundaries validates environment helpers addEnv, deduplicateEnv, and mergeEnv
// across small (<= 16) and large (> 16) boundaries and with/without duplicates.
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

	t.Run("deduplicateEnv small list <= 16 with duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"A=9", "I=10", "B=11", "J=12", "K=13", "L=14", "M=15", "N=16",
		}
		expected := []string{"A=9", "B=11", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=10", "J=12", "K=13", "L=14", "M=15", "N=16"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv small list <= 16 without duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"I=9", "J=10", "K=11", "L=12", "M=13", "N=14", "O=15", "P=16",
		}
		res := deduplicateEnv(input)
		// Should return the exact same slice (same reference) to avoid allocation
		assert.Equal(t, input, res)
		assert.Same(t, &input[0], &res[0])
	})

	t.Run("deduplicateEnv large list > 16 with duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"I=9", "J=10", "K=11", "L=12", "M=13", "N=14", "O=15", "P=16",
			"A=17", "Q=18", "B=19",
		}
		expected := []string{"A=17", "B=19", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=9", "J=10", "K=11", "L=12", "M=13", "N=14", "O=15", "P=16", "Q=18"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv large list > 16 without duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"I=9", "J=10", "K=11", "L=12", "M=13", "N=14", "O=15", "P=16",
			"Q=17", "R=18",
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

	t.Run("mergeEnv small list <= 16 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3", "D=4"}
		p2 := []string{"E=5", "F=6", "A=7", "G=8"}
		p1 := []string{"H=9", "I=10", "B=11", "J=12"}
		// Total strings = 12 (<= 16)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=7", "B=11", "C=3", "D=4", "E=5", "F=6", "G=8", "H=9", "I=10", "J=12"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv large list > 16 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6"}
		p2 := []string{"G=7", "H=8", "I=9", "J=10", "K=11", "L=12"}
		p1 := []string{"M=13", "N=14", "O=15", "A=16", "D=17", "P=18"}
		// Total strings = 18 (> 16)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=16", "B=2", "C=3", "D=17", "E=5", "F=6", "G=7", "H=8", "I=9", "J=10", "K=11", "L=12", "M=13", "N=14", "O=15", "P=18"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv exact cutover limit 16 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6"}
		p2 := []string{"G=7", "H=8", "I=9", "J=10", "K=11"}
		p1 := []string{"A=12", "D=13", "L=14", "M=15", "N=16"}
		// Total strings = 16 (<= 16)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=12", "B=2", "C=3", "D=13", "E=5", "F=6", "G=7", "H=8", "I=9", "J=10", "K=11", "L=14", "M=15", "N=16"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv exact cutover limit 17 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6"}
		p2 := []string{"G=7", "H=8", "I=9", "J=10", "K=11", "L=12"}
		p1 := []string{"A=13", "D=14", "M=15", "N=16", "O=17"}
		// Total strings = 17 (> 16)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=13", "B=2", "C=3", "D=14", "E=5", "F=6", "G=7", "H=8", "I=9", "J=10", "K=11", "L=12", "M=15", "N=16", "O=17"}
		assert.Equal(t, expected, res)
	})
}
