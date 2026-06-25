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

func TestRoundTrip(t *testing.T) {
	t.Run("short_string", func(t *testing.T) {
		input := "a man a plan a canal panama"
		testRoundTrip(t, []byte(input), nil)
	})

	t.Run("all_bytes", func(t *testing.T) {
		// Every byte value appears at least once.
		var input []byte
		for i := range 256 {
			for range i + 1 { // varying frequencies
				input = append(input, byte(i))
			}
		}
		testRoundTrip(t, input, nil)
	})

	t.Run("single_char", func(t *testing.T) {
		input := bytes.Repeat([]byte("x"), 100)
		testRoundTrip(t, input, nil)
	})

	t.Run("two_chars", func(t *testing.T) {
		testRoundTrip(t, []byte("aaabbb"), nil)
	})

	t.Run("pride_and_prejudice", func(t *testing.T) {
		input, err := os.ReadFile(filepath.Join("testdata", "pride-and-prejudice.txt"))
		if err != nil {
			t.Fatal(err)
		}
		testRoundTrip(t, input, nil)
	})

	t.Run("explicit_frequencies", func(t *testing.T) {
		// Build code from explicit frequencies, encode symbols, decode, compare.
		freqs := []int{5, 9, 12, 13, 16, 45}
		code, err := NewCode(freqs)
		if err != nil {
			t.Fatal(err)
		}

		symbols := []Symbol{0, 1, 2, 3, 4, 5, 5, 5, 4, 3, 2, 1, 0}

		// Compute total encoded bits.
		totalBits := 0
		for _, s := range symbols {
			totalBits += int(code.codes[s].len)
		}

		var buf bytes.Buffer
		enc := code.NewEncoder(&buf, nil)
		enc.WriteSymbols(symbols)
		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		dec := code.NewDecoder()
		got, err := dec.Decode(buf.Bytes(), totalBits)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, symbols) {
			t.Errorf("round trip failed:\n  got  %v\n  want %v", got, symbols)
		}
	})
}

func testRoundTrip(t *testing.T, input []byte, split SplitFunc) {
	t.Helper()

	cb := NewCodeBuilder(split)
	cb.Write(input)
	code, err := cb.Code()
	if err != nil {
		t.Fatal(err)
	}

	// Encode.
	var buf bytes.Buffer
	enc := code.NewEncoder(&buf, split)
	enc.Write(input)
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	// Compute total encoded bits.
	totalBits := 0
	for _, b := range input {
		totalBits += int(code.codes[Symbol(b)].len)
	}

	// Decode.
	dec := code.NewDecoder()
	symbols, err := dec.Decode(buf.Bytes(), totalBits)
	if err != nil {
		t.Fatal(err)
	}

	// Convert symbols back to bytes and compare.
	gotBytes := make([]byte, len(symbols))
	for i, s := range symbols {
		gotBytes[i] = byte(s)
	}
	if !bytes.Equal(gotBytes, input) {
		max := min(len(input), 100)
		t.Errorf("round trip failed: got %d bytes, want %d bytes\n  got[:100]  %q\n  want[:100] %q",
			len(gotBytes), len(input), gotBytes[:max], input[:max])
	}
}

func TestRoundTripLargeAlphabet(t *testing.T) {
	const numSymbols = 2000

	// Build frequencies: symbol i has frequency i+1, giving a skewed distribution.
	freqs := make([]int, numSymbols)
	for i := range freqs {
		freqs[i] = i + 1
	}
	code, err := NewCode(freqs)
	if err != nil {
		t.Fatal(err)
	}

	// Build a test sequence using all 2000 symbols, repeated with varying patterns.
	var symbols []Symbol
	for i := range numSymbols {
		symbols = append(symbols, Symbol(i))
	}
	// Append a longer run weighted toward high-frequency symbols.
	for i := range 5000 {
		symbols = append(symbols, Symbol(i%numSymbols))
	}

	// Compute total encoded bits.
	totalBits := 0
	for _, s := range symbols {
		totalBits += int(code.codes[s].len)
	}

	// Encode using WriteSymbols (required for >256-symbol alphabets without a SplitFunc).
	var buf bytes.Buffer
	enc := code.NewEncoder(&buf, func(b []byte) []Symbol { return nil }) // dummy split
	enc.WriteSymbols(symbols)
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	t.Logf("%d symbols, %d distinct, %d encoded bits (%d bytes)",
		len(symbols), numSymbols, totalBits, buf.Len())

	// Decode.
	dec := code.NewDecoder()
	got, err := dec.Decode(buf.Bytes(), totalBits)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if !slices.Equal(got, symbols) {
		mismatch := -1
		for i := range min(len(got), len(symbols)) {
			if got[i] != symbols[i] {
				mismatch = i
				break
			}
		}
		t.Fatalf("round trip failed: got %d symbols, want %d; first mismatch at index %d",
			len(got), len(symbols), mismatch)
	}
}

func FuzzRoundTrip(f *testing.F) {
	// Seed corpus.
	f.Add([]byte("a man a plan a canal panama"))
	f.Add([]byte("aaabbb"))
	f.Add(bytes.Repeat([]byte("x"), 100))
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{0xff, 0x00, 0xff, 0x00})

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) == 0 {
			return
		}

		// Build code from the input itself.
		cb := NewCodeBuilder(nil)
		cb.Write(input)
		code, err := cb.Code()
		if err != nil {
			t.Fatal(err)
		}

		// Encode.
		var buf bytes.Buffer
		enc := code.NewEncoder(&buf, nil)
		enc.Write(input)
		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		// Compute total encoded bits.
		totalBits := 0
		for _, b := range input {
			totalBits += int(code.codes[Symbol(b)].len)
		}

		// Decode.
		dec := code.NewDecoder()
		symbols, err := dec.Decode(buf.Bytes(), totalBits)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		// Compare.
		if len(symbols) != len(input) {
			t.Fatalf("length mismatch: decoded %d symbols, want %d", len(symbols), len(input))
		}
		for i, s := range symbols {
			if byte(s) != input[i] {
				t.Fatalf("mismatch at index %d: got %d, want %d", i, s, input[i])
			}
		}
	})
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
