//go:build windows

package command

import (
	"cderun/internal/config"
)

func getSocketGID(fs config.FileSystem, socketPath string) (string, error) {
	return "", nil
}
