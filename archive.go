package main

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
type tarchive struct {
	dstName string
	dst     *os.File
	writer  *tar.Writer
	// Map inodes to hardlinks.
	hardlinks map[uint64]string

	srcName string
	src     *os.File
	reader  *tar.Reader
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Create a tar archive of a directory.
func tarDir(src *os.File) (string, error) {

	// Get file info for the source directory.
	srcInfo, err := src.Stat()
	if err != nil {
		return "", err
	}

	srcName := src.Name()
	srcMode := srcInfo.Mode()
	baseName := filepath.Base(srcName)
	dstName := concat(baseName, ".tar")
	setDstName(&dstName)

	var t *tarchive
	err = t.create(dstName, srcMode)
	if err != nil {
		return "", err
	}

	defer t.close()

	err = t.tar(srcName)
	if err != nil {
		return "", err
	}

	return dstName, nil
}

func (t *tarchive) create(dstName string, mode os.FileMode) error {

	dst, err := create(dstName, mode)
	if err != nil {
		return err
	}

	var dstWriteCloser io.WriteCloser = dst

	t.dstName = dstName
	t.dst = dst
	t.writer = tar.NewWriter(dstWriteCloser)
	t.hardlinks = make(map[uint64]string)

	return nil
}

func (t *tarchive) close() {

	if t.dst != nil {
		t.dst.Close()
		t.writer.Close()
		t.hardlinks = nil
	}

	if t.src != nil {
		t.src.Close()
		t.reader = nil
	}

	t = nil
}

// Walk through the directory.
// Add a header to the tar archive for each file encountered.
func (t *tarchive) tar(srcName string) error {

	dstName := t.dstName
	var total int
	var progress int
	var start time.Time
	parent := filepath.Dir(srcName)

	if !DoQuiet {
		total = dirSize(srcName)
	}

	if !DoQuiet {
		print(concat(srcName, "  >  ", dstName))
		defer print()
	}

	return filepath.Walk(srcName, func(path string, fi os.FileInfo, err error) error {
		// Quit if any errors occur.
		if err != nil {
			return err
		}

		// Don't use the full path of the file in its header name.
		// Otherwise, the archive may extract an unnecessarily long path with
		//   anoying, empty diretories.
		// E.g., make an archive of '/home/me/Documents' extract to
		//   'Documents', not to '/home/me/Documents'.
		name, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}

		// Get a header for the file.
		hdr, err := t.header(path, name)
		if err != nil {
			return err
		}

		// Write the header.
		if err := t.write(hdr, path); err != nil {
			return err
		}

		// Skip printing progress if user requested it.
		if DoQuiet {
			return nil
		}

		// Make sure progress isn't outputted too quickly
		//   for the console.
		progress++
		percent := int(float64(progress) / float64(total) * float64(100))
		if int(time.Since(start)) < 500000 && percent < 98 {
			return nil
		}
		start = time.Now()

		// Print progress.
		fmt.Printf(
			"\r  %v%%   %v / %v files",
			percent, progress, total,
		)
		return nil
	})
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Add a file [as a header] to a tar archive.
func (t *tarchive) header(path, name string) (*tar.Header, error) {

	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	// If the file is a symlink, find its target.
	var link string
	if isSymlink := (fi.Mode()&os.ModeSymlink != 0); isSymlink {
		if link, err = os.Readlink(path); err != nil {
			return nil, err
		}
	}

	// Create the tar header.
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return nil, err
	}

	// Set the header name.
	// If the file is a directory, add a trailing "/".
	if isDir := (fi.Mode()&os.ModeDir != 0); isDir {
		fmtDir(&name)
	}
	hdr.Name = name

	// Check if the file has hard links.
	hasHardlinks, inode, err := tarSetHeader(hdr, fi.Sys())
	if err != nil {
		return nil, err
	}

	// If any other regular files link to the same inode as this file,
	//   prepare to treat it as a "hardlink" in the header.
	// If the tar archive contains another hardlink to this file's inode,
	//   set it as a "hardlink" in the tar header.
	// Otherwise, treat it as a regular file.
	if fi.Mode().IsRegular() && hasHardlinks {
		// If this file is NOT the first found hardlink to this inode,
		//   set the previously found hardlink as its 'Linkname'.
		if firstInode, ok := t.hardlinks[inode]; ok {
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = firstInode
			// Set size to 0 when not adding additional inodes.
			//   Otherwise, the writer's math will not add up correctly.
			hdr.Size = 0

			// If this file IS the first hardlink to this inode,
			//   note the file with its inode and treat it as a regular file.
			// It will become the 'Linkname' for another hardlink
			//   further down in the archive.
		} else {
			t.hardlinks[inode] = name
		}
	}

	// Find any security.capability xattrs and set the header accordingly.
	capability, _ := lgetxattr(path, "security.capability")
	if capability != nil {
		hdr.Xattrs = make(map[string]string)
		hdr.Xattrs["security.capability"] = string(capability)
	}

	return hdr, nil
}

