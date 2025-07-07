// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestEncoder(t *testing.T) {
	// This is a simple test of an encoder. The Code is not a Huffman code.
	// Every code is 8 bits to simplify comparisons.
	c := &Code{
		codes: []bitcode{
			{0, 8},
			{1, 8},
			{2, 8},
			{3, 8},
		},
	}

	var buf bytes.Buffer
	enc := c.NewEncoder(&buf, nil)
	data := []byte{1, 3, 2, 0}
	enc.Write(data[:2])
	enc.Write(data[2:])
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if !bytes.Equal(got, data) {
		t.Errorf("got %v, want %v", got, data)
	}

	// TODO: test Encoder with splitfunc.
}

func TestEncodeDecode(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		input := "a man a plan a canal panama"
		cb := NewCodeBuilder(nil)
		cb.Write([]byte(input))
		code, err := cb.Code()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		enc := code.NewEncoder(&buf, nil)
		enc.Write([]byte(input))
		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}
		// The code is canonical, so we can compare between runs.
		want := "721af9d5c8a8cdab03"
		gotBytes := buf.Bytes()
		got := hex.EncodeToString(gotBytes)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}

		// dec := code.NewDecoder()

		// TODO: decode
	})
	t.Run("pride bytes", func(t *testing.T) {
		input, err := os.ReadFile(filepath.Join("testdata", "pride-and-prejudice.txt"))
		if err != nil {
			t.Fatal(err)
		}
		cb := NewCodeBuilder(nil)
		cb.Write([]byte(input))
		code, err := cb.Code()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		enc := code.NewEncoder(&buf, nil)
		enc.Write([]byte(input))
		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCodeMarshal(t *testing.T) {
	for tci, tc := range []struct {
		lens []int
		want []byte
	}{
		// The zero form: RRRRRRR0
		{[]int{0}, []byte{0b00}},
		{[]int{0, 0, 0}, []byte{0b100}},
		{slices.Repeat([]int{0}, 130), []byte{127 << 1, 1 << 1}},
		// The 1-16 form: RRCCCC01
		{[]int{1, 2, 2, 5, 5, 5}, []byte{0b00_0000_01, 0b01_0001_01, 0b10_0100_01}},
		{append(slices.Repeat([]int{3}, 10), 11), []byte{0b11_0010_01, 0b11_0010_01, 0b01_0010_01, 0b00_1010_01}},
		// The 17-20 form: RRRRCC11
		{[]int{20, 20, 17}, []byte{0b0001_11_11, 0b0000_00_11}},
	} {
		var c Code
		for _, l := range tc.lens {
			c.codes = append(c.codes, bitcode{len: uint32(l)})
		}
		marsh := c.Marshal()
		if marsh[0] != 0b11000000 {
			t.Fatal("bad first byte")
		}
		got := marsh[1:]
		if !bytes.Equal(got, tc.want) {
			t.Errorf("%v:\ngot  %b\nwant %b", tc.lens, got, tc.want)
		}

		dec, err := UnmarshalCode(marsh)
		if err != nil {
			t.Fatal(err)
		}
		if g, w := len(dec.codes), len(c.codes); g != w {
			t.Fatalf("#%d: decoded %d codes, wanted %d", tci, g, w)
		}
		for i := range len(c.codes) {
			if g, w := c.codes[i].len, dec.codes[i].len; g != w {
				t.Errorf("#%d: %3d: %d != %d", tci, i, g, w)
			}
		}
	}
}

func TestAssignValues(t *testing.T) {
	// Example from RFC 1951, section 3.2.2.
	codes := []bitcode{{0, 2}, {0, 1}, {0, 3}, {0, 3}}
	want := []bitcode{{2, 2}, {0, 1}, {6, 3}, {7, 3}}
	assignValues(codes)
	if !slices.Equal(codes, want) {
		t.Errorf("got %v, want %v", codes, want)
	}
}
