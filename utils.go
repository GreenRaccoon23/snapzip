package main

import (
	"bytes"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Concatenate strings.
func concat(slc ...string) string {
	var b bytes.Buffer
	defer b.Reset()
	for _, s := range slc {
		b.WriteString(s)
	}
	return b.String()
}

// Check whether a file is a directory.
func isDir(file *os.File) bool {
	fi, err := file.Stat()
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// Check a file's contents for a snappy file signature.
func isSz(file *os.File) bool {

	snappySignature := []byte{255, 6, 0, 0, 115, 78, 97, 80, 112, 89}
	offset := int64(0)

	lenSignature := len(snappySignature)
	chunk := make([]byte, lenSignature)

	nRead, _ := file.ReadAt(chunk, offset)
	if nRead < lenSignature {
		return false
	}

	return bytes.Equal(chunk, snappySignature)
}

// Check a file's contents for a tar file signature.
func isTar(file *os.File) bool {

	tarSignature := []byte{117, 115, 116, 97, 114}
	offset := int64(257)

	lenSignature := len(tarSignature)
	chunk := make([]byte, lenSignature)

	nRead, _ := file.ReadAt(chunk, offset)
	if nRead < lenSignature {
		return false
	}

	return bytes.Equal(chunk, tarSignature)
}

// Create a file if it doesn't exist. Otherwise, just open it.
func create(filename string, mode os.FileMode) (*os.File, error) {
	// unusedFilename(&filename)
	file, err := os.OpenFile(
		filename,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		mode,
	)
	return file, err
}

func setDstName(dstName *string) {
	if DstDir != "" {
		*dstName = path.Join(DstDir, *dstName)
	}
	unusedFilename(dstName)
}

// Modify a filename to one that has not been used by the system.
func unusedFilename(filename *string) {

	if !exists(*filename) {
		return
	}

	base, ext := splitExt(*filename)
	// Go's date of birth. :)
	for i := 1; i < 20091110; i++ {
		// May change this convention later,
		//   since bash doesn't like the parentheses.
		testname := concat(base, "(", strconv.Itoa(i), ")", ext)
		if exists(testname) {
			continue
		}
		*filename = testname
		return
	}
}

// Split the extension off a filename.
// Return the basename and the extension.
func splitExt(filename string) (base, ext string) {

	base = filepath.Clean(filename)

	for {

		testext := filepath.Ext(base)

		if !isExtension(testext) {
			return
		}

		ext = concat(testext, ext)
		base = strings.TrimSuffix(base, testext)
	}
}

func isExtension(ext string) bool {

	if ext == "" {
		return false
	}

	if recognizedByFS := (mime.TypeByExtension(ext) != ""); recognizedByFS {
		return true
	}

	switch ext {
	case ".tar", ".sz", ".tar.sz":
		return true
	}

	return false
}

// Check whether a file exists.
func exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

// Append a "/" to a string if it doesn't have one already.
func fmtDir(name *string) {
	if *name == "." || *name == "" {
		return
	}
	s := string(filepath.Separator)
	if !strings.HasSuffix(*name, s) {
		*name = concat(*name, s)
	}
}

// Return the total size in bytes and number of files under a directory.
func dirSize(dir string) (i int) {
	filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		i++
		return nil
	})
	return
}

// Return a slice of all the paths under a directory.
func getPaths(dir string) (paths []string) {
	filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		paths = append(paths, path)
		return nil
	})
	return
}
