package command

import (
	"context"
	"io"
	"os"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// TestUnit_Command_DoubleDashHoisting_Expansion verifies hoisting logic when a double-dash is present.
// Reference: docs/features/argument-parsing.md
func TestUnit_Command_DoubleDashHoisting_Expansion(t *testing.T) {
	t.Parallel()

	mockRuntime := &runtime.MockRuntime{}
	args := []string{
		"cderun",
		"sh",
		"--",
		"--cderun-image=alpine:latest",
		"--cderun-tty=true",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)

	// Since cderun ignores '--' for hoisting, `--cderun-image=alpine:latest` and `--cderun-tty=true` are hoisted.
	assert.Equal(t, "alpine:latest", cfg.Image)
	assert.True(t, cfg.TTY)
	assert.Equal(t, []string{"--"}, cfg.Command)
}

// TestUnit_Command_ValueTakingP1_Expansion verifies that space-separated and equals-sign formatted
// value-taking overrides are both handled correctly during preprocessing.
// Reference: docs/features/argument-parsing.md
func TestUnit_Command_ValueTakingP1_Expansion(t *testing.T) {
	t.Parallel()

	// 1. Space-separated format: `--cderun-image alpine:3.18`
	t.Run("space separated format wins", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"sh",
			"--cderun-image", "alpine:3.18",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "alpine:3.18", cfg.Image)
	})

	// 1b. Equals-sign format: `--cderun-image=alpine:3.18`
	t.Run("equals sign format wins", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		args := []string{
			"cderun",
			"sh",
			"--cderun-image=alpine:3.18",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "alpine:3.18", cfg.Image)
	})

	// 2. Requires a value error when adjacent parameter is missing or is another flag
	t.Run("requires a value error on missing adjacent parameter", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image", // missing value
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})

	t.Run("requires a value error on adjacent flag", func(t *testing.T) {
		t.Parallel()
		args := []string{
			"cderun",
			"sh",
			"--cderun-image", "--cderun-tty", // value is another flag (invalid)
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return &runtime.MockRuntime{}, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a value")
	})
}

// TestUnit_Command_NestedExecution_Priority verifies priority layers in nested execution contexts.
// Reference: docs/features/nested-execution.md
func TestUnit_Command_NestedExecution_Priority(t *testing.T) {
	t.Parallel()

	// Level 0 (Host): PWD resolves to host WD
	t.Run("Level 0 Host context resolution", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/my/host/pwd",
		}
		args := []string{
			"cderun",
			"--image", "alpine",
			"--env", "TEST_DIR={{BASE_PWD}}",
			"sh",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.Env, "TEST_DIR=/my/host/pwd")
	})

	// Level 1 (Nested): BASE_PWD resolves to physical outer host WD rather than local container WD
	t.Run("Level 1 Nested context resolution", func(t *testing.T) {
		t.Parallel()
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/local/container/pwd",
			Files: map[string][]byte{
				"/local/container/pwd/.cderun.yaml": []byte(`
hostContext:
  level: 1
  homeDir: /physical/host/home
  workingDir: /physical/host/pwd
`),
			},
			Dirs: map[string]bool{
				"/local/container/pwd": true,
			},
		}
		args := []string{
			"cderun",
			"--image", "alpine",
			"--env", "TEST_DIR={{BASE_PWD}}",
			"sh",
		}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Contains(t, cfg.Env, "TEST_DIR=/physical/host/pwd")
	})
}

// TestUnit_Command_SymlinkMode_Unicode_Expansion expands testing of Symlink Mode with unicode / CJK.
// Reference: docs/features/polyglot-entry.md
func TestUnit_Command_SymlinkMode_Unicode_Expansion(t *testing.T) {
	t.Parallel()

	// Test CJK arguments are perfectly preserved and cleaned
	mockRuntime := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/workspace",
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte("python:\n  image: python:latest"),
		},
	}

	args := []string{
		"./python/../python", // relative unclean path
		"-c",
		"print('日本語 & Emojis ✨')",
	}

	err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})

	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "python:latest", cfg.Image)
	assert.Equal(t, []string{"-c", "print('日本語 & Emojis ✨')"}, cfg.Command)
}

type signalRecordingMockRuntime struct {
	runtime.MockRuntime
	waitChan    chan int
	signals     []string
	mu          sync.Mutex
	startedChan chan struct{}
	startOnce   sync.Once
}

func (m *signalRecordingMockRuntime) StartContainer(ctx context.Context, id string) error {
	if m.startedChan != nil {
		m.startOnce.Do(func() {
			close(m.startedChan)
		})
	}
	return m.MockRuntime.StartContainer(ctx, id)
}

func (m *signalRecordingMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.signals = append(m.signals, sig)
	m.mu.Unlock()
	return nil
}

