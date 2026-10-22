//go:build windows

package syswin

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = modKernel32.NewProc("GetDiskFreeSpaceExW")
)

func GetDiskFreeSpace(path string) (*DiskSpace, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	volumeRoot := filepath.VolumeName(absPath)
	if volumeRoot == "" {
		volumeRoot = "C:"
	}
	driveName := volumeRoot
	if !strings.HasSuffix(volumeRoot, `\`) {
		volumeRoot += `\`
	}

	utf16Path, err := syscall.UTF16PtrFromString(volumeRoot)
	if err != nil {
		return nil, fmt.Errorf("path conversion failed: %w", err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	result, _, lastErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(utf16Path)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if result == 0 {
		return nil, fmt.Errorf("could not check disk space on %s: %w", volumeRoot, lastErr)
	}

	usedBytes := totalBytes - totalFreeBytes
	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return &DiskSpace{
		Drive:          driveName,
		FreeBytes:      totalFreeBytes,
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: freeBytesAvailable,
		UsedPercent:    usedPercent,
	}, nil
}

func NormalizeLongPath(path string) string {
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, `\\?\`) || strings.HasPrefix(cleaned, `\\.\`) {
		return cleaned
	}
	if filepath.IsAbs(cleaned) {
		return `\\?\` + cleaned
	}
	return cleaned
}

func OpenBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