func (t *tarchive) write(hdr *tar.Header, path string) error {

	// Write the header.
	tw := t.writer
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	// If the file is not a regular one,
	// i.e., a symlink, directory, or hardlink,
	// skip adding its contents to the archive (since it does not have any).
	if hdr.Typeflag != tar.TypeReg {
		return nil
	}

	// Write the file's contents to the archive.
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// tb := t.bufioWriter
	// var tb *bufio.Writer
	// tb.Reset(tw)
	tb := bufio.NewWriter(tw)
	defer tb.Reset(nil)

	_, err = io.Copy(tb, file)
	file.Close()
	if err != nil {
		return err
	}

	return tb.Flush()
}

// Extract a tar archive.
func untar(src *os.File) (string, error) {

	srcName := src.Name()

	var t *tarchive
	err := t.open(srcName)
	if err != nil {
		return "", err
	}

	defer t.close()

	// Get the smallest directory name (top directory).
	headName, err := t.head()
	if err != nil {
		return "", err
	}

	// Make sure existing files are not overwritten.
	dstName := headName
	setDstName(&dstName)

	err = t.untar(dstName, headName)
	if err != nil {
		return "", fmt.Errorf("%v\nFailed to extract %v", err, srcName)
	}

	return dstName, nil
}

func (t *tarchive) open(srcName string) error {

	src, err := os.Open(srcName)
	if err != nil {
		return err
	}

	t.srcName = srcName
	t.src = src
	t.reader = tar.NewReader(src)

	return nil
}

// Search a tar file for the top-level directory to be extracted.
func (t *tarchive) head() (string, error) {

	tr := t.reader
	srcName := t.srcName

	var headName string
	var err error

	defer func() {
		t.close()
		t = &tarchive{}
		t.open(srcName)
	}()

	// Get the smallest directory name (top directory).
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()

		// Break if the end of the tar archive has been reached.
		if err != nil {
			if done := (err == io.EOF); done {
				err = nil
			}
			break
		}

		// Set headName to the very first header name.
		// Most likely, this will be the name of the top directory anyway.
		if headName == "" {
			headName = hdr.Name
		}

		// Skip non-directories.
		if hdr.Typeflag != tar.TypeDir {
			continue
		}

		// The top directory is the shortest path and has the shortest name.
		if higher := (len(hdr.Name) < len(headName)); higher {
			headName = hdr.Name
		}
	}

	if err != nil {
		return "", err
	}

	// If no names were found, the data is corrupt.
	if noHead := (headName == ""); noHead {
		return "", fmt.Errorf("unable to read %v", srcName)
	}

	// Strip off the trailing '/'.
	headName = headName[0 : len(headName)-1]

	return headName, nil
}

// Extract a tar archive.
func (t *tarchive) untar(dstName string, headName string) error {

	src := t.src
	srcName := t.srcName
	tr := t.reader

	// Get file info.
	srcInfo, err := src.Stat()
	if err != nil {
		return err
	}
	srcSize := srcInfo.Size()
	total := uint64(srcSize)

	var progress uint64
	var outputLength int
	var start time.Time

	if !DoQuiet {
		print(concat(srcName, "  >  ", dstName))
		defer print()
	}

	// Extract the archive.
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()

		// Break if the end of the tar archive has been reached.
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		// Make sure existing files are not overwritten.
		name := hdr.Name
		name = strings.Replace(name, headName, dstName, 1)
		name = unusedPath(name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			// Extract a directory.
			err = os.MkdirAll(name, os.FileMode(hdr.Mode))

		case tar.TypeReg, tar.TypeRegA:
			// Extract a regular file.
			var w *os.File
			w, err = create(name, os.FileMode(hdr.Mode))
			if err != nil {
				break
			}
			_, err = io.Copy(w, tr)
			w.Close()

		case tar.TypeLink:
			// Extract a hard link.
			err = os.Link(hdr.Linkname, name)

		case tar.TypeSymlink:
			// Extract a symlink.
			err = os.Symlink(hdr.Linkname, name)

		default:
			// If the Typeflag is missing, the data is probably corrupt.
			// Just skip to the next one anyway if this happens.
			continue
		}

		if err != nil {
			break
		}

		// Print progress.
		if DoQuiet || hdr.Size == int64(0) {
			continue
		}
		progress = progress + uint64(hdr.Size)
		percent := int(float64(progress) / float64(total) * float64(100))

		// Make sure progress isn't outputted more quickly
		//   than the console can print.
		if int(time.Since(start)) < 100000 && percent < 99 {
			continue
		}
		start = time.Now()

		output := fmt.Sprintf(
			"  %v%%   %v / %v",
			percent, sizeLabel(progress), sizeLabel(total),
		)
		// Clear previous output.
		if len(output) > outputLength {
			outputLength = len(output)
		}
		fmt.Printf("\r%v", strings.Repeat(" ", outputLength))
		// Print new output.
		fmt.Printf("\r%v", output)
	}

	if err != nil {
		return err
	}

	return nil
}
