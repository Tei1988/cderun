//go:build !windows

package command

import (
	"strconv"
	"syscall"

	"cderun/internal/config"
)

func getSocketGID(fs config.FileSystem, socketPath string) (string, error) {
	info, err := fs.Stat(socketPath)
	if err != nil {
		return "", err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Gid != 0 {
		return strconv.FormatUint(uint64(stat.Gid), 10), nil
	}
	return "", nil
}
