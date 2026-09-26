package config

import (
	"testing"
)

func BenchmarkParseSlice_Empty(b *testing.B) {
	emptySlice := []string{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := parseSlice[string](emptySlice, "test", nil)
		if err != nil || res != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolveUlimitsFromRaws_Empty(b *testing.B) {
	emptyRaws := []string{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := resolveUlimitsFromRaws(emptyRaws, nil)
		if err != nil || res != nil {
			b.Fatal(err)
		}
	}
}
