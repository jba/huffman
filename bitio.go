// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import "io"

// Much of the code in this file is adapted from the standard library's compress/flate package.

// A bitWriter can write up to 32 bits at a time.
// Full bytes are flushed to its contained [io.Writer].
// Write errors are stored and reported by [bitWriter.Close]
// or [bitWriter.Err].
// If the bitWriter is flushed on a non-byte boundary, the last byte
// is zero-padded on the high side.
type bitWriter struct {
	err error
	w   io.Writer
	// bits is a buffer of unwritten bits.
	// Only the low-order 32 bits are valid between calls to writeBits,
	// and those bytes are stored in reverse order: byte 3 | byte 2 | byte 1 | byte 0.
	bits  uint64
	nbits int // number of bits in bits; always <= 32
}

func newBitWriter(w io.Writer) *bitWriter {
	return &bitWriter{w: w}
}

// writeBits writes the n
func (w *bitWriter) writeBits(b uint32, n int) {
	if w.err != nil {
		return
	}
	w.bits |= uint64(b) << w.nbits // w.bits = b concat w.bits
	w.nbits += n                   // there are n more bits in w.bits
	if w.nbits > 32 {              // if w.bits is too large
		var buf [4]byte // write out the low-order part
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

type bitReader struct {
	err       error
	r         io.Reader
	remaining int // number of bits left to read
	// bits is a buffer of unread bits.
	// Only the low-order 32 bits are valid between calls to readBits,
	// and those bytes are stored in reverse order: byte 3 | byte 2 | byte 1 | byte 0.
	bits  uint64
	nbits int // number of bits in bits; >= 8 except at end
}

func newBitReader(r io.Reader, n int) *bitReader {
	br := &bitReader{r: r, remaining: n}
	br.fill()
	return br
}

// precondition: the high 32 bits of r.bits are empty.
// postcondition: the low 32 bits of r.bits are populated, or fewer at EOF.
func (r *bitReader) fill() {
	var buf [4]byte
	n, err := r.r.Read(buf[:])
	if err != nil && err != io.EOF {
		r.err = err
		return
	}
	if n == 0 {
		if r.remaining == 0 {
			r.err = io.EOF
		} else {
			r.err = io.ErrUnexpectedEOF
		}
		return
	}
	// Put the n bytes of buf into a uint32, as byte[3] | byte[2] | byte[1] | byte[0].
	u := (uint32(buf[3]) << 24) | (uint32(buf[2]) << 16) | (uint32(buf[1]) << 8) | uint32(buf[0])
	// Put those bytes into r.bits just above its current contents.
	r.bits = uint64(u<<r.nbits) | uint64(lowOrderBits(r.bits, r.nbits))
	r.nbits = min(r.nbits+n*8, r.remaining)
}

// read n bits, up to 8
// Reading past the end is ErrUnexpectedEOF, not EOF.
func (r *bitReader) readBits(n int) (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if n <= 0 || n > 8 {
		panic("bad number of bits to read")
	}
	if n > r.remaining {
		return 0, io.ErrUnexpectedEOF
	}
	if r.nbits < n {
		r.fill()
		if r.err != nil {
			return 0, r.err
		}
	}
	if r.nbits < n {
		panic("r.nbits < n after fill: should not happen")
	}
	res := lowOrderBits(r.bits, n)
	r.nbits -= n
	r.bits >>= n
	return byte(res), nil
}

// Return the next byte, even if it has <8 bits.
func (r *bitReader) peek() (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.nbits == 0 {
		return 0, io.EOF
	}
	return byte(lowOrderBits(r.bits, 8)), nil
}

// lowOrderBits returns the n low-order bits of u.
func lowOrderBits[T uint8 | uint16 | uint32 | uint64](u T, n int) T {
	return u & ((T(1) << n) - 1)
}
