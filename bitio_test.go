// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import (
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestWriteBits(t *testing.T) {
	const N = 64
	for range 100 {
		// Create two sets of inputs of different bit sizes.
		var bs [N]uint32
		var ns [N]int
		for i := range 32 {
			ns[i] = i + 1
			bs[i] = lowOrderBits(rand.Uint32(), ns[i])
		}
		for i := range 32 {
			ns[i+32] = i + 1
			bs[i+32] = lowOrderBits(rand.Uint32(), ns[i+32])
		}
		rand.Shuffle(N, func(i, j int) {
			bs[i], bs[j] = bs[j], bs[i]
			ns[i], ns[j] = ns[j], ns[i]
		})

		testBitWriter(t, bs[:], ns[:])
	}

	testBitWriter(t, []uint32{17, 1232323, 1 << 31}, []int{32, 32, 32})
	testBitWriter(t, []uint32{0, 1, 1, 1, 0, 1}, []int{1, 1, 1, 1, 1, 1})
}

func testBitWriter(t *testing.T, bs []uint32, ns []int) {
	t.Helper()
	var buf bytes.Buffer
	bw := newBitWriter(&buf)
	for i := range len(bs) {
		if bs[i]&(^((1 << ns[i]) - 1)) != 0 {
			t.Fatalf("bad value: %d does not fit int %d bits", bs[i], ns[i])
		}
		bw.writeBits(bs[i], ns[i])
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
	gotb := buf.Bytes()
	var sb strings.Builder
	for i, b := range gotb {
		if i > 0 {
			sb.WriteByte(':')
		}
		sb.WriteString(fmt.Sprintf("%08b", b))
	}
	got := sb.String()
	want := bitstring(bs[:], ns[:])
	if got != want {
		t.Errorf("\ngot  %s\nwant %s", got, want)
	}
}

func bitstring(bs []uint32, ns []int) string {
	var ss []string
	for i, b := range bs {
		s := fmt.Sprintf("%032b", b)
		// Take rightmost ns[i] bits.
		s = s[len(s)-ns[i]:]
		ss = append(ss, s)
	}
	slices.Reverse(ss)
	s := strings.Join(ss, "")
	var bytes []string
	for len(s) >= 8 {
		by := s[len(s)-8:]
		bytes = append(bytes, by)
		s = s[:len(s)-8]
	}
	if len(s) > 0 {
		for len(s) < 8 {
			s = "0" + s
		}
		bytes = append(bytes, s)
	}
	return strings.Join(bytes, ":")
}

func TestBitRead(t *testing.T) {
	r := bytes.NewReader([]byte{1, 2, 3, 4, 5, 6})
	br := newBitReader(r, 48)

	checkBits := func(wantNbits int, wantBits uint64) {
		t.Helper()
		if br.err != nil {
			t.Fatal(br.err)
		}
		if g := br.nbits; g != wantNbits {
			t.Fatalf("got %d, want %d", g, wantNbits)
		}
		if g := br.bits; g != wantBits {
			t.Errorf("got %x, want %x", g, wantBits)
		}
	}

	checkRead := func(n int, want byte) {
		t.Helper()
		got, err := br.readBits(n)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	}

	checkBits(32, 0x04030201)
	g, err := br.peek()
	if err != nil {
		t.Fatal(err)
	}
	if g != 1 {
		t.Errorf("got %d, want 1", g)
	}

	for i := range 6 {
		checkRead(8, byte(i+1))
	}
	if _, err := br.readBits(1); err != io.ErrUnexpectedEOF {
		t.Fatalf("got %v, want unexpected EOF", err)
	}

	br = newBitReader(bytes.NewReader([]byte{1, 2, 3, 4, 5, 6}), 48)

	for i := range 6 {
		checkRead(4, byte(i+1))
		checkRead(4, 0)
	}
	if _, err := br.readBits(1); err != io.ErrUnexpectedEOF {
		t.Fatalf("got %v, want unexpected EOF", err)
	}

	br = newBitReader(bytes.NewReader([]byte{1, 2, 3, 4, 5, 6}), 48)
	checkRead(3, 1)
	checkRead(2, 0)
	checkRead(3, 0)
	checkRead(2, 2)
	checkRead(5, 0)
	checkRead(3, 3)
}
