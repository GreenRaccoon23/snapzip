package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GreenRaccoon23/slices"
	"github.com/golang/snappy"
)

var (
	DoQuiet bool
	Files   []string
	// doBring         bool
	// doSingleArchive bool
	// dstArchive      string

	print = fmt.Println
)

func init() {
	if helpRequested() {
		printHelp()
		os.Exit(0)
	}
	setGlobalVars()
}

// Check whether the user requested help.
func helpRequested() bool {

	if tooFewArgs := (len(os.Args) < 2); tooFewArgs {
		return true
	}

	switch os.Args[1] {
	case "-h", "h", "help", "--help", "-H", "H", "HELP", "--HELP", "-help", "--h", "--H":
		return true
	}

	return false

}

// Print help and exit with a status code.
func printHelp() {
	fmt.Printf(
		"%s\n",
		`snapzip
Usage: snapzip [option ...] [file ...]
Description:
    Compress/uncompress files to/from snappy archives.
Options:
    -q        Do not show any output
Notes:
    This program automatically determines whether a file should be
      compressed or decompressed.
    This program can also compress directories;
      they are added to a tar archive prior to compression.`,
	)
}

// Parse user arguments and modify global variables accordingly.
func setGlobalVars() {

	// Parse commandline arguments.
	//flag.StringVar(&dstArchive, "a", "", "")
	flag.BoolVar(&DoQuiet, "q", false, "")
	flag.Parse()

	// Modify global variables based on commandline arguments.
	Files = os.Args[1:]
	// if !DoQuiet && dstArchive == "" {
	//  return
	// }

	if DoQuiet {
		boolArgs := []string{"-q"}
		Files = slices.Filter(Files, boolArgs...)
		print = printNoop
	}
	// if dstArchive != "" {
	//  // doSingleArchive = true
	//  Files = slices.Filter(Files, dstArchive)
	// }
	return
}

// Empty print func for when 'DoQuiet' is set.
func printNoop(x ...interface{}) (int, error) {
	return 0, nil
}

func main() {
	/*
	   if doSingleArchive {

	   }
	*/

	var wg sync.WaitGroup
	lenTrgtFiles := len(Files)
	wg.Add(lenTrgtFiles)

	for _, f := range Files {
		go func(f string) {
			defer wg.Done()
			//f = filepath.Clean(f)
			err := analyze(f)
			if err != nil && !DoQuiet {
				print(err)
			}
		}(f)
	}

	wg.Wait()
}

// Concatenate strings.
func concat(slc ...string) string {
	var b bytes.Buffer
	defer b.Reset()
	for _, s := range slc {
		b.WriteString(s)
	}
	return b.String()
}

// Determine whether a file should be compressed, uncompressed, or
//   added to a tar archive and then compressed.
func analyze(filename string) error {

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func(f *os.File) { f.Close() }(file)

	switch {

	// If the file is a snappy file, uncompress it.
	case isSz(file):
		return unsnapAndUntar(file)

	// If the file is a directory, tar it before compressing it.
	// (Simultaneously compressing and tarring the file
	//   results in a much lower compression ratio.)
	case isDir(file):
		return tarAndSnap(file)

	// If the file is any other type, compress it.
	default:
		_, err := snap(file)
		return err
	}

	return nil
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

	total := 10
	offset := int64(0)

	chunk := make([]byte, total)
	nRead, _ := file.ReadAt(chunk, offset)
	if nRead < total {
		return false
	}

	snappySignature := []byte{255, 6, 0, 0, 115, 78, 97, 80, 112, 89}
	return bytes.Equal(chunk, snappySignature)
}

// Check a file's contents for a tar file signature.
func isTar(file *os.File) bool {

	total := 5
	offset := int64(257)

	chunk := make([]byte, total)
	nRead, _ := file.ReadAt(chunk, offset)
	if nRead < total {
		return false
	}

	tarSignature := []byte{117, 115, 116, 97, 114}
	return bytes.Equal(chunk, tarSignature)
}