func (m *signalRecordingMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	select {
	case code := <-m.waitChan:
		return code, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (m *signalRecordingMockRuntime) getSignals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	sigs := make([]string, len(m.signals))
	copy(sigs, m.signals)
	return sigs
}

// TestUnit_Command_Signals_SighupSigquitAndConsecutive verifies signal forwarding and consecutive cancellations.
// Reference: docs/features/signal-handling-security.md
func TestUnit_Command_Signals_SighupSigquitAndConsecutive(t *testing.T) {
	t.Parallel()

	// 1. SIGHUP forwarding
	t.Run("SIGHUP forwarding", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		startedChan := make(chan struct{})
		mock := &signalRecordingMockRuntime{
			waitChan:    waitChan,
			startedChan: startedChan,
		}
		mock.CreatedContainerID = "sighup-test"

		var capturedSigChan chan os.Signal
		var sigChanMu sync.Mutex

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var execErr error
		done := make(chan struct{})
		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return false }
				o.setupSignals = func(sigChan chan os.Signal) {
					sigChanMu.Lock()
					capturedSigChan = sigChan
					sigChanMu.Unlock()
				}
				o.stopSignalHandling = func(sigChan chan os.Signal) {}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			close(done)
		}()

		require.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Wait deterministically for container startup to begin
		select {
		case <-startedChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Container did not start in time")
		}

		// Copy the channel under the mutex before sending
		sigChanMu.Lock()
		localSigChan := capturedSigChan
		sigChanMu.Unlock()

		// Send simulated SIGHUP
		localSigChan <- syscall.SIGHUP

		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGHUP")
		}, 2*time.Second, 10*time.Millisecond)

		// Exit normally using timeout select
		select {
		case waitChan <- 0:
		case <-time.After(1 * time.Second):
			t.Fatal("Failed to send exit code to waitChan (timed out)")
		}
		<-done
		require.NoError(t, execErr)
	})

	// 2. SIGQUIT forwarding
	t.Run("SIGQUIT forwarding", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		startedChan := make(chan struct{})
		mock := &signalRecordingMockRuntime{
			waitChan:    waitChan,
			startedChan: startedChan,
		}
		mock.CreatedContainerID = "sigquit-test"

		var capturedSigChan chan os.Signal
		var sigChanMu sync.Mutex

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var execErr error
		done := make(chan struct{})
		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return false }
				o.setupSignals = func(sigChan chan os.Signal) {
					sigChanMu.Lock()
					capturedSigChan = sigChan
					sigChanMu.Unlock()
				}
				o.stopSignalHandling = func(sigChan chan os.Signal) {}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			close(done)
		}()

		require.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Wait deterministically for container startup to begin
		select {
		case <-startedChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Container did not start in time")
		}

		// Copy the channel under the mutex before sending
		sigChanMu.Lock()
		localSigChan := capturedSigChan
		sigChanMu.Unlock()

		// Send simulated SIGQUIT
		localSigChan <- syscall.SIGQUIT

		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGQUIT")
		}, 2*time.Second, 10*time.Millisecond)

		// Exit normally using timeout select
		select {
		case waitChan <- 0:
		case <-time.After(1 * time.Second):
			t.Fatal("Failed to send exit code to waitChan (timed out)")
		}
		<-done
		require.NoError(t, execErr)
	})

	// 3. Consecutive signals context cancellation
	t.Run("second SIGINT cancels the host context", func(t *testing.T) {
		t.Parallel()
		waitChan := make(chan int)
		startedChan := make(chan struct{})
		mock := &signalRecordingMockRuntime{
			waitChan:    waitChan,
			startedChan: startedChan,
		}
		mock.CreatedContainerID = "consecutive-cancel-test"

		var capturedSigChan chan os.Signal
		var sigChanMu sync.Mutex

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var execErr error
		done := make(chan struct{})
		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return false }
				o.setupSignals = func(sigChan chan os.Signal) {
					sigChanMu.Lock()
					capturedSigChan = sigChan
					sigChanMu.Unlock()
				}
				o.stopSignalHandling = func(sigChan chan os.Signal) {}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			close(done)
		}()

		require.Eventually(t, func() bool {
			sigChanMu.Lock()
			defer sigChanMu.Unlock()
			return capturedSigChan != nil
		}, 2*time.Second, 10*time.Millisecond)

		// Wait deterministically for container startup to begin
		select {
		case <-startedChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Container did not start in time")
		}

		// Copy the channel under the mutex before sending
		sigChanMu.Lock()
		localSigChan := capturedSigChan
		sigChanMu.Unlock()

		// First SIGINT (forwarded)
		localSigChan <- syscall.SIGINT

		assert.Eventually(t, func() bool {
			return slices.Contains(mock.getSignals(), "SIGINT")
		}, 2*time.Second, 10*time.Millisecond)

		// Second SIGINT (triggers context cancellation)
		localSigChan <- syscall.SIGINT

		// Wait for execution to finish with context.Canceled error
		select {
		case <-done:
			require.Error(t, execErr)
			require.ErrorIs(t, execErr, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("Execution did not cancel after rapid second SIGINT")
		}
	})
}
