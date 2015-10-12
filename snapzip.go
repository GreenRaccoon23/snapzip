package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/golang/snappy"
)

var (
	doBring         bool
	doSingleArchive bool
	doQuiet         bool
	dstArchive      string
	trgtFiles       []string
)

func init() {
	chkHelp()
	flags()
}

// Check whether the user requested help.
func chkHelp() {
	if len(os.Args) < 2 {
		return
	}

	switch os.Args[1] {
	case "-h", "h", "help", "--help", "-H", "H", "HELP", "--HELP", "-help", "--h", "--H":
		help(0)
	}

}

// Print help and exit with a status code.
func help(status int) {
	defer os.Exit(status)
	fmt.Printf(
		//"%s\n\n  %s\n\n  %s\n%s\n\n  %s\n%s\n\n  %s\n%s\n%s\n%s\n%s\n",
		"%s\n\n  %s\n\n  %s\n%s\n\n  %s\n%s\n\n  %s\n%s\n%s\n%s\n%s\n",
		"snapzip",
		"Usage: snapzip [option ...] [file ...]",
		"Description:",
		"    Compress/uncompress files to/from snappy archives.",
		"Options:",
		//"   -a <name>    Compress all files into a single snappy archive.",
		//"                (default is to compress each file individually)",
		"   -q           Do not show any output",
		"Notes:",
		"    This program automatically determines whether a file should be",
		"      compressed or decompressed.",
		"    This program can also compress directories;",
		"      they are added to a tar archive prior to compression.",
	)
}

// Parse user arguments and modify global variables accordingly.
func flags() {
	// Program requires at least one user argument.
	// Print help and exit with status 1 if none have been received.
	if len(os.Args) < 2 {
		help(1)
	}

	// Parse commandline arguments.
	//flag.StringVar(&dstArchive, "a", "", "")
	flag.BoolVar(&doQuiet, "q", false, "")
	flag.Parse()

	// Modify global variables based on commandline arguments.
	trgtFiles = os.Args[1:]
	if !doQuiet && dstArchive == "" {
		return
	}

	if doQuiet {
		bools := []string{"-q"}
		trgtFiles = filter(trgtFiles, bools...)
	}
	if dstArchive != "" {
		doSingleArchive = true
		trgtFiles = filter(trgtFiles, dstArchive)
	}
	return
}

// Remove elements in a slice (if they exist).
// Only remove EXACT matches.
func filter(slc []string, args ...string) (filtered []string) {
	for _, s := range slc {
		if slcHas(args, s) {
			continue
		}
		filtered = append(filtered, s)
	}
	return
}

// Check whether a slice contains a string.
// Only return true if an element in the slice EXACTLY matches the string.
// If testing for more than one string,
//   return true if ANY of them match an element in the slice.
func slcHas(slc []string, args ...string) bool {
	for _, s := range slc {
		for _, a := range args {
			if s == a {
				return true
			}
		}
	}
	return false
}

func main() {
	/*
		if doSingleArchive {

		}
	*/
	for _, f := range trgtFiles {
		//f = filepath.Clean(f)
		err := analyze(f)
		if err == nil || doQuiet {
			continue
		}
		print(err)
	}
}

// Pass to fmt.Println() unless quiet mode is active.
func print(a ...interface{}) {
	if doQuiet {
		return
	}
	fmt.Println(a...)
}

// Print a newline unless quiet mode is active.
func println() {
	if doQuiet {
		return
	}
	fmt.Println()
}

// Pass to fmt.Printf() unless quiet mode is active.
func printf(format string, a ...interface{}) {
	if doQuiet {
		return
	}
	fmt.Printf(format, a...)
}
func chkerr(err error) {
	if err == nil {
		return
	}
	if doQuiet {
		os.Exit(1)
	}
	log.Fatal(err)
}