// Uncompress a file.
// Then, if the uncompressed file is a tar archive, extract it as well.
func unsnapAndUntar(file *os.File) error {

	// Uncompress it.
	uncompressed, err := unsnap(file)
	if err != nil {
		return err
	}

	// If the uncompressed file is not a tar archive, don't try to untar it.
	if !isTar(uncompressed) {
		return nil
	}

	// Remember to remove the uncompressed tar archive.
	defer func() {
		os.Remove(uncompressed.Name())
	}()

	return untar(uncompressed)
}

// Make a temporary tar archive of a file and then compress it.
// (Simultaneously compressing and tarring the file
//  results in a much lower compression ratio.)
// Remove the temporary tar archive if no errors occur.
func tarAndSnap(file *os.File) error {

	// Tar it.
	tmpArchive, err := tarDir(file)
	if err != nil {
		return err
	}

	// Remember to close and remove the temporary tar archive.
	defer tmpArchive.Close()
	defer func() {
		if err == nil {
			os.Remove(tmpArchive.Name())
		}
	}()

	// Compress it.
	_, err = snap(tmpArchive)
	return err
}

// Create a file if it doesn't exist. Otherwise, just open it.
func create(filename string, mode os.FileMode) (*os.File, error) {
	// getUnusedFilename(&filename)
	file, err := os.OpenFile(
		filename,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		mode,
	)
	return file, err
}

// Modify a filename to one that has not been used by the system.
func getUnusedFilename(filename *string) {

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

// Credits to jimt from here:
// https://stackoverflow.com/questions/22421375/how-to-print-the-bytes-while-the-file-is-being-downloaded-golang
//
// passthru wraps an existing io.Reader or io.Writer.
// It simply forwards the Read() or Write() call, while displaying
// the results from individual calls to it.
type passthru struct {
	io.Reader
	io.Writer
	total        uint64 // Total # of bytes transferred
	length       uint64 // Expected length
	progress     float64
	outputLength int
}

// Write 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
// NOTE: Print a new line after any commands which use this io.Reader.
func (pt *passthru) Read(b []byte) (int, error) {
	n, err := pt.Reader.Read(b)
	if n <= 0 || DoQuiet {
		return n, err
	}
	pt.total += uint64(n)

	percentage := float64(pt.total) / float64(pt.length) * float64(100)
	percent := int(percentage)
	if percentage-pt.progress < 1 && percent < 99 {
		return n, err
	}

	total := sizeLabel(pt.total)
	goal := sizeLabel(pt.length)

	output := fmt.Sprintf(
		"  %v%%   %v / %v",
		percent, total, goal,
	)
	outputLength := len(output)
	if outputLength > pt.outputLength {
		pt.outputLength = outputLength
	}

	fmt.Printf("\r%v", strings.Repeat(" ", pt.outputLength))
	fmt.Printf("\r%v", output)
	pt.progress = percentage

	return n, err
}

// Write 'overrides' the underlying io.Writer's Write method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
// NOTE: Print a new line after any commands which use this io.Writer.
func (pt *passthru) Write(b []byte) (int, error) {
	n, err := pt.Writer.Write(b)
	if n <= 0 || DoQuiet {
		return n, err
	}

	pt.total += uint64(n)
	percentage := float64(pt.total) / float64(pt.length) * float64(100)
	percent := int(percentage)
	if percentage-pt.progress < 1 && percent < 99 {
		return n, err
	}

	total := sizeLabel(pt.total)
	goal := sizeLabel(pt.length)
	ratio := fmt.Sprintf("%.3f", float64(pt.total)/float64(pt.length))

	output := fmt.Sprintf(
		"  %v%%   %v / %v = %v",
		percent, total, goal, ratio,
	)

	outputLength := len(output)
	if outputLength > pt.outputLength {
		pt.outputLength = outputLength
	}
	fmt.Printf("\r%v", strings.Repeat(" ", pt.outputLength))
	fmt.Printf("\r%v", output)
	pt.progress = percentage

	return n, err
}

// Slight variation of bytefmt.ByteSize() from:
// https://github.com/pivotal-golang/bytefmt/blob/master/bytes.go
const (
	BYTE     = 1.0
	KIBIBYTE = 1000 * BYTE
	MEBIBYTE = 1000 * KIBIBYTE
	GIBIBYTE = 1000 * MEBIBYTE
	TEBIBYTE = 1000 * GIBIBYTE
)

func sizeLabel(byteSize uint64) string {
	unit := ""
	value := float64(byteSize)

	switch {
	case byteSize >= TEBIBYTE:
		unit = "TiB"
		value = value / TEBIBYTE
	case byteSize >= GIBIBYTE:
		unit = "GiB"
		value = value / GIBIBYTE
	case byteSize >= MEBIBYTE:
		unit = "MiB"
		value = value / MEBIBYTE
	case byteSize >= KIBIBYTE:
		unit = "KiB"
		value = value / KIBIBYTE
	case byteSize >= BYTE:
		unit = "Bytes"
	case byteSize == 0:
		return "0"
	}

	stringValue := fmt.Sprintf("%.1f", value)
	return concat(stringValue, " ", unit)
}

// Decompress a snappy archive.
func unsnap(src *os.File) (dst *os.File, err error) {
	srcInfo, err := src.Stat()
	if err != nil {
		return
	}
	srcName := srcInfo.Name()

	// Make sure existing files are not overwritten.
	dstName := strings.TrimSuffix(srcName, ".sz")

	getUnusedFilename(&dstName)
	print(concat(srcName, "  >  ", dstName))

	// Create the destination file.
	dst, err = create(dstName, srcInfo.Mode())
	if err != nil {
		return
	}
	// Remember to re-open the uncompressed file after it has been written.
	defer func() {
		if err == nil {
			dst, err = os.Open(dstName)
		}
	}()

	pt := &passthru{
		Reader: src,
		length: uint64(srcInfo.Size()),
	}
	defer func() { pt.Reader = nil }()
	szr := snappy.NewReader(pt)
	defer szr.Reset(nil)

	defer print()
	_, err = io.Copy(dst, szr)
	return
}

// Extract a tar archive.
func untar(file *os.File) error {
	// Get the smallest directory name (top directory).
	topDir, err := findTopDirInArchive(file)
	if err != nil {
		return err
	}

	// Strip off the trailing '/'.
	topDir = topDir[0 : len(topDir)-1]

	// Make sure existing files are not overwritten.
	dstName := topDir
	getUnusedFilename(&dstName)

	// Re-open the readers.
	file, err = os.Open(file.Name())
	if err != nil {
		return err
	}
	tr := tar.NewReader(file)

	// Get file info.
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	total := uint64(fi.Size())
	name := fi.Name()

	// Extract the archive.
	print(concat(name, "  >  ", dstName))
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
		getUnusedFilename(&name)

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
		return fmt.Errorf("%v\nFailed to extract %v", err, name)
	}
	return nil
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
		err = fmt.Errorf("Unable to read %v. Data is corrupt.", file.Name())
	}

	return
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
		i += 1
		return nil
	})
	return
}

