// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import (
	"bytes"
	"encoding/hex"
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
