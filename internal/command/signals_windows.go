//go:build windows
// +build windows

package command

import (
	"os"
	"os/signal"
)

// setupSignals sets up SIGINT notification on Windows.
func setupSignals(sigChan chan os.Signal) {
	signal.Notify(sigChan, os.Interrupt)
}

// setupResizeSignal is a stub for Windows.
func setupResizeSignal(resizeChan chan os.Signal) {
	// SIGWINCH is not available on Windows
}

// getSignalName returns the standard name for a signal on Windows.
func getSignalName(sig os.Signal) string {
	if sig == os.Interrupt {
		return "SIGINT"
	}
	return sig.String()
}
