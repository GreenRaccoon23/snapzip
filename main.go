package main

import (
	"os"
	"sync"
)

var (
	// DoQuiet means no output
	DoQuiet bool
	// Files are the filepaths to be compressed/uncompressed
	Files []string
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

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-q":
			DoQuiet = true
		default:
			Files = append(Files, arg)
		}
	}

	if len(Files) > 1 {
		DoQuiet = true
	}

	if DoQuiet {
		print = printNoop
	}

	return
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
			chanErr <- compressOrDecompress(path)
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
func compressOrDecompress(path string) error {

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

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