// Concatenate strings.
func concat(slc ...string) string {
	b := bytes.NewBuffer(nil)
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
	chkerr(err)
	defer func(f *os.File) { f.Close() }(file)

	switch {

	// If the file is a snappy file, uncompress it.
	case isSz(file):
		// Uncompress it.
		uncompressed, err := unsnap(file)
		chkerr(err)

		// If the uncompressed file is a tar archive, untar it.
		if !isTar(uncompressed) {
			return nil
		}
		// Remember to remove the uncompressed tar archive.
		defer func() {
			os.Remove(uncompressed.Name())
		}()
		err = untar(uncompressed)
		chkerr(err)
		return nil

	// If the file is a directory, tar it before compressing it.
	// (Simultaneously compressing and tarring the file
	//   results in a much lower compression ratio.)
	case isDir(file):
		// Tar it.
		file, err = tarDir(file)
		chkerr(err)
		// Remove to close and remove the temporary tar archive.
		defer func() {
			file.Close()
			if err == nil {
				os.Remove(file.Name())
			}
		}()
		fallthrough

	// If the file is any other type, compress it.
	default:
		// Compress it.
		_, err := snap(file)
		chkerr(err)
		break
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
	bytes := make([]byte, total)
	n, _ := file.ReadAt(bytes, 0)
	if n < total {
		return false
	}

	szSig := []byte{255, 6, 0, 0, 115, 78, 97, 80, 112, 89}
	for i, b := range bytes {
		if b != szSig[i] {
			return false
		}
	}
	return true
}

// Check a file's contents for a tar file signature.
func isTar(file *os.File) bool {
	bytes := make([]byte, 5)
	n, _ := file.ReadAt(bytes, 257)
	if n < 5 {
		return false
	}

	tarSig := []byte{117, 115, 116, 97, 114}
	for i, b := range bytes {
		if b != tarSig[i] {
			return false
		}
	}
	return true
}

// Create a file if it doesn't exist. Otherwise, just open it.
func create(filename string, mode os.FileMode) (*os.File, error) {
	genUnusedFilename(&filename)
	file, err := os.OpenFile(
		filename,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		mode,
	)
	return file, err
}

// Modify a filename to one that has not been used by the system.
func genUnusedFilename(filename *string) {
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
		if testext == "" {
			return
		}
		if mime.TypeByExtension(testext) == "" {
			switch testext {
			case ".tar", ".sz", ".tar.sz":
				break
			default:
				return
			}
		}
		ext = concat(testext, ext)
		base = strings.TrimSuffix(base, testext)
	}
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
	if n <= 0 || doQuiet {
		return n, err
	}
	pt.total += uint64(n)

	percentage := float64(pt.total) / float64(pt.length) * float64(100)
	percent := int(percentage)
	if percentage-pt.progress < 1 && percent < 99 {
		return n, err
	}

	total := fmtSize(pt.total)
	goal := fmtSize(pt.length)

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
	if n <= 0 || doQuiet {
		return n, err
	}

	pt.total += uint64(n)
	percentage := float64(pt.total) / float64(pt.length) * float64(100)
	percent := int(percentage)
	if percentage-pt.progress < 1 && percent < 99 {
		return n, err
	}

	total := fmtSize(pt.total)
	goal := fmtSize(pt.length)
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

func fmtSize(bytes uint64) string {
	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= TEBIBYTE:
		unit = "TiB"
		value = value / TEBIBYTE
	case bytes >= GIBIBYTE:
		unit = "GiB"
		value = value / GIBIBYTE
	case bytes >= MEBIBYTE:
		unit = "MiB"
		value = value / MEBIBYTE
	case bytes >= KIBIBYTE:
		unit = "KiB"
		value = value / KIBIBYTE
	case bytes >= BYTE:
		unit = "Bytes"
	case bytes == 0:
		return "0"
	}

	stringValue := fmt.Sprintf("%.1f", value)
	return concat(stringValue, " ", unit)
}

// Decompress a snappy archive.
func unsnap(src *os.File) (dst *os.File, err error) {
	srcInfo, err := src.Stat()
	chkerr(err)
	srcName := srcInfo.Name()

	// Make sure existing files are not overwritten.
	dstName := strings.TrimSuffix(srcName, ".sz")
	genUnusedFilename(&dstName)
	print(concat(srcName, "  >  ", dstName))

	// Create the destination file.
	dst, err = create(dstName, srcInfo.Mode())
	chkerr(err)
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

	defer println()
	_, err = io.Copy(dst, szr)
	chkerr(err)
	return
}

