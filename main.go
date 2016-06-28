package main

import (
	"flag"
	"os"
	"sync"

	"github.com/GreenRaccoon23/slices"
)

var (
	DoQuiet bool
	Files   []string
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

	if len(Files) > 1 {
		DoQuiet = true
	}

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

	return nil
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
