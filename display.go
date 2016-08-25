package main

import (
	"fmt"
	"io"
	"math"
	"strings"
)

var (
	print = fmt.Println
)

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

// Empty print func for when 'DoQuiet' is set.
func printNoop(x ...interface{}) (int, error) {
	return 0, nil
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

	nTransferred uint64 // Total # of bytes transferred
	nExpected    uint64 // Expected length

	percentTransferred float64

	outputLength int
}

// Write 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
// NOTE: Print a new line after any commands which use this io.Reader.
func (pt *passthru) Read(b []byte) (int, error) {

	nRead, err := pt.Reader.Read(b)
	if eof := (nRead <= 0); eof || DoQuiet {
		return nRead, err
	}

	percent, shouldPrint := pt.updatePercentTransferred(nRead)
	if !shouldPrint {
		return nRead, err
	}

	labelSoFar := sizeLabel(pt.nTransferred)
	labelTotal := sizeLabel(pt.nExpected)

	output := fmt.Sprintf(
		"  %v%%   %v / %v",
		percent, labelSoFar, labelTotal,
	)

	pt.print(output)

	return nRead, err
}

func (pt *passthru) updatePercentTransferred(nTransferred int) (percentRounded int, shouldPrint bool) {

	pt.nTransferred += uint64(nTransferred)

	percentTransferred := float64(pt.nTransferred) / float64(pt.nExpected) * float64(100)
	percentRounded = int(percentTransferred)

	percentSinceLastPrint := percentTransferred - pt.percentTransferred
	tooSoonToPrint := percentSinceLastPrint < 2
	shouldPrint = (!tooSoonToPrint || percentRounded > 99)

	if !shouldPrint {
		return
	}

	pt.percentTransferred = percentTransferred

	return
}

func (pt *passthru) print(output string) {

	pt.updateOutputLength(output)
	pt.clearPreviousOutput()

	fmt.Printf("\r%v", output)
}

func (pt *passthru) updateOutputLength(nextOutput string) {

	nextOutputLength := len(nextOutput)
	prevOutputLength := pt.outputLength

	pt.outputLength = greaterOf(nextOutputLength, prevOutputLength)
}

func (pt *passthru) clearPreviousOutput() {
	emptySpace := strings.Repeat(" ", pt.outputLength)
	fmt.Printf("\r%v", emptySpace)
}

func greaterOf(x int, y int) int {
	greater := math.Max(float64(x), float64(y))
	return int(greater)
}

// Write 'overrides' the underlying io.Writer's Write method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
// NOTE: Print a new line after any commands which use this io.Writer.
func (pt *passthru) Write(b []byte) (int, error) {

	nWritten, err := pt.Writer.Write(b)
	if eof := (nWritten <= 0); eof || DoQuiet {
		return nWritten, err
	}

	percent, shouldPrint := pt.updatePercentTransferred(nWritten)
	if !shouldPrint {
		return nWritten, err
	}

	labelSoFar := sizeLabel(pt.nTransferred)
	labelTotal := sizeLabel(pt.nExpected)
	ratio := fmt.Sprintf("%.3f", float64(pt.nTransferred)/float64(pt.nExpected))

	output := fmt.Sprintf(
		"  %v%%   %v / %v = %v",
		percent, labelSoFar, labelTotal, ratio,
	)

	pt.print(output)

	return nWritten, err
}

func (pt *passthru) Reset() {
	go func() { pt.Reader = nil }()
	go func() { pt.Writer = nil }()
}

func sizeLabel(byteSize uint64) string {

	value, unit := bytesToSymbol(float64(byteSize))
	if tooSmall := (unit == ""); tooSmall {
		return "0"
	}

	stringValue := fmt.Sprintf("%.1f", value)
	return concat(stringValue, " ", unit)
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

func bytesToSymbol(b float64) (float64, string) {
	switch {
	case b >= TEBIBYTE:
		return (b / TEBIBYTE), "TiB"
	case b >= GIBIBYTE:
		return (b / GIBIBYTE), "GiB"
	case b >= MEBIBYTE:
		return (b / MEBIBYTE), "MiB"
	case b >= KIBIBYTE:
		return (b / KIBIBYTE), "KiB"
	case b >= BYTE:
		return b, "Bytes"
	}
	return b, ""
}
