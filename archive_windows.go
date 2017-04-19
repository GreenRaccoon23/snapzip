package main

import "archive/tar"

// https://github.com/docker/docker/blob/master/pkg/archive/archive_unix.go
// Windows does not use anything like this on its filesystem.
// Add a file's device major and minor numbers
//   to the file's header within a tar archive.
// Return the file's inode and the number of hardlinks to that inode.
func tarSetHeader(hdr *tar.Header, stat interface{}) (hasHardlinks bool, inode uint64, err error) {
	return
}

// https://github.com/docker/docker/blob/master/pkg/system/xattrs_linux.go
// This only works for linux.
// Get the underlying data for an xattr of a file.
// Return a nil slice and nil error if the xattr is not set.
// Other than that, I have no idea how this function works.
func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}
