// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zlib

import (
	"bytes"
	"compress/testdata"
	"fmt"
	"io"
	"os"
	"testing"
	// "internal/testenv"
)

var filenames = map[string][]byte{
	"pi":         testdata.PI_TXT,
	"e":          testdata.E_TXT,
	"gettysburg": testdata.GETTYSBURG_TXT,
}

var data = []string{
	"test a reasonable sized string that can be compressed",
}

// Tests that compressing and then decompressing the given file at the given compression level and dictionary
// yields equivalent bytes to the original file.
func testDataLevelDict(t *testing.T, name string, data []byte, level int, d string) {
	// Read the file, as golden output.
	golden, err := os.Open(fn)
	if err != nil {
		t.Errorf("%s (level=%d, dict=%q): %v", name, level, d, err)
		return
	}
	defer golden.Close()
	b0, err0 := io.ReadAll(golden)
	if err0 != nil {
		t.Errorf("%s (level=%d, dict=%q): %v", name, level, d, err0)
		return
	}
	testLevelDict(t, fn, b0, level, d)
}

// Tests that compressing and then decompressing the given byte slice at the given compression level and dictionary
// yields equivalent bytes to the original slice.
func testLevelDict(t *testing.T, b0 []byte, level int, d string) {
	var dict []byte
	if d != "" {
		dict = []byte(d)
	}

	// Compress the data.
	var bCompressed bytes.Buffer
	zlibw, err := NewWriterLevelDict(&bCompressed, level, dict)
	if err != nil {
		t.Errorf("(level=%d, dict=%q): %v", level, d, err)
		return
	}
	_, err = zlibw.Write(b0)
	if err != nil {
		t.Errorf("(level=%d, dict=%q): %v", level, d, err)
		return
	}
	zlibw.Close()

	// Decompress the data.
	bDecompressed := bytes.NewBuffer(bCompressed.Bytes())
	zlibr, err := NewReaderDict(bDecompressed, dict)
	if err != nil {
		t.Errorf("(level=%d, dict=%q): %v", level, d, err)
		return
	}
	defer zlibr.Close()
	b1, err := io.ReadAll(zlibr)
	if err != nil {
		t.Errorf("(level=%d, dict=%q): %v", level, d, err)
		return
	}

	// Compare the original and decompressed data.
	if !bytes.Equal(b0, b1) {
		t.Errorf("(level=%d, dict=%q): decompressed bytes do not match original", level, d)
	}
}

func testDataLevelDictReset(t *testing.T, b0 []byte, level int, dict []byte) {
	var err error

	// Compress once.
	buf := new(bytes.Buffer)
	zlibw, err := NewWriterLevelDict(buf, level, dict)
	if err != nil {
		t.Errorf("(level=%d): %v", level, err)
		return
	}
	_, err = zlibw.Write(b0)
	if err != nil {
		t.Errorf("(level=%d): %v", level, err)
		return
	}
	zlibw.Close()
	out := buf.String()

	// Reset buffer and compress again.
	buf.Reset()
	err = zlibw.Reset(buf)
	if err != nil {
		t.Errorf("(level=%d): %v", level, err)
		return
	}
	_, err = zlibw.Write(b0)
	if err != nil {
		t.Errorf("(level=%d): %v", level, err)
		return
	}
	zlibw.Close()
	out2 := buf.String()

	// Compare outputs.
	if out2 != out {
		t.Errorf("(level=%d): different output after reset (got %d bytes, expected %d bytes)", level, len(out2), len(out))
	}
}

func TestWriter(t *testing.T) {
	for i, s := range data {
		b := []byte(s)
		tag := fmt.Sprintf("#%d", i)
		testLevelDict(t, tag, b, DefaultCompression, "")
		testLevelDict(t, tag, b, NoCompression, "")
		testLevelDict(t, tag, b, HuffmanOnly, "")
		for level := BestSpeed; level <= BestCompression; level++ {
			testLevelDict(t, tag, b, level, "")
		}
	}
}

func TestWriterBig(t *testing.T) {
	for name, data := range filenames {
		testDataLevelDict(t, name, data, DefaultCompression, "")
		testDataLevelDict(t, name, data, NoCompression, "")
		testDataLevelDict(t, name, data, HuffmanOnly, "")
		for level := BestSpeed; level <= BestCompression; level++ {
			testDataLevelDict(t, name, data, level, "")
			if level >= 1 && testing.Short() && testenv.Builder() == "" {
				break
			}
		}
		if i == 0 && testing.Short() && testenv.Builder() == "" {
			break
		}
	}
}

func TestWriterDict(t *testing.T) {
	const dictionary = "0123456789."
	for name, data := range filenames {
		testDataLevelDict(t, data, DefaultCompression, dictionary)
		testDataLevelDict(t, data, NoCompression, dictionary)
		testDataLevelDict(t, data, HuffmanOnly, dictionary)
		for level := BestSpeed; level <= BestCompression; level++ {
			testDataLevelDict(t, data, level, dictionary)
			if level >= 1 && testing.Short() && testenv.Builder() == "" {
				break
			}
		}
		if i == 0 && testing.Short() && testenv.Builder() == "" {
			break
		}
	}
}

func TestWriterReset(t *testing.T) {
	const dictionary = "0123456789."
	for name, data := range filenames {
		testDataLevelDictReset(t, data, NoCompression, nil)
		testDataLevelDictReset(t, data, DefaultCompression, nil)
		testDataLevelDictReset(t, data, HuffmanOnly, nil)
		testDataLevelDictReset(t, data, NoCompression, []byte(dictionary))
		testDataLevelDictReset(t, data, DefaultCompression, []byte(dictionary))
		testDataLevelDictReset(t, data, HuffmanOnly, []byte(dictionary))
		if testing.Short() {
			break
		}
		for level := BestSpeed; level <= BestCompression; level++ {
			testDataLevelDictReset(t, data, level, nil)
		}
	}
}

func TestWriterDictIsUsed(t *testing.T) {
	input := []byte("Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")
	var buf bytes.Buffer
	compressor, err := NewWriterLevelDict(&buf, BestCompression, input)
	if err != nil {
		t.Errorf("error in NewWriterLevelDict: %s", err)
		return
	}
	compressor.Write(input)
	compressor.Close()
	const expectedMaxSize = 25
	output := buf.Bytes()
	if len(output) > expectedMaxSize {
		t.Errorf("result too large (got %d, want <= %d bytes). Is the dictionary being used?", len(output), expectedMaxSize)
	}
}