// Extract a tar archive.
func untar(file *os.File) error {
	// Get file info.
	fi, err := file.Stat()
	chkerr(err)
	total := uint64(fi.Size())
	name := fi.Name()

	// Wrap a *tar.Reader around the *os.File.
	tr := tar.NewReader(file)

	// Get the smallest directory name (top directory).
	var topDir string
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()
		// Break if the end of the tar archive has been reached.
		if err == io.EOF {
			err = nil
			break
		}
		// Set topDir to the very first header name.
		// Most likely, this will be the name of the top directory anyway.
		if topDir == "" {
			topDir = hdr.Name
		}
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
		err = fmt.Errorf("Unable to read %v. Data is corrupt.", name)
		return err
	}
	// Strip off the trailing '/'.
	topDir = topDir[0 : len(topDir)-1]

	// Make sure existing files are not overwritten.
	dstName := topDir
	genUnusedFilename(&dstName)

	// Re-open the readers.
	tr = tar.NewReader(nil)
	file.Close()
	file, err = os.Open(file.Name())
	chkerr(err)
	tr = tar.NewReader(file)

	// Extract the archive.
	print(concat(name, "  >  ", dstName))
	defer println()
	var progress uint64
	var outputLength int
	var start time.Time
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()
		// Break if the end of the tar archive has been reached.
		if err == io.EOF {
			err = nil
			break
		}
		chkerr(err)

		// Make sure existing files are not overwritten.
		name := hdr.Name
		name = strings.Replace(name, topDir, dstName, 1)
		genUnusedFilename(&name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			// Extract a directory.
			err = os.MkdirAll(name, os.FileMode(hdr.Mode))
			chkerr(err)

		case tar.TypeReg, tar.TypeRegA:
			// Extract a regular file.
			var w *os.File
			w, err = create(name, os.FileMode(hdr.Mode))
			chkerr(err)
			_, err = io.Copy(w, tr)
			chkerr(err)
			w.Close()

		case tar.TypeLink:
			// Extract a hard link.
			err = os.Link(hdr.Linkname, name)
			chkerr(err)

		case tar.TypeSymlink:
			// Extract a symlink.
			err = os.Symlink(hdr.Linkname, name)
			chkerr(err)

		default:
			// If the Typeflag is missing, the data is probably corrupt.
			// Just skip to the next one anyway if this happens.
			continue
		}

		// Print progress.
		if doQuiet || hdr.Size == int64(0) {
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
			percent, fmtSize(progress), fmtSize(total),
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
	chkerr(err)
	srcTotal := uint64(srcInfo.Size())
	srcName := src.Name()

	// Make sure existing files are not overwritten.
	dstName := concat(srcName, ".sz")
	genUnusedFilename(&dstName)
	print(concat(srcName, "  >  ", dstName))

	// Create the destination file.
	dst, err = create(dstName, srcInfo.Mode())
	chkerr(err)

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
	println()
	chkerr(err)
	return
}

// snappy.maxUncompressedChunkLen
const snappyStep = 65536

// Read data from a source file,
//   compress the data,
//   and write it to a *snappy.Writer destination file.
// Serves as a makeshift snappy replacement for io.Copy
//   as long as the source Reader is an *os.File
//   and the destination Writer is a *snappy.Writer.
func snapCopy(sz *snappy.Writer, src *os.File) (written int64, err error) {
	var tab int
	for {
		// Read byte chunk.
		bytes := make([]byte, snappyStep)
		nr, _ := src.ReadAt(bytes, int64(tab))
		// Stop if nothing was read.
		if nr == 0 {
			break
		}

		// Write byte chunk.
		nw, err := sz.Write(bytes)
		written += int64(nw)
		if err != nil {
			break
		}

		// Stop if the byte chunk wasn't the max size:
		//   EOF has been reached.
		if nr < snappyStep {
			break
		}
		tab += snappyStep
	}
	return
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
		chkerr(err)
	}()

	// Get file info for the source directory.
	dirInfo, err := dir.Stat()
	chkerr(err)
	dirName := dir.Name()
	baseName := filepath.Base(dirName)
	parent := filepath.Dir(dirName)

	// Make sure existing files are not overwritten.
	dstName := concat(baseName, ".tar")
	genUnusedFilename(&dstName)
	print(concat(dirName, "  >  ", dstName))
	defer println()

	// Create the destination file.
	dst, err = create(dstName, dirInfo.Mode())
	chkerr(err)

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
		chkerr(err)
	}()

	// Walk through the directory.
	// Add a header to the tar archive for each file encountered.
	var total, progress int
	var start time.Time
	if !doQuiet {
		total = dirSize(dirName)
	}
	err = filepath.Walk(dirName, func(path string, fi os.FileInfo, err error) error {
		// Quit if any errors occur.
		chkerr(err)

		// Don't use the full path of the file in its header name.
		// Otherwise, the archive may extract an unnecessarily long path with
		//   anoying, empty diretories.
		// E.g., make an archive of '/home/me/Documents' extract to
		//   'Documents', not to '/home/me/Documents'.
		name, err := filepath.Rel(parent, path)
		chkerr(err)

		// Add a header for the file.
		err = ta.add(path, name)
		chkerr(err)

		// Skip printing progress if user requested it.
		if doQuiet {
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
	chkerr(err)

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
	nlink, inode, err := tarSetHeader(hdr, fi.Sys())
	if err != nil {
		return err
	}

	// If any other regular files link to the same inode as this file,
	//   prepare to treat it as a "hardlink" in the header.
	// If the tar archive contains another hardlink to this file's inode,
	//   set it as a "hardlink" in the tar header.
	// Otherwise, treat it as a regular file.
	if fi.Mode().IsRegular() && nlink > 1 {
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

// https://github.com/docker/docker/blob/master/pkg/archive/archive_unix.go
// Add a file's device major and minor numbers
//   to the file's header within a tar archive.
// Return the file's inode and the number of hardlinks to that inode.
func tarSetHeader(hdr *tar.Header, stat interface{}) (nlink uint32, inode uint64, err error) {
	s, ok := stat.(*syscall.Stat_t)
	if !ok {
		err = fmt.Errorf("cannot convert stat value to syscall.Stat_t")
		return
	}

	nlink = uint32(s.Nlink)
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
