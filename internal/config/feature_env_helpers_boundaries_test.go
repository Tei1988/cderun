package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Specification reference: docs/features/argument-priority-logic.md
// Deduplication and merging adhere to the first-seen key ordering rule while updating
// the value with last-one-wins precedence across configuration layers (P1 > P2 > Base).

func TestUnit_DeduplicateEnv_Boundaries(t *testing.T) {
	t.Run("64 unique entries exercising all 64 stack-array slots without duplicates", func(t *testing.T) {
		env := make([]string, 64)
		for i := range 64 {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}

		deduped := deduplicateEnv(env)

		// Expected output matches input exactly when no duplicates are present.
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 64)
		for i := range 64 {
			expected[i] = fmt.Sprintf("VAR_%d=v1", i)
		}

		assert.Equal(t, expected, deduped)
	})

	t.Run("64-entry boundary with duplicate keys (fixed stack array path)", func(t *testing.T) {
		// 60 unique entries + 4 duplicates = 64 total items
		env := make([]string, 60)
		for i := range 60 {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}
		// Append duplicates for VAR_0, VAR_1, VAR_2, VAR_3
		env = append(env, "VAR_0=v2", "VAR_1=v2", "VAR_2=v2", "VAR_3=v2")

		deduped := deduplicateEnv(env)

		// First-seen key ordering rule: keys retain their original index position,
		// while duplicate key values are updated in-place (last-one-wins semantics).
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 60)
		for i := range 60 {
			if i < 4 {
				expected[i] = fmt.Sprintf("VAR_%d=v2", i)
			} else {
				expected[i] = fmt.Sprintf("VAR_%d=v1", i)
			}
		}

		assert.Equal(t, expected, deduped)
	})

	t.Run("65-entry boundary with duplicate keys (map fallback path)", func(t *testing.T) {
		// 60 unique entries + 5 duplicates = 65 total items
		env := make([]string, 60)
		for i := range 60 {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}
		// Append duplicates for VAR_0, VAR_1, VAR_2, VAR_3, VAR_4
		env = append(env, "VAR_0=v2", "VAR_1=v2", "VAR_2=v2", "VAR_3=v2", "VAR_4=v2")

		deduped := deduplicateEnv(env)

		// First-seen key ordering rule: keys retain their original index position,
		// while duplicate key values are updated in-place (last-one-wins semantics).
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 60)
		for i := range 60 {
			if i < 5 {
				expected[i] = fmt.Sprintf("VAR_%d=v2", i)
			} else {
				expected[i] = fmt.Sprintf("VAR_%d=v1", i)
			}
		}

		assert.Equal(t, expected, deduped)
	})
}

func TestUnit_MergeEnv_Boundaries(t *testing.T) {
	t.Run("64 unique total entries across base, p2, p1 exercising all 64 stack-array slots", func(t *testing.T) {
		base := make([]string, 20)
		for i := range 20 {
			base[i] = fmt.Sprintf("VAR_%d=base", i)
		}

		p2 := make([]string, 20)
		for i := range 20 {
			p2[i] = fmt.Sprintf("VAR_%d=p2", i+20) // VAR_20..VAR_39
		}

		p1 := make([]string, 24)
		for i := range 24 {
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+40) // VAR_40..VAR_63
		}

		merged := mergeEnv(base, p2, p1)

		// Complete 64 unique keys with layer-specific values
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 64)
		for i := range 64 {
			if i < 20 {
				expected[i] = fmt.Sprintf("VAR_%d=base", i)
			} else if i < 40 {
				expected[i] = fmt.Sprintf("VAR_%d=p2", i)
			} else {
				expected[i] = fmt.Sprintf("VAR_%d=p1", i)
			}
		}

		assert.Equal(t, expected, merged)
	})

	t.Run("64-entry total boundary across base, p2, p1 with overlapping keys (fixed stack array path)", func(t *testing.T) {
		base := make([]string, 20)
		for i := range 20 {
			base[i] = fmt.Sprintf("VAR_%d=base", i)
		}

		p2 := make([]string, 20)
		for i := range 20 {
			p2[i] = fmt.Sprintf("VAR_%d=p2", i+10) // Overlaps VAR_10..VAR_19
		}

		p1 := make([]string, 24)
		for i := range 24 {
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+20) // Overlaps VAR_20..VAR_29
		}

		merged := mergeEnv(base, p2, p1)

		// First-seen key ordering rule across layers (Base -> P2 -> P1) with last-one-wins precedence.
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 44)
		for i := range 44 {
			if i < 10 {
				expected[i] = fmt.Sprintf("VAR_%d=base", i)
			} else if i < 20 {
				expected[i] = fmt.Sprintf("VAR_%d=p2", i)
			} else {
				expected[i] = fmt.Sprintf("VAR_%d=p1", i)
			}
		}

		assert.Equal(t, expected, merged)
	})

	t.Run("65-entry total boundary across base, p2, p1 (map fallback path)", func(t *testing.T) {
		base := make([]string, 25)
		for i := range 25 {
			base[i] = fmt.Sprintf("VAR_%d=base", i)
		}

		p2 := make([]string, 20)
		for i := range 20 {
			p2[i] = fmt.Sprintf("VAR_%d=p2", i+15) // Overlaps VAR_15..VAR_24
		}

		p1 := make([]string, 20)
		for i := range 20 {
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+25) // Overlaps VAR_25..VAR_34
		}

		merged := mergeEnv(base, p2, p1)

		// First-seen key ordering rule across layers (Base -> P2 -> P1) with last-one-wins precedence.
		// Specification reference: docs/features/argument-priority-logic.md
		expected := make([]string, 45)
		for i := range 45 {
			if i < 15 {
				expected[i] = fmt.Sprintf("VAR_%d=base", i)
			} else if i < 25 {
				expected[i] = fmt.Sprintf("VAR_%d=p2", i)
			} else {
				expected[i] = fmt.Sprintf("VAR_%d=p1", i)
			}
		}

		assert.Equal(t, expected, merged)
	})
}
