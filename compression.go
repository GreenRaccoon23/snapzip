package main

import (
	"io"
	"os"
	"strings"

	"github.com/golang/snappy"
)

// Compress a file to a snappy archive.
func snap(src *os.File) (string, error) {
	// Get file info.
	srcInfo, err := src.Stat()
	if err != nil {
		return "", err
	}
	srcName := src.Name()

	// Make sure existing files are not overwritten.
	dstName := concat(srcName, ".sz")
	setDstName(&dstName)

	print(concat(srcName, "  >  ", dstName))

	// Create the destination file.
	dst, err := create(dstName, srcInfo.Mode())
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Set up a *passthru writer in order to print progress.
	pt := &passthru{
		Writer:    dst,
		nExpected: uint64(srcInfo.Size()),
	}
	defer pt.Reset()

	// Wrap a *snappy.Writer around the *passthru method.
	sz := snappy.NewWriter(pt)
	defer sz.Reset(nil)

	// Write the source file's contents to the new snappy file.
	_, err = snapCopy(sz, src)
	print()
	if err != nil {
		return "", err
	}

	return dstName, nil
}

// SnappyMaxUncompressedChunkLen is a copy of snappy.maxUncompressedChunkLen
const SnappyMaxUncompressedChunkLen = 65536

// Read data from a source file,
//   compress the data,
//   and write it to a *snappy.Writer destination file.
// Serves as a makeshift snappy replacement for io.Copy
//   as long as the source Reader is an *os.File
//   and the destination Writer is a *snappy.Writer.
func snapCopy(sz *snappy.Writer, src *os.File) (int64, error) {

	buf := make([]byte, SnappyMaxUncompressedChunkLen)
	return io.CopyBuffer(sz, src, buf)

	// Slow and dangerous. Kept for testing purposes.
	// srcContents, err := ioutil.ReadAll(src)
	// if err != nil {
	//  return
	// }
	// totalWritten, err = sz.Write(srcContents)
	// return int64(totalWritten), err
}

// Decompress a snappy archive.
func unsnap(src *os.File) (dst *os.File, err error) {
	var srcInfo os.FileInfo
	srcInfo, err = src.Stat()
	if err != nil {
		return
	}
	srcName := srcInfo.Name()

	// Make sure existing files are not overwritten.
	dstName := strings.TrimSuffix(srcName, ".sz")
	setDstName(&dstName)

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
		Reader:    src,
		nExpected: uint64(srcInfo.Size()),
	}
	defer pt.Reset()

	szr := snappy.NewReader(pt)
	defer szr.Reset(nil)

	defer print()
	_, err = io.Copy(dst, szr)
	return
}
