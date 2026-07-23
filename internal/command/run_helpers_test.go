package command

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"

	"cderun/internal/config"
)

func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, args...)
}

func runCderunCore(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	capturedExitCode := 0
	execErr := ExecuteContextWithOptions(ctx, append([]string{"cderun"}, args...), func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		o.mountInfoReader = &stubMountInfoReader{}
		o.socketGIDGetter = func(fs config.FileSystem, path string) (string, error) {
			return "", nil
		}
		if stdin != nil {
			cmd.SetIn(stdin)
		}
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
	})

	return outBuf.String(), errBuf.String(), capturedExitCode, execErr
}

type stubMountInfoReader struct{}

func (stubMountInfoReader) ReadMountInfo(fs config.FileSystem) ([]byte, error) {
	return []byte{}, nil
}
