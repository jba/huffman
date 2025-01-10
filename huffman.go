// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

// TODO: find an efficient way to decode that doesn't decode everything at once.
//      Some sort of chunking.

package huffman

import (
	"errors"
	"math"
	"slices"
)

// A Symbol is a symbol in an alphabet. It may represent a byte or Unicode code point,
// or it may be an index into a table of arbitrary runes or strings.
// For the most part, this package does not distinguish those cases. The exception
// is [Encoder.AddBytes], which expects the symbols to be bytes.
type Symbol = uint32

// A Code is a mapping from Symbols to bit sequences.
type Code struct {
	codes []bits
}

type bits struct {
	val uint32
	len uint8
}

// NewCode constructs a [Code] for symbols with the given frequencies.
// The values at frequencies[i] is the frequency for Symbol(i).
// If a frequency is 0, the corresponding symbol must not appear
// in the input given to an [Encoder].
func NewCode(frequencies []int) (*Code, error) {
	if len(frequencies) > math.MaxInt32 {
		return nil, errors.New("huffman.NewCode: too many frequencies (max 2^32)")
	}
	for _, f := range frequencies {
		if f < 0 {
			return nil, errors.New("huffman.NewCode: negative frequency")
		}
	}
	// TODO
	return nil, nil
}

// Marshal compactly represents the Code as a sequence of bytes.
func (c *Code) Marshal() []byte {
	// Use algorithm like RFC 1951, but with a larger alphabet to handle larger code sizes.
	return nil
}

// UnmarshalCode reconstructs a [Code] from the data, which must have been created with [Code.Marshal].
func UnmarshalCode(data []byte) (*Code, error) {
	return nil, nil
}

// TODO: is a bits for (byte) faster?
// TODO: just panic if out of range?
func (c *Code) bits(s Symbol) bits {
	if s >= uint32(len(c.codes)) {
		return bits{}
	}
	return c.codes[s]
}

type SplitFunc func([]byte) []Symbol

type CodeBuilder struct {
	split SplitFunc
	freqs []int
}

func NewCodeBuilder(split SplitFunc) *CodeBuilder {
	return &CodeBuilder{split: split}
}

func (cb *CodeBuilder) Write(data []byte) (int, error) {
	syms := cb.split(data)
	for _, s := range syms {
		ulen := uint32(len(cb.freqs))
		if ulen < s {
			n := int(s-ulen) + 1
			cb.freqs = slices.Grow(cb.freqs, n)
			for range n {
				cb.freqs = append(cb.freqs, 0)
			}
		}
		cb.freqs[s]++
	}
	return len(data), nil
}

func (cb *CodeBuilder) Code() (*Code, error) {
	return NewCode(cb.freqs)
}

// An Encode encodes symbols with a [Code].
type Encoder struct {
	c   *Code
	err error
}

func (c *Code) NewEncoder() *Encoder {
	return &Encoder{c: c}
}

// It is an error if the Encoder's [Code] contains more than 256 symbols, or if any
// of the byte values exceed the largest symbol, or if any of the byte values had
// a zero frequency when the [Code] was constructed.
func (e *Encoder) AddBytes(data []byte) {
	for _, b := range data {
		_ = b
	}
}

func (e *Encoder) AddSymbol(s Symbol) {
	if e.err != nil {
		return
	}
	// TODO
}

func (e *Encoder) AddSymbols(syms []Symbol) {
	for _, s := range syms {
		if e.err != nil {
			return
		}
		e.AddSymbol(s)
	}
}

// Bytes returns the encoded bytes constructed from the calls to the AddXXX methods,
// along with the first error encountered while adding.
func (e *Encoder) Bytes() ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	return nil, nil
}

// Err returns the first error encountered from adding data, if any.
func (e *Encoder) Err() error { return e.err }

// Reset restores the Encoder to its initial state.
func (e *Encoder) Reset() {}

// A Decoder decodes data encoded by an Encoder.
type Decoder struct{}

func (c *Code) NewDecoder(encoded []byte) *Decoder { return nil }

func (d *Decoder) DecodeBytes(buf []byte) (int, error) {
	return 0, nil

}

func (d *Decoder) DecodeSymbols(buf []Symbol) (int, error) { return 0, nil }

func (d *Decoder) SetEncoded(e []byte) {}
