package runtime

import (
	"testing"

	"cderun/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestUnit_Containerd_Options(t *testing.T) {
	logger := logging.GetGlobalLogger()
	rt := &ContainerdRuntime{}
	WithContainerdLogger(logger)(rt)
	assert.Equal(t, logger, rt.logger)
}

func TestUnit_Containerd_Basic(t *testing.T) {
	rt := &ContainerdRuntime{}
	assert.Equal(t, "containerd", rt.Name())
}
