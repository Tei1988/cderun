package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	return runCderunCore(nil, args...)
}

func runCderunCore(stdin io.Reader, args ...string) (stdout, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	timeout := 60 * time.Second
	if val, ok := os.LookupEnv("CDERUN_TEST_TIMEOUT"); ok {
		if d, err := time.ParseDuration(val); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	capturedExitCode := 0
	execErr := ExecuteContextWithOptions(ctx, append([]string{"cderun"}, args...), func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}
		if stdin != nil {
			cmd.SetIn(stdin)
		}
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
	})

	return outBuf.String(), errBuf.String(), capturedExitCode, execErr
}
