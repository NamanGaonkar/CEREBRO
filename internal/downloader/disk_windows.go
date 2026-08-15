//go:build windows

package downloader

import "golang.org/x/sys/windows"

// DiskFree reports free and total bytes on the volume containing path.
func DiskFree(path string) (free, total uint64, err error) {
	var freeBytes, totalBytes, totalFree uint64
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	err = windows.GetDiskFreeSpaceEx(p, &freeBytes, &totalBytes, &totalFree)
	return freeBytes, totalBytes, err
}
