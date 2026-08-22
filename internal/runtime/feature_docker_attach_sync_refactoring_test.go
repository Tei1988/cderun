package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"cderun/internal/logging"
)

func TestUnit_DockerRuntime_DrainOutputOnStdinError(t *testing.T) {
	t.Run("output done during grace period", func(t *testing.T) {
		rt := &DockerRuntime{
			logger:                logging.GetGlobalLogger(),
			attachCloseWriteGrace: 100 * time.Millisecond,
			sleepFunc:             SleepFunc,
		}

		ctx := context.Background()
		stdinErr := errors.New("stdin broken pipe")
		outputDone := make(chan error, 1)
		outputDone <- nil

		err := rt.drainOutputOnStdinError(ctx, stdinErr, outputDone)
		assert.Equal(t, stdinErr, err)
	})

	t.Run("grace period expires before output done", func(t *testing.T) {
		rt := &DockerRuntime{
			logger:                logging.GetGlobalLogger(),
			attachCloseWriteGrace: 5 * time.Millisecond,
			sleepFunc:             SleepFunc,
		}

		ctx := context.Background()
		stdinErr := errors.New("stdin connection reset")
		outputDone := make(chan error, 1) // empty, output goroutine hangs or slow

		err := rt.drainOutputOnStdinError(ctx, stdinErr, outputDone)
		assert.Equal(t, stdinErr, err)
	})

	t.Run("context canceled during drain", func(t *testing.T) {
		rt := &DockerRuntime{
			logger:                logging.GetGlobalLogger(),
			attachCloseWriteGrace: 1 * time.Second,
			sleepFunc:             SleepFunc,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // canceled context

		stdinErr := errors.New("stdin write error")
		outputDone := make(chan error, 1)

		err := rt.drainOutputOnStdinError(ctx, stdinErr, outputDone)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestUnit_RuntimeCommon_ErrorHelpers(t *testing.T) {
	t.Run("IsRetryablePullError classified messages", func(t *testing.T) {
		assert.False(t, IsRetryablePullError(nil))
		assert.True(t, IsRetryablePullError(fmt.Errorf("connection refused by peer")))
		assert.True(t, IsRetryablePullError(fmt.Errorf("token expired")))
		assert.False(t, IsRetryablePullError(fmt.Errorf("unrecognized critical failure")))
	})

	t.Run("IsTemporaryAuthError classified keywords", func(t *testing.T) {
		assert.False(t, IsTemporaryAuthError(nil))
		assert.True(t, IsTemporaryAuthError(fmt.Errorf("token expired")))
		assert.True(t, IsTemporaryAuthError(fmt.Errorf("please reauthenticate to continue")))
		assert.False(t, IsTemporaryAuthError(fmt.Errorf("invalid password")))
	})
}
