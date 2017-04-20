package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var (
	// nameDir string = "tmp"
	// nameDirTar string = "tmp.tar"
	// nameDirSnapped string = "tmp.tar.sz"
	// nameDirUnsnapped string = "tmp(1)"
	// nameReg string = "tmp/tmp/big-file"
	// nameRegSnapped string = "tmp/tmp/big-file.sz"

	nameDir                 = "tmp"
	nameDirTar              = nameDir + ".tar"
	nameDirSnapped          = nameDirTar + ".sz"
	nameDirUnsnapped        = nameDir + "(1)"
	nameReg                 = filepath.Join(nameDir, "tmp", "big-file")
	nameRegSnapped          = nameReg + ".sz"
	nameRegUnsnapped        = filepath.Join(nameDirUnsnapped, "tmp", "big-file")
	nameRegSnappedUnsnapped = nameRegUnsnapped + ".sz"

	sumReg        = "ba9670d3cf03043c7cb72acfdc2991f63fb1d2eb13e32306ef61c219d4e81ef0"
	sumRegSnapped = "ec94e9ebb6771856e0a574ecf735c6c93bd686f11e9d821809246cec45378800"
)

// TestSnap tests snappy compression of a file.
func TestSnap(t *testing.T) {

	srcName := nameReg
	dstNameExpected := nameRegSnapped
	sumName := nameRegSnapped
	sumExpected := sumRegSnapped

	dstName, err := compressOrDecompress(srcName)
	if err != nil {
		t.Error(err)
		return
	}
	if dstName != dstNameExpected {
		t.Errorf("Expected `dstName` to be %v but got %v.\n", dstNameExpected, dstName)
		return
	}

	sum, err := sha256sum(sumName)
	if err != nil {
		t.Error(err)
		return
	}
	if sum != sumExpected {
		t.Errorf("Expected `sum` to be %v but got %v.\n", sumExpected, sum)
		return
	}
}

// TestTarAndSnap tests tar archiving and snappy compression of file.
func TestTarAndSnap(t *testing.T) {

	srcName := nameDir
	tmpName := nameDirTar
	dstNameExpected := nameDirSnapped

	dstName, err := compressOrDecompress(srcName)
	if err != nil {
		t.Error(err)
		return
	}
	if dstName != dstNameExpected {
		t.Errorf("Expected `dstName` to be %v but got %v.\n", dstNameExpected, dstName)
		return
	}

	// snappy compressed files do not have a consistent checksum

	if exists(tmpName) {
		t.Errorf("Temporary archive %v should have been removed.\n", tmpName)
		return
	}
}

// TestUnsnapAndUntar tests tar extraction and snappy decompression of file.
func TestUnsnapAndUntar(t *testing.T) {

	srcName := nameDirSnapped
	tmpName := nameDirTar
	dstNameExpected := nameDirUnsnapped
	sumName := nameRegUnsnapped
	sumExpected := sumReg
	sumName2 := nameRegSnappedUnsnapped
	sumExpected2 := sumRegSnapped

	dstName, err := compressOrDecompress(srcName)
	if err != nil {
		t.Error(err)
		return
	}
	if dstName != dstNameExpected {
		t.Errorf("Expected `dstName` to be %v but got %v.\n", dstNameExpected, dstName)
		return
	}

	sum, err := sha256sum(sumName)
	if err != nil {
		t.Error(err)
		return
	}
	if sum != sumExpected {
		t.Errorf("Expected `sum` to be %v but got %v.\n", sumExpected, sum)
		return
	}

	sum2, err := sha256sum(sumName2)
	if err != nil {
		t.Error(err)
		return
	}
	if sum2 != sumExpected2 {
		t.Errorf("Expected `sum2` to be %v but got %v.\n", sumExpected2, sum2)
		return
	}

	if exists(tmpName) {
		t.Errorf("Temporary archive %v should have been removed.\n", tmpName)
		return
	}
}

// TestCleanup removes files created by this test.
func TestCleanup(t *testing.T) {

	paths := []string{
		nameRegSnapped,
		nameDirSnapped,
		nameDirUnsnapped,
	}

	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	}
}

func sha256sum(path string) (string, error) {

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer file.Close()

	hash := sha256.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	// If running low on RAM, use this instead.
	// hash := sha256.New()
	// blockSize := sha256.BlockSize
	// buf := make([]byte, blockSize)
	//
	// _, err = io.CopyBuffer(hash, file, buf)
	// if err != nil {
	// 	return "", err
	// }

	sum := fmt.Sprintf("%x", hash.Sum(nil))

	return sum, nil
}