// Compress a file to a snappy archive.
func snap(src *os.File) (dst *os.File, err error) {
	// Remember to re-open the destination file after compression.
	defer func() {
		dst, err = os.Open(dst.Name())
	}()

	// Get file info.
	srcInfo, err := src.Stat()
	if err != nil {
		return
	}
	srcTotal := uint64(srcInfo.Size())
	srcName := src.Name()

	// Make sure existing files are not overwritten.
	dstName := concat(srcName, ".sz")
	getUnusedFilename(&dstName)
	print(concat(srcName, "  >  ", dstName))

	// Create the destination file.
	dst, err = create(dstName, srcInfo.Mode())
	if err != nil {
		return
	}

	// Set up a *passthru writer in order to print progress.
	pt := &passthru{
		Writer: dst,
		length: uint64(srcTotal),
	}
	defer func() { pt.Writer = nil }()

	// Wrap a *snappy.Writer around the *passthru method.
	sz := snappy.NewWriter(pt)
	defer sz.Reset(nil)

	// Write the source file's contents to the new snappy file.
	_, err = snapCopy(sz, src)
	print()
	if err != nil {
		return
	}
	return
}

// snappy.maxUncompressedChunkLen
const SNAPPY_MAX_UNCOMPRESSED_CHUNK_LEN = 65536

