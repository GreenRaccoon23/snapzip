// +build !windows

package main

import (
	"archive/tar"
	"fmt"
	"syscall"
)

// https://github.com/docker/docker/blob/master/pkg/archive/archive_unix.go
// Add a file's device major and minor numbers
//   to the file's header within a tar archive.
// Return the file's inode and the number of hardlinks to that inode.
func tarSetHeader(hdr *tar.Header, stat interface{}) (hasHardLinks bool, inode uint64, err error) {
	s, ok := stat.(*syscall.Stat_t)
	if !ok {
		err = fmt.Errorf("cannot convert stat value to syscall.Stat_t")
		return
	}

	hasHardLinks = (uint32(s.Nlink) > 1)
	inode = uint64(s.Ino)

	// Currently go does not fil in the major/minors
	if s.Mode&syscall.S_IFBLK != 0 || s.Mode&syscall.S_IFCHR != 0 {
		hdr.Devmajor = int64(devmajor(uint64(s.Rdev)))
		hdr.Devminor = int64(devminor(uint64(s.Rdev)))
	}

	return
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive_unix.go
// Return the device major number of system data from syscall.Stat_t.Rdev.
func devmajor(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive_unix.go
// Return the device minor number of system data from syscall.Stat_t.Rdev.
func devminor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}
