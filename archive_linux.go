package main

import (
	"syscall"
	"unsafe"
)

// https://github.com/docker/docker/blob/master/pkg/system/xattrs_linux.go
// Get the underlying data for an xattr of a file.
// Return a nil slice and nil error if the xattr is not set.
// Other than that, I have no idea how this function works.
func lgetxattr(path string, attr string) ([]byte, error) {
	pathBytes, err := syscall.BytePtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrBytes, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return nil, err
	}

	dest := make([]byte, 128)
	destBytes := unsafe.Pointer(&dest[0])
	sz, _, errno := syscall.Syscall6(
		syscall.SYS_LGETXATTR,
		uintptr(unsafe.Pointer(pathBytes)),
		uintptr(unsafe.Pointer(attrBytes)),
		uintptr(destBytes),
		uintptr(len(dest)),
		0,
		0,
	)
	if errno == syscall.ENODATA {
		return nil, nil
	}
	if errno == syscall.ERANGE {
		dest = make([]byte, sz)
		destBytes := unsafe.Pointer(&dest[0])
		sz, _, errno = syscall.Syscall6(
			syscall.SYS_LGETXATTR,
			uintptr(unsafe.Pointer(pathBytes)),
			uintptr(unsafe.Pointer(attrBytes)),
			uintptr(destBytes),
			uintptr(len(dest)),
			0,
			0,
		)
	}
	if errno != 0 {
		return nil, errno
	}

	return dest[:sz], nil
}
