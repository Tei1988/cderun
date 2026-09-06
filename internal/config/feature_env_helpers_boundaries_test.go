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

		assert.Len(t, env, 64)

		deduped := deduplicateEnv(env)

		// Assert length (60 unique keys)
		assert.Len(t, deduped, 60)

		// Assert last-one-wins values
		assert.Contains(t, deduped, "VAR_0=v2")
		assert.Contains(t, deduped, "VAR_1=v2")
		assert.Contains(t, deduped, "VAR_2=v2")
		assert.Contains(t, deduped, "VAR_3=v2")
		assert.NotContains(t, deduped, "VAR_0=v1")
		assert.NotContains(t, deduped, "VAR_1=v1")
	})

	t.Run("65-entry boundary with duplicate keys (map fallback path)", func(t *testing.T) {
		// 60 unique entries + 5 duplicates = 65 total items
		env := make([]string, 60)
		for i := 0; i < 60; i++ {
			env[i] = fmt.Sprintf("VAR_%d=v1", i)
		}
		// Append duplicates for VAR_0, VAR_1, VAR_2, VAR_3, VAR_4
		env = append(env, "VAR_0=v2", "VAR_1=v2", "VAR_2=v2", "VAR_3=v2", "VAR_4=v2")

		assert.Len(t, env, 65)

		deduped := deduplicateEnv(env)

		// Assert length (60 unique keys)
		assert.Len(t, deduped, 60)

		// Assert last-one-wins values
		assert.Contains(t, deduped, "VAR_0=v2")
		assert.Contains(t, deduped, "VAR_1=v2")
		assert.Contains(t, deduped, "VAR_2=v2")
		assert.Contains(t, deduped, "VAR_3=v2")
		assert.Contains(t, deduped, "VAR_4=v2")
		assert.NotContains(t, deduped, "VAR_0=v1")
		assert.NotContains(t, deduped, "VAR_1=v1")
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
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+15) // Overlaps VAR_15..VAR_29
		}

		// Total inputs: 20 + 20 + 24 = 64
		assert.Equal(t, 64, len(base)+len(p2)+len(p1))

		merged := mergeEnv(base, p2, p1)

		// Total unique keys: VAR_0..VAR_38 (39 unique keys)
		assert.Len(t, merged, 39)

		// Priority check: p1 > p2 > base
		assert.Contains(t, merged, "VAR_5=base")
		assert.Contains(t, merged, "VAR_12=p2")
		assert.Contains(t, merged, "VAR_18=p1")
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
			p1[i] = fmt.Sprintf("VAR_%d=p1", i+20) // Overlaps VAR_20..VAR_34
		}

		// Total inputs: 25 + 20 + 20 = 65 (>64)
		assert.Equal(t, 65, len(base)+len(p2)+len(p1))

		merged := mergeEnv(base, p2, p1)

		// Total unique keys: VAR_0..VAR_39 (40 unique keys)
		assert.Len(t, merged, 40)

		// Priority check: p1 > p2 > base
		assert.Contains(t, merged, "VAR_5=base")
		assert.Contains(t, merged, "VAR_18=p2")
		assert.Contains(t, merged, "VAR_22=p1")
	})
}
