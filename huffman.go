// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package huffman

import (
	"errors"
	"fmt"
	"io"
	"slices"
)

// A Symbol is a symbol in an alphabet. It may represent a byte or Unicode code point,
// or it may be an index into a table of arbitrary runes or strings.
// For the most part, this package does not distinguish those cases. The exception
// is [Encoder.AddBytes], which expects the symbols to be bytes.
type Symbol = uint32

// A Code is a mapping from Symbols to bit sequences.
type Code struct {
	codes []bitcode
}

type bitcode struct {
	val, len uint32
}

const maxCodeLen = 20

// NewCode constructs a [Code] for symbols with the given frequencies.
// The values at frequencies[i] is the frequency for Symbol(i).
// If a frequency is 0, the corresponding symbol must not appear
// in the input given to an [Encoder].
func NewCode(frequencies []int) (*Code, error) {
	if len(frequencies) > 1<<maxCodeLen {
		return nil, fmt.Errorf("huffman.NewCode: too many frequencies (max 2^%d)", maxCodeLen)
	}
	for _, f := range frequencies {
		if f < 0 {
			return nil, errors.New("huffman.NewCode: negative frequency")
		}
	}
	enc := newHuffmanEncoder(len(frequencies))
	freqs := make([]int32, len(frequencies))
	for i, f := range frequencies {
		freqs[i] = int32(f)
	}
	enc.generate(freqs, 15)
	c := &Code{codes: make([]bitcode, len(enc.codes))}
	for i, hc := range enc.codes {
		c.codes[i] = bitcode{val: uint32(hc.code), len: uint32(hc.len)}
	}
	return c, nil
}

const marshalVersion = 0

// Marshal compactly represents the Code as a sequence of bytes.
func (c *Code) Marshal() []byte {
	// We may eventually use an algorithm like RFC 1951, but with a larger alphabet to handle larger code sizes.
	// For now we do something simpler:
	// The data contains the code sizes for each symbol in the alphabet.
	// First byte: version number, with the top two bits 1's as a tiny magic header.
	// Other bytes:
	// - bottom five bits are code size 0-20 (the rest are wasted)
	// - top three bits are 2^repetitions (0=1, 1=2, 2=4, ..., 7=128)

	buf := []byte{0b11<<6 | marshalVersion}
	i := 0
	for i < len(c.codes) {
		n := c.codes[i].len
		var j int
		for j = i + 1; j < len(c.codes) && c.codes[j].len == n; j++ {
		}
		rep := j - i
		for rep > 0 {
			r := min(rep, 128)
			rep -= r
			p := uint32(0)
			for r > 0 && p < 8 {
				if r&1 == 1 {
					buf = append(buf, byte(p<<5|n))
				}
				r >>= 1
				p++
			}
		}
		i = j
	}
	return buf
}

// UnmarshalCode reconstructs a [Code] from the data, which must have been created with [Code.Marshal].
func UnmarshalCode(data []byte) (*Code, error) {
	if len(data) == 0 {
		return nil, errors.New("huffman.UnmarshalCode: empty data")
	}
	if data[0] != byte(0b11<<6|marshalVersion) {
		return nil, errors.New("huffman.UnmarshalCode: bad magic/version")
	}
	var codes []bitcode
	for _, b := range data[1:] {
		len := b & 0b11111
		rep := b >> 5
		codes = slices.Grow(codes, int(rep))
		for range rep {
			codes = append(codes, bitcode{len: uint32(len)})
		}
	}
	return &Code{codes: codes}, nil
}

// TODO: is a code for (byte) faster?
// TODO: just panic if out of range?
func (c *Code) code(s Symbol) bitcode {
	if s >= uint32(len(c.codes)) {
		return bitcode{}
	}
	return c.codes[s]
}

// TODO: return (int, []Symbol) so SplitFunc doesn't have to consume all its input.
type SplitFunc func([]byte) []Symbol

type CodeBuilder struct {
	split SplitFunc
	freqs []int
}

func NewCodeBuilder(split SplitFunc) *CodeBuilder {
	return &CodeBuilder{split: split}
}

// Always returns a nil error.
func (cb *CodeBuilder) Write(data []byte) (int, error) {
	if cb.split != nil {
		syms := cb.split(data)
		for _, s := range syms {
			cb.growFreqs(s)
			cb.freqs[s]++
		}
	} else {
		for _, b := range data {
			cb.growFreqs(uint32(b))
			cb.freqs[b]++
		}
	}
	return len(data), nil
}

// growFreqs grows cb.freqs so that freqs[n] will not panic.
func (cb *CodeBuilder) growFreqs(n uint32) {
	ulen := uint32(len(cb.freqs))
	if ulen <= n {
		g := int(n-ulen) + 1
		cb.freqs = slices.Grow(cb.freqs, g)
		for range g {
			cb.freqs = append(cb.freqs, 0)
		}
	}
}

func (cb *CodeBuilder) Code() (*Code, error) {
	return NewCode(cb.freqs)
}

// An Encode encodes symbols with a [Code].
type Encoder struct {
	c     *Code
	bw    *bitWriter
	split SplitFunc
}

// If there is no SplitFunc, it is an error if the Encoder's [Code] contains more than 256 symbols
func (c *Code) NewEncoder(w io.Writer, split SplitFunc) *Encoder {
	if split == nil && len(c.codes) > 256 {
		panic("no split func but more than 256 codes")
	}
	return &Encoder{c: c, bw: newBitWriter(w), split: split}
}

// If there is no SplitFunc, it is an error if the Encoder's [Code] contains more than 256 symbols, or if any
// of the byte values exceed the largest symbol, or if any of the byte values had
// a zero frequency when the [Code] was constructed.
// Always returns len(data), nil. Errors reported by [Encoder.Close].
func (e *Encoder) Write(data []byte) (int, error) {
	if e.split != nil {
		e.WriteSymbols(e.split(data))
	} else {
		e.WriteBytes(data)
	}
	return len(data), nil
}

func (e *Encoder) WriteBytes(bs []byte) {
	for _, b := range bs {
		e.WriteSymbol(Symbol(b))
	}
}

func (e *Encoder) WriteSymbol(s Symbol) {
	// TODO: faster to have a specialized bits(byte)?
	b := e.c.code(s)
	if b.len == 0 {
		panic(fmt.Sprintf("no code for symbol %d", s))
	}
	// TODO: benchmark if WriteBits takes a uint8, or bits.len is an int.
	e.bw.WriteBits(b.val, int(b.len))
}

func (e *Encoder) WriteSymbols(syms []Symbol) {
	for _, s := range syms {
		e.WriteSymbol(s)
	}
}

// TODO: rewrite
// Bytes returns the encoded bytes constructed from the calls to the AddXXX methods,
// along with the first error encountered while adding.
func (e *Encoder) Close() error {
	return e.bw.Close()
}

// A Decoder decodes data encoded by an Encoder.
type Decoder struct{}

func (c *Code) NewDecoder(encoded []byte) *Decoder { return nil }

func (d *Decoder) DecodeBytes(buf []byte) (int, error) {
	return 0, nil

}

func (d *Decoder) DecodeSymbols(buf []Symbol) (int, error) { return 0, nil }

func (d *Decoder) SetEncoded(e []byte) {}
