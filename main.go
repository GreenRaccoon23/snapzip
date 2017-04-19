package main

import (
	"os"
	"path"
	"strings"
	"sync"
)

var (
	// DoQuiet means no output
	DoQuiet bool
	// Files are the filepaths to be compressed/uncompressed
	Files []string
	// DstDir is the optional location to place compressed/uncompressed files
	DstDir string
	// doBring         bool
	// doSingleArchive bool
	// dstArchive      string
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

// Parse user arguments and modify global variables accordingly.
func setGlobalVars() {

	max := len(os.Args)

	for i := 1; i < max; i++ {
		arg := os.Args[i]

		switch arg {
		case "-q":
			DoQuiet = true
		case "--dst-dir":
			i, arg = nextArg(i)
			DstDir = arg
		default:
			if strings.HasPrefix(arg, "--dst-dir=") {
				DstDir = strings.Replace(arg, "--dst-dir=", "", 1)
			} else {
				Files = append(Files, arg)
			}
		}
	}

	if len(Files) > 1 {
		DoQuiet = true
	}

	if DoQuiet {
		print = printNoop
	}

	if DstDir != "" {
		DstDir = path.Clean(DstDir)
	}

	return
}

func nextArg(i int) (int, string) {
	i++
	if i >= len(os.Args) {
		printHelp()
		os.Exit(2)
	}
	arg := os.Args[i]
	return i, arg
}

func main() {

	// if doSingleArchive {
	// }

	editFiles()
}

func editFiles() {

	lenFiles := len(Files)

	var wg sync.WaitGroup
	wg.Add(lenFiles)

	chanErr := make(chan error, lenFiles)

	for _, path := range Files {
		go func(path string) {
			defer wg.Done()
			//path = filepath.Clean(path)
			dstName, err := compressOrDecompress(path)
			print(dstName)
			chanErr <- err
		}(path)
	}

	wg.Wait()
	close(chanErr)

	for err := range chanErr {
		if err != nil {
			print(err)
		}
	}
}

// Determine whether a file should be compressed, uncompressed, or
//   added to a tar archive and then compressed.
func compressOrDecompress(path string) (string, error) {

	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()

	var dstName string

	switch {

	// If `src` is a snappy file, uncompress it.
	case isSz(src):
		dstName, err = unsnapAndUntar(src)

	// If `src` is a directory, tar it before compressing it.
	// (Simultaneously compressing and tarring the file
	//   results in a much lower compression ratio.)
	case isDir(src):
		dstName, err = tarAndSnap(src)

	// If `src` is any other type, compress it.
	default:
		dstName, err = snap(src)
	}

	return dstName, err
}

// Uncompress a file.
// Then, if the uncompressed file is a tar archive, extract it as well.
func unsnapAndUntar(src *os.File) (string, error) {

	// Uncompress it.
	unsnappedName, err := unsnap(src)
	if err != nil {
		return "", err
	}

	unsnapped, err := os.Open(unsnappedName)
	if err != nil {
		return "", err
	}

	// If the unsnapped file is not a tar archive, don't try to untar it.
	if !isTar(unsnapped) {
		return "", nil
	}

	// Remember to remove the unsnapped tar archive.
	defer func() {
		unsnapped.Close()
		os.Remove(unsnappedName)
	}()

	dstName, err := untar(unsnapped)
	if err != nil {
		return "", err
	}

	return dstName, err
}

// Make a temporary tar archive of a file and then compress it.
// (Simultaneously compressing and tarring the file
//  results in a much lower compression ratio.)
// Remove the temporary tar archive if no errors occur.
func tarAndSnap(src *os.File) (string, error) {

	// Tar it.
	tarredName, err := tarDir(src)
	if err != nil {
		return "", err
	}

	tarred, err := os.Open(tarredName)
	if err != nil {
		return "", err
	}

	// Remember to close and remove the temporary tar archive.
	defer func() {
		tarred.Close()
		if err == nil {
			os.Remove(tarredName)
		}
	}()

	// Compress it.
	dstName, err := snap(tarred)
	if err != nil {
		return "", err
	}

	return dstName, nil
}
