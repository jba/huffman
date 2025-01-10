// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import "io"

// Much of the code in this file is adapted from the standard library's compress/flate package.

// A bitWriter can write up to 32 bits at a time.
// Full bytes are flushed to its contained [io.Writer].
// When the bitWriter is flushed, the final byte is zero-padded.
// Write errors are stored and reported by [bitWriter.Close]
// or [bitWriter.Err].
// If the bitWriter is flushed on a non-byte boundary, the last byte
// is padded on the high side.
type bitWriter struct {
	err error
	w   io.Writer
	// bits is a buffer of unwritten bits.
	// The low-order part is stored in reverse order: ... | byte 1 | byte 0 |.
	bits  uint64
	nbits int // number of bits in bits; always <= 32
}

func newBitWriter(w io.Writer) *bitWriter {
	return &bitWriter{w: w}
}

// WriteBits assumes that b is an n-bit number; that is, that
func (w *bitWriter) WriteBits(b uint32, n int) {
	if w.err != nil {
		return
	}
	w.bits |= uint64(b) << w.nbits
	w.nbits += n
	if w.nbits > 32 {
		var buf [4]byte
		buf[0] = byte(w.bits)
		buf[1] = byte(w.bits >> 8)
		buf[2] = byte(w.bits >> 16)
		buf[3] = byte(w.bits >> 24)
		w.bits >>= 32
		w.nbits -= 32
		w.write(buf[:])
	}
}

func (w *bitWriter) Close() error {
	w.flush()
	return w.err
}

func (w *bitWriter) flush() {
	var buf [4]byte
	var i int
	for i = 0; i < 4 && w.nbits > 0; i++ {
		buf[i] = byte(w.bits)
		w.bits >>= 8
		if w.nbits > 8 {
			w.nbits -= 8
		} else {
			w.nbits = 0
		}
	}
	w.write(buf[:i])
}

func (w *bitWriter) write(buf []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.Write(buf)
}

func (w *bitWriter) Err() error {
	return w.err
}
