package command

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Signals_GetSignalName(t *testing.T) {
	tests := []struct {
		sig  os.Signal
		want string
	}{
		{os.Interrupt, "SIGINT"},
		{os.Kill, "SIGKILL"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := getSignalName(tt.sig); got != tt.want {
				t.Errorf("getSignalName() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("Other signals", func(t *testing.T) {
		// Just ensure it doesn't panic
		assert.NotEmpty(t, getSignalName(os.Interrupt))
	})
}
