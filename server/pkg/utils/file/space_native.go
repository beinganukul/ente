//go:build !js

package file

import "syscall"

func FreeSpace(path string) (uint64, error) {
	var fs syscall.Statfs_t
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return 0, err
	}
	return fs.Bfree * uint64(fs.Bsize), nil
}
