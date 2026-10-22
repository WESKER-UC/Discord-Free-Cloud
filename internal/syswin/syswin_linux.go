//go:build linux

package syswin

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func GetDiskFreeSpace(path string) (*DiskSpace, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("could not check disk space on %s: %w", path, err)
	}

	blockSize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * blockSize
	freeBytes := stat.Bfree * blockSize
	availableBytes := stat.Bavail * blockSize
	usedBytes := totalBytes - freeBytes

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return &DiskSpace{
		Drive:          filepath.Clean(path),
		FreeBytes:      freeBytes,
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availableBytes,
		UsedPercent:    usedPercent,
	}, nil
}

func NormalizeLongPath(path string) string {
	return filepath.Clean(path)
}

func OpenBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}
