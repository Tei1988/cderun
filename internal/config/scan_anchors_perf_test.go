package config

import (
	"strings"
	"testing"
	"time"
)

func TestScanAnchorsComplexity(t *testing.T) {
	// Create a string with many unmatched openers
	n := 10000
	input := strings.Repeat("{{", n)

	start := time.Now()
	scanAnchors(input)
	duration := time.Since(start)

	// If it's O(n), it should be very fast (typically < 1ms)
	// If it's O(n^2), 10000 openers would result in roughly 50 million operations,
	// which would take significantly longer (e.g. > 100ms on many environments).
	if duration > 100*time.Millisecond {
		t.Errorf("scanAnchors took too long: %v (potential O(n^2) complexity)", duration)
	}
	t.Logf("scanAnchors with %d unmatched openers took %v", n, duration)
}

func TestScanAnchorsCorrectnessWithUnmatched(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"{{ { { }}", []string{"{{ { { }}"}},
		{"{{ { { }", []string{}},
		{"{{ {{ }} }}", []string{"{{ {{ }} }}"}}, // nested outermost
		{"{{ }} {{", []string{"{{ }}"}},
		{"{{ {{", []string{}},
		{"{{ }} }}", []string{"{{ }}"}},
		{"prefix {{ A }} middle {{ B }} suffix", []string{"{{ A }}", "{{ B }}"}},
		{"{{ {{ A }} {{ B }} }}", []string{"{{ {{ A }} {{ B }} }}"}}, // nested
	}

	for _, tt := range tests {
		res := scanAnchors(tt.input)
		if len(res) != len(tt.expected) {
			t.Errorf("scanAnchors(%q) = %d ranges, expected %d", tt.input, len(res), len(tt.expected))
			continue
		}
		for i, r := range res {
			actual := tt.input[r.start:r.end]
			if actual != tt.expected[i] {
				t.Errorf("scanAnchors(%q) range %d = %q, expected %q", tt.input, i, actual, tt.expected[i])
			}
		}
	}
}
