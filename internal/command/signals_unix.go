//go:build !windows
// +build !windows

package command

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignals sets up SIGINT and SIGTERM notification.
func setupSignals(sigChan chan os.Signal) {
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
}

// setupResizeSignal sets up SIGWINCH notification.
func setupResizeSignal(resizeChan chan os.Signal) {
	signal.Notify(resizeChan, syscall.SIGWINCH)
}

// getSignalName returns the standard name for a signal.
func getSignalName(sig os.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return sig.String()
	}
}
