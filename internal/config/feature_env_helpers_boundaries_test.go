package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_DeduplicateEnv_Boundaries(t *testing.T) {
	t.Run("64-entry boundary with duplicate keys (fixed stack array path)", func(t *testing.T) {
		// 60 unique entries + 4 duplicates = 64 total items
		env := make([]string, 60)
		for i := 0; i < 60; i++ {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}
		// Append duplicates for VAR_0, VAR_1, VAR_2, VAR_3
		env = append(env, "VAR_0=v2", "VAR_1=v2", "VAR_2=v2", "VAR_3=v2")

		deduped := deduplicateEnv(env)

		// Define exact expected ordered slice (60 items)
		expected := make([]string, 60)
		for i := 0; i < 60; i++ {
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
		for i := 0; i < 60; i++ {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}
		// Append duplicates for VAR_0, VAR_1, VAR_2, VAR_3, VAR_4
		env = append(env, "VAR_0=v2", "VAR_1=v2", "VAR_2=v2", "VAR_3=v2", "VAR_4=v2")

		deduped := deduplicateEnv(env)

		// Define exact expected ordered slice (60 items)
		expected := make([]string, 60)
		for i := 0; i < 60; i++ {
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
	t.Run("64-entry total boundary across base, p2, p1 (fixed stack array path)", func(t *testing.T) {
		base := make([]string, 20)
		for i := 0; i < 20; i++ {
			base[i] = fmt.Sprintf("VAR_%d=base", i)
		}

		p2 := make([]string, 20)
		for i := 0; i < 20; i++ {
			p2[i] = fmt.Sprintf("VAR_%d=p2", i+10) // Overlaps VAR_10..VAR_19
		}

		p1 := make([]string, 24)
		for i := 0; i < 24; i++ {
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+20) // Overlaps VAR_20..VAR_29
		}

		merged := mergeEnv(base, p2, p1)

		// Construct complete expected ordered slice (44 items: VAR_0..VAR_43)
		expected := make([]string, 44)
		for i := 0; i < 44; i++ {
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
		for i := 0; i < 25; i++ {
			base[i] = fmt.Sprintf("VAR_%d=base", i)
		}

		p2 := make([]string, 20)
		for i := 0; i < 20; i++ {
			p2[i] = fmt.Sprintf("VAR_%d=p2", i+15) // Overlaps VAR_15..VAR_24
		}

		p1 := make([]string, 20)
		for i := 0; i < 20; i++ {
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+25) // Overlaps VAR_25..VAR_34
		}

		merged := mergeEnv(base, p2, p1)

		// Construct complete expected ordered slice (45 items: VAR_0..VAR_44)
		expected := make([]string, 45)
		for i := 0; i < 45; i++ {
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
