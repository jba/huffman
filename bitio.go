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
	err     error
	w       io.Writer
	written bool // true if any bits have been written
	// bits is a buffer of unwritten bits.
	// Only the low-order 32 bits are valid between calls to writeBits,
	// and those bytes are stored in reverse order: byte 3 | byte 2 | byte 1 | byte 0.
	bits  uint64
	nbits int // number of bits in bits; always <= 32
}

func newBitWriter(w io.Writer) *bitWriter {
	return &bitWriter{w: w}
}

// writeBits writes the n low-order bits of b.
func (w *bitWriter) writeBits(b uint32, n int) {
	if w.err != nil {
		return
	}
	w.written = true
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
	// Flush remaining bits, then write a trailer byte indicating
	// how many bits in the last data byte are valid (1-8), or 0
	// if no data was written.
	validBits := byte(0)
	if w.written || w.nbits > 0 {
		if w.nbits == 0 {
			validBits = 8
		} else {
			validBits = byte((w.nbits-1)%8 + 1)
		}
	}
	w.flush()
	w.write([]byte{validBits})
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

// A bitReader reads bits from an io.Reader whose last byte is a trailer
// written by [bitWriter.Close]. The trailer indicates how many bits in
// the preceding byte are valid (1-8), or 0 if no data was written.
//
// The reader stays one byte ahead: the most recently read byte might be
// the trailer. When the underlying reader returns EOF, the lookahead
// byte is the trailer, and remaining valid bits are computed.
type bitReader struct {
	err error
	r   io.Reader
	// bits is a buffer of unread bits.
	// Only the low-order 32 bits are valid between calls to readBits,
	// and those bytes are stored in reverse order: byte 3 | byte 2 | byte 1 | byte 0.
	bits  uint64
	nbits int // number of valid bits in bits

	ahead    byte // one byte of lookahead; might be the trailer
	hasAhead bool
	atEOF    bool
	remaining int // valid bits left to read; -1 until trailer is seen
}

func newBitReader(r io.Reader) *bitReader {
	br := &bitReader{r: r, remaining: -1}
	// Prime the lookahead with the first byte.
	var buf [1]byte
	n, err := r.Read(buf[:])
	if n == 1 {
		br.ahead = buf[0]
		br.hasAhead = true
	}
	if err == io.EOF {
		br.atEOF = true
		br.remaining = 0
		br.hasAhead = false // if set, the only byte was the trailer
	} else if err != nil {
		br.err = err
		return br
	}
	if !br.atEOF {
		br.fill()
	}
	return br
}

// fill reads more bytes from the underlying reader and packs confirmed
// data bytes into the bit buffer. It maintains one byte of lookahead
// so it can identify the trailer when EOF arrives.
//
// precondition: the high 32 bits of r.bits are empty.
func (r *bitReader) fill() {
	if r.atEOF || r.err != nil {
		return
	}

	// Read up to 4 new bytes.
	var buf [4]byte
	n, err := r.r.Read(buf[:])
	isEOF := err == io.EOF
	if err != nil && !isEOF {
		r.err = err
		return
	}

	// Assemble all bytes in hand: old lookahead + new bytes.
	var all [5]byte
	na := 0
	if r.hasAhead {
		all[0] = r.ahead
		na = 1
		r.hasAhead = false
	}
	copy(all[na:], buf[:n])
	na += n

	if na == 0 {
		if isEOF {
			r.atEOF = true
			r.remaining = r.nbits
		}
		return
	}

	if isEOF {
		// Last byte is the trailer.
		r.atEOF = true
		trailer := int(all[na-1])
		na-- // remove trailer

		r.packBytes(all[:na])

		// Compute remaining valid bits. We may have packed the partial
		// last data byte as a full 8 bits; subtract the padding.
		if trailer == 0 {
			r.remaining = 0
		} else {
			r.remaining = r.nbits - (8 - trailer)
		}
	} else {
		// Hold back the last byte as new lookahead.
		r.ahead = all[na-1]
		r.hasAhead = true
		na--

		r.packBytes(all[:na])
	}
}

// packBytes packs up to 4 data bytes into r.bits.
func (r *bitReader) packBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	u := uint32(0)
	for i, b := range data {
		u |= uint32(b) << (i * 8)
	}
	r.bits = (uint64(u) << r.nbits) | uint64(lowOrderBits(r.bits, r.nbits))
	r.nbits += len(data) * 8
}

// readBits reads n bits (1-8) and returns them in the low-order bits.
func (r *bitReader) readBits(n int) (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if n <= 0 || n > 8 {
		panic("bad number of bits to read")
	}
	if r.remaining >= 0 && n > r.remaining {
		return 0, io.ErrUnexpectedEOF
	}
	if r.nbits < n {
		r.fill()
		if r.err != nil {
			return 0, r.err
		}
	}
	// Re-check remaining after fill may have discovered the trailer.
	if r.remaining >= 0 && n > r.remaining {
		return 0, io.ErrUnexpectedEOF
	}
	if r.nbits < n {
		panic("r.nbits < n after fill: should not happen")
	}
	res := lowOrderBits(r.bits, n)
	r.nbits -= n
	r.bits >>= n
	if r.remaining >= 0 {
		r.remaining -= n
	}
	return byte(res), nil
}

// peek returns the next 8 bits (or fewer at the end) without consuming them.
func (r *bitReader) peek() (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if r.nbits < 8 {
		r.fill()
		if r.err != nil {
			if r.nbits == 0 || r.remaining == 0 {
				return 0, r.err
			}
			r.err = nil
		}
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	return byte(lowOrderBits(r.bits, 8)), nil
}

// lowOrderBits returns the n low-order bits of u.
func lowOrderBits[T uint8 | uint16 | uint32 | uint64](u T, n int) T {
	return u & ((T(1) << n) - 1)
}
