package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"cderun/internal/logging"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitDockerRetryablePullErrorExhaustive(t *testing.T) {
	assert.False(t, IsRetryablePullError(nil))
	assert.True(t, IsRetryablePullError(errors.New("toomanyrequests")))
	assert.True(t, IsRetryablePullError(errors.New("rate exceeded")))
	assert.True(t, IsRetryablePullError(errors.New("rate limit")))
	assert.True(t, IsRetryablePullError(errors.New("data limit exceeded")))
	assert.True(t, IsRetryablePullError(errors.New("i/o timeout")))
	assert.True(t, IsRetryablePullError(errors.New("connection refused")))
	assert.True(t, IsRetryablePullError(errors.New("connection reset")))
	assert.True(t, IsRetryablePullError(io.ErrUnexpectedEOF))
	assert.True(t, IsRetryablePullError(errors.New("token expired")))
	assert.False(t, IsRetryablePullError(errors.New("unauthorized")))
	assert.False(t, IsRetryablePullError(errors.New("other error")))
	assert.False(t, IsRetryablePullError(io.EOF))
}

type blockingReader struct {
	ctx context.Context
}

func (r *blockingReader) Read(p []byte) (n int, err error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func TestUnit_Docker_AttachContainer_RacesAndErrors(t *testing.T) {
	logger := logging.GetGlobalLogger()

	t.Run("stdout finishes before stdin error is processed", func(t *testing.T) {
		conn := &mockConn{}
		// Output finishes immediately
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(bytes.NewReader([]byte("fast output"))),
			},
		}
		rt := &DockerRuntime{
			logger:    logger,
			client:    mock,
			sleepFunc: noopSleepFunc,
		}

		// Stdin fails after a short delay
		pr, pw := io.Pipe()
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = pw.CloseWithError(errors.New("slow stdin error"))
		}()

		err := rt.AttachContainer(context.Background(), "id", true, pr, io.Discard, io.Discard, nil)

		// In the current implementation, if output finishes fast, the pending stdin error might be missed.
		// This test documents the behavior (T09). Since output finishes immediately, it's expected to return nil.
		assert.NoError(t, err, "Expected no error because stdout finishes before stdin error is recorded")
	})

	t.Run("stdin error wins", func(t *testing.T) {
		conn := &mockConn{}
		// Output blocks
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(&blockingReader{ctx: ctx}),
			},
		}
		rt := &DockerRuntime{
			logger:    logger,
			client:    mock,
			sleepFunc: noopSleepFunc,
		}

		// Stdin fails immediately
		err := rt.AttachContainer(ctx, "id", true, &syncFailingReader{started: make(chan struct{}, 1)}, io.Discard, io.Discard, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read error")
	})

	t.Run("context cancellation during copy", func(t *testing.T) {
		conn := &mockConn{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(&blockingReader{ctx: ctx}),
			},
		}
		rt := &DockerRuntime{
			logger:    logger,
			client:    mock,
			sleepFunc: noopSleepFunc,
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := rt.AttachContainer(ctx, "id", true, &blockingReader{ctx: ctx}, io.Discard, io.Discard, nil)
		require.ErrorIs(t, err, context.Canceled)
	})
}

type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

func TestUnit_Docker_AttachContainer_WriterErrors(t *testing.T) {
	logger := logging.GetGlobalLogger()

	t.Run("stdout write error", func(t *testing.T) {
		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(bytes.NewReader([]byte("some output"))),
			},
		}
		rt := &DockerRuntime{
			logger:    logger,
			client:    mock,
			sleepFunc: noopSleepFunc,
		}

		err := rt.AttachContainer(context.Background(), "id", true, nil, &errorWriter{err: errors.New("write failed")}, io.Discard, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write failed")
	})
}

func TestUnit_Docker_AttachContainer_CloseWriteGrace(t *testing.T) {
	logger := logging.GetGlobalLogger()

	t.Run("CloseWrite is called after grace", func(t *testing.T) {
		conn := &mockConn{}
		mock := &mockDockerClient{
			attachResp: types.HijackedResponse{
				Conn:   conn,
				Reader: bufio.NewReader(strings.NewReader("")),
			},
		}

		graceCalled := make(chan struct{})
		rt := &DockerRuntime{
			logger: logger,
			client: mock,
			attachCloseWriteGrace: 10 * time.Millisecond,
			sleepFunc: func(ctx context.Context, d time.Duration) error {
				assert.Equal(t, 10*time.Millisecond, d)
				close(graceCalled)
				return nil
			},
		}

		stdin := strings.NewReader("input")
		err := rt.AttachContainer(context.Background(), "id", false, stdin, io.Discard, io.Discard, nil)
		require.NoError(t, err)

		select {
		case <-graceCalled:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for graceCalled")
		}
		assert.Eventually(t, func() bool { return conn.closeWriteCalled.Load() }, 500*time.Millisecond, 10*time.Millisecond)
	})
}

func TestUnit_Docker_WithAttachCloseWriteGrace(t *testing.T) {
	rt := &DockerRuntime{}
	WithAttachCloseWriteGrace(50 * time.Millisecond)(rt)
	assert.Equal(t, 50*time.Millisecond, rt.attachCloseWriteGrace)

	WithAttachCloseWriteGrace(0)(rt)
	assert.Equal(t, 1*time.Millisecond, rt.attachCloseWriteGrace)
}

func TestUnit_Docker_NewDockerRuntimeWithName(t *testing.T) {
	rt, err := NewDockerRuntimeWithName("/tmp/docker.sock", "podman")
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name())
}

func TestUnit_Docker_PullImage_MaxRetries(t *testing.T) {
	mock := &mockDockerClient{
		imagePullErr: errors.New("toomanyrequests"),
	}
	rt := &DockerRuntime{
		logger:    logging.GetGlobalLogger(),
		client:    mock,
		sleepFunc: noopSleepFunc,
	}

	// maxRetries = -1 should be treated as 0
	err := rt.PullImage(context.Background(), "img", "always", -1, 1*time.Second)
	require.Error(t, err)
	assert.Equal(t, 1, mock.pullCount)
}
