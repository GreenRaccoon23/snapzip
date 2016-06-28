package main

// https://github.com/docker/docker/blob/master/pkg/system/xattrs_linux.go
// This only works for linux.
func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}
