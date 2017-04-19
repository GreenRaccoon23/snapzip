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
type tarAppender struct {
	tarWriter *tar.Writer
	// Map inodes to hardlinks.
	hardLinks  map[uint64]string
	hardLinks2 []uint64
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
	baseName := filepath.Base(srcName)
	parent := filepath.Dir(srcName)

	// Make sure existing files are not overwritten.
	dstName := concat(baseName, ".tar")
	setDstName(&dstName)

	if !DoQuiet {
		fmt.Println(concat(srcName, "  >  ", dstName))
		defer fmt.Println()
	}

	// Create the destination file.
	dst, err := create(dstName, srcInfo.Mode())
	if err != nil {
		return "", err
	}

	// Pipe the destination file through a *tarAppender.
	var dstWriter io.WriteCloser = dst
	ta := &tarAppender{
		tarWriter: tar.NewWriter(dstWriter),
		hardLinks: make(map[uint64]string),
	}

	// Remember to close the tarWriter.
	defer ta.tarWriter.Close()

	// Walk through the directory.
	// Add a header to the tar archive for each file encountered.
	var total, progress int
	var start time.Time
	if !DoQuiet {
		total = dirSize(srcName)
	}

	err = filepath.Walk(srcName, func(path string, fi os.FileInfo, err error) error {
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
		hdr, err := ta.getHeader(path, name)
		if err != nil {
			return err
		}

		// Write the header.
		if err := ta.write(hdr, path); err != nil {
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
	if err != nil {
		return "", err
	}

	return dstName, nil
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Add a file [as a header] to a tar archive.
func (ta *tarAppender) getHeader(path, name string) (*tar.Header, error) {

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
	hasHardLinks, inode, err := tarSetHeader(hdr, fi.Sys())
	if err != nil {
		return nil, err
	}

	// If any other regular files link to the same inode as this file,
	//   prepare to treat it as a "hardlink" in the header.
	// If the tar archive contains another hardlink to this file's inode,
	//   set it as a "hardlink" in the tar header.
	// Otherwise, treat it as a regular file.
	if fi.Mode().IsRegular() && hasHardLinks {
		// If this file is NOT the first found hardlink to this inode,
		//   set the previously found hardlink as its 'Linkname'.
		if firstInode, ok := ta.hardLinks[inode]; ok {
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
			ta.hardLinks[inode] = name
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

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Create a tar archive of a directory.
func tarDir2(src *os.File) (dst *os.File, err error) {

	// Remember to re-open the tar archive after creation.
	defer func() {
		if err != nil {
			return
		}
		dst, err = os.Open(dst.Name())
	}()

	// Get file info for the source directory.
	var srcInfo os.FileInfo
	srcInfo, err = src.Stat()
	if err != nil {
		return
	}
	srcName := src.Name()
	baseName := filepath.Base(srcName)
	parent := filepath.Dir(srcName)

	// Make sure existing files are not overwritten.
	dstName := concat(baseName, ".tar")
	setDstName(&dstName)

	if !DoQuiet {
		fmt.Println(concat(srcName, "  >  ", dstName))
		defer fmt.Println()
	}

	// Create the destination file.
	dst, err = create(dstName, srcInfo.Mode())
	if err != nil {
		return
	}

	// Pipe the destination file through a *tarAppender.
	var dstWriter io.WriteCloser = dst
	ta := &tarAppender{
		tarWriter: tar.NewWriter(dstWriter),
		hardLinks: make(map[uint64]string),
	}

	// Remember to close the tarWriter.
	defer func() {
		err = ta.tarWriter.Close()
	}()

	// Walk through the directory.
	// Add a header to the tar archive for each file encountered.
	paths := getPaths(srcName)
	total := len(paths)

	var progress int
	var start time.Time

	for _, path := range paths {
		err = func() error {

			// Don't use the full path of the file in its header name.
			// Otherwise, the archive may extract an unnecessarily long path with
			//   anoying, empty diretories.
			// E.g., make an archive of '/home/me/Documents' extract to
			//   'Documents', not to '/home/me/Documents'.
			var name string
			name, err = filepath.Rel(parent, path)
			if err != nil {
				return err
			}

			// Get a header for the file.
			var hdr *tar.Header
			hdr, err = ta.getHeader(path, name)
			if err != nil {
				return err
			}

			// Write the header.
			if err = ta.write(hdr, path); err != nil {
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

			tooSoonToPrint := (int(time.Since(start)) < 500000)
			shouldPrint := (!tooSoonToPrint)
			needToPrint := (percent > 98)
			if !shouldPrint && !needToPrint {
				return nil
			}

			start = time.Now()

			// Print progress.
			fmt.Printf(
				"\r  %v%%   %v / %v files",
				percent, progress, total,
			)

			return nil
		}()
		if err != nil {
			return
		}
	}

	return
}

func (ta *tarAppender) write(hdr *tar.Header, path string) error {

	// Write the header.
	tw := ta.tarWriter
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

	// tb := ta.bufioWriter
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
	// Get the smallest directory name (top directory).
	topDir, err := findTopDirInArchive(src)
	if err != nil {
		return "", err
	}

	// Strip off the trailing '/'.
	topDir = topDir[0 : len(topDir)-1]

	// Make sure existing files are not overwritten.
	dstName := topDir
	setDstName(&dstName)

	// Re-open the readers.
	src, err = os.Open(src.Name())
	if err != nil {
		return "", err
	}
	tr := tar.NewReader(src)

	// Get file info.
	srcInfo, err := src.Stat()
	if err != nil {
		return "", err
	}
	total := uint64(srcInfo.Size())
	srcName := srcInfo.Name()

	// Extract the archive.
	print(concat(srcName, "  >  ", dstName))
	defer print()
	var progress uint64
	var outputLength int
	var start time.Time
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
		name = strings.Replace(name, topDir, dstName, 1)
		unusedFilename(&name)

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
		return "", fmt.Errorf("%v\nFailed to extract %v", err, srcName)
	}
	return dstName, nil
}

// Search a tar file for the top-level directory to be extracted.
func findTopDirInArchive(file *os.File) (topDir string, err error) {
	// Wrap a *tar.Reader around the *os.File.
	tr := tar.NewReader(file)
	defer func() {
		tr = nil
		file.Close()
	}()

	// Get the smallest directory name (top directory).
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

		// Set topDir to the very first header name.
		// Most likely, this will be the name of the top directory anyway.
		if topDir == "" {
			topDir = hdr.Name
		}

		// Skip non-directories.
		if hdr.Typeflag != tar.TypeDir {
			continue
		}

		// The top directory is the shortest path and has the shortest name.
		if len(hdr.Name) < len(topDir) {
			topDir = hdr.Name
		}
	}

	// If no names were found, the data is corrupt.
	if topDir == "" {
		err = fmt.Errorf("unable to read %v", file.Name())
	}

	return
}