// Read data from a source file,
//   compress the data,
//   and write it to a *snappy.Writer destination file.
// Serves as a makeshift snappy replacement for io.Copy
//   as long as the source Reader is an *os.File
//   and the destination Writer is a *snappy.Writer.
func snapCopy(sz *snappy.Writer, src *os.File) (int64, error) {

	buf := make([]byte, SNAPPY_MAX_UNCOMPRESSED_CHUNK_LEN)
	return io.CopyBuffer(sz, src, buf)

	// Slow and dangerous. Kept for testing purposes.
	// srcContents, err := ioutil.ReadAll(src)
	// if err != nil {
	// 	return
	// }
	// totalWritten, err = sz.Write(srcContents)
	// return int64(totalWritten), err
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
type tarAppender struct {
	tarWriter   *tar.Writer
	bufioWriter *bufio.Writer
	// Map inodes to hardlinks.
	hardLinks map[uint64]string
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Create a tar archive of a directory.
func tarDir(dir *os.File) (dst *os.File, err error) {
	// Remember to re-open the tar archive after creation.
	defer func() {
		if err != nil {
			return
		}
		dst, err = os.Open(dst.Name())
	}()

	// Get file info for the source directory.
	dirInfo, err := dir.Stat()
	if err != nil {
		return
	}
	dirName := dir.Name()
	baseName := filepath.Base(dirName)
	parent := filepath.Dir(dirName)

	// Make sure existing files are not overwritten.
	dstName := concat(baseName, ".tar")
	getUnusedFilename(&dstName)

	if !DoQuiet {
		fmt.Println(concat(dirName, "  >  ", dstName))
		defer fmt.Println()
	}

	// Create the destination file.
	dst, err = create(dstName, dirInfo.Mode())
	if err != nil {
		return
	}

	// Pipe the destination file through a *tarAppender.
	var dstWriter io.WriteCloser = dst
	ta := &tarAppender{
		tarWriter:   tar.NewWriter(dstWriter),
		bufioWriter: bufio.NewWriter(nil),
		hardLinks:   make(map[uint64]string),
	}

	// Remember to close the tarWriter.
	defer func() {
		err = ta.tarWriter.Close()
	}()

	// Walk through the directory.
	// Add a header to the tar archive for each file encountered.
	var total, progress int
	var start time.Time
	if !DoQuiet {
		total = dirSize(dirName)
	}
	err = filepath.Walk(dirName, func(path string, fi os.FileInfo, err error) error {
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

		// Add a header for the file.
		if err = ta.add(path, name); err != nil {
			return err
		}

		// Skip printing progress if user requested it.
		if DoQuiet {
			return nil
		}

		// Make sure progress isn't outputted too quickly
		//   for the console.
		progress += 1
		percent := int(float64(progress) / float64(total) * float64(100))
		if int(time.Since(start)) < 100000 && percent < 100 {
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

	return
}

// https://github.com/docker/docker/blob/master/pkg/archive/archive.go
// Add a file [as a header] to a tar archive.
func (ta *tarAppender) add(path, name string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	// If the file is a symlink, find its target.
	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	// Create the tar header.
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}

	// Set the header name.
	// If the file is a directory, add a trailing "/".
	if fi.Mode()&os.ModeDir != 0 {
		fmtDir(&name)
	}
	hdr.Name = name

	// Check if the file has hard links.
	hasHardLinks, inode, err := tarSetHeader(hdr, fi.Sys())
	if err != nil {
		return err
	}

	// If any other regular files link to the same inode as this file,
	//   prepare to treat it as a "hardlink" in the header.
	// If the tar archive contains another hardlink to this file's inode,
	//   set it as a "hardlink" in the tar header.
	// Otherwise, treat it as a regular file.
	if fi.Mode().IsRegular() && hasHardLinks {
		// If this file is NOT the first found hardlink to this inode,
		//   set the previously found hardlink as its 'Linkname'.
		if oldpath, ok := ta.hardLinks[inode]; ok {
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = oldpath
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

	// Write the header.
	tw := ta.tarWriter
	if err = tw.WriteHeader(hdr); err != nil {
		return err
	}

	// If the file is a regular one,
	//   i.e., not a symlink, directory, or hardlink,
	//   write the file's contents to the buffer.
	if hdr.Typeflag == tar.TypeReg {
		tb := ta.bufioWriter
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		tb.Reset(tw)
		defer tb.Reset(nil)
		_, err = io.Copy(tb, file)
		file.Close()
		if err != nil {
			return err
		}
		err = tb.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}
