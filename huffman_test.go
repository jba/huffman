// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import (
	"bytes"
	"encoding/hex"
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
}

func TestCodeMarshal(t *testing.T) {
	for _, tc := range []struct {
		lens []int
		want []byte
	}{
		{[]int{1}, []byte{0<<5 | 1}},
		{[]int{1, 1, 1}, []byte{0<<5 | 1, 1<<5 | 1}},
		{[]int{1, 1, 1, 2, 3, 3, 3, 3}, []byte{0<<5 | 1, 1<<5 | 1, 0<<5 | 2, 2<<5 | 3}},
		{slices.Repeat([]int{12}, 128), []byte{7<<5 | 12}},
		{slices.Repeat([]int{12}, 130), []byte{7<<5 | 12, 1<<5 | 12}},
		{slices.Repeat([]int{12}, 256), []byte{7<<5 | 12, 7<<5 | 12}},
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
			t.Errorf("%v:\ngot  %v\nwant %v", tc.lens, got, tc.want)
		}

		dec := UnmarshalCode(marsh)
		for i := range min(len(c.codes, dec.codes)) {
			if g, w := c.codes[i], dec.codes[i]; g != w {
				t.Errorf("%3d: %d != %d", i, g, w)
			}

		}

	}

}
