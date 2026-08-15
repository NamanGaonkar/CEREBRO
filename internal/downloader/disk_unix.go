//go:build !windows

package downloader

import "golang.org/x/sys/unix"

// DiskFree reports free and total bytes on the volume containing path.
func DiskFree(path string) (free, total uint64, err error) {
	var st unix.Statfs_t
	if err = unix.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize), nil
}
