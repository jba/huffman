// Copyright 2025 Jonathan Amsterdam. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

// TODO:
// Large zip (68M): github.com/vkcom/statshouse	v1.0.0-beta1.0.20250117104732-ffd3f62aa4c1
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
// The value at frequencies[i] is the frequency for Symbol(i).
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
	// Encode the lengths of the bitcodes, in order.
	// We may eventually use an algorithm like RFC 1951, but with a larger alphabet to handle larger code sizes.
	// For now we do something simpler, and byte-oriented.
	// First byte: version number, with the top two bits 1's as a tiny magic header.
	// Other bytes:
	// There are three formats:
	//   RRRRRRR0:  length 0, with 7 bits of repeat (1-128)
	//   RRLLLL01:  lengths 1-16, with 2 bits of repeat (1-4)
	//   RRRRLL11:  lengths 17-20, with 4 bits of repeat (1-16)

	buf := []byte{0b11<<6 | marshalVersion}

	rep := func(R, len int, bottom byte) {
		shift := 8 - len
		max := 1 << len
		for R >= max {
			buf = append(buf, byte((max-1)<<shift)|bottom)
			R -= max
		}
		if R > 0 {
			buf = append(buf, byte((R-1)<<shift)|bottom)
		}
	}

	i := 0
	for i < len(c.codes) {
		L := c.codes[i].len
		var j int
		for j = i + 1; j < len(c.codes) && c.codes[j].len == L; j++ {
		}
		R := j - i
		// Code C appears R times consecutively.
		switch {
		case L == 0:
			rep(R, 7, 0)

		case L >= 1 && L <= 16:
			rep(R, 2, byte((L-1)<<2|1))

		case L >= 17 && L <= 20:
			rep(R, 4, byte((L-17)<<2|0b11))

		default:
			panic(fmt.Sprintf("code out of range 0-20: %d", L))
		}
		i = j
	}
	return buf
	// Encode the lengths of the bitcodes, in order.
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
		var L, R byte
		switch {
		case b&1 == 0:
			L = 0
			R = b>>1 + 1
		case b&3 == 1:
			L = (b>>2)&15 + 1
			R = b>>6 + 1
		case b&3 == 3:
			L = (b>>2)&3 + 17
			R = b>>4 + 1
		}
		codes = slices.Grow(codes, int(R))
		for range R {
			codes = append(codes, bitcode{len: uint32(L)})
		}
	}
	assignValues(codes)
	return &Code{codes: codes}, nil
}

func assignValues(codes []bitcode) {
	// Assign values to the codes, given their lengths.
	// Algorithm from RFC 1951, section 3.2.2.
	var counts, nextVal [maxCodeLen + 1]uint32
	for _, c := range codes {
		counts[c.len]++
	}
	val := uint32(0)
	counts[0] = 0
	for len := 1; len <= maxCodeLen; len++ {
		val = (val + counts[len-1]) << 1
		nextVal[len] = val
	}
	for i, c := range codes {
		if c.len != 0 {
			codes[i].val = nextVal[c.len]
			nextVal[c.len]++
		}
	}
}

// TODO: is a code for (byte) faster?
// TODO: just panic if out of range?
func (c *Code) code(s Symbol) bitcode {
	if s >= uint32(len(c.codes)) {
		return bitcode{}
	}
	return c.codes[s]
}

// A SplitFunc splits bytes into symbols.
// TODO: return (int, []Symbol), where the int is how many bytes consumed, so SplitFunc
// doesn't have to consume all its input.
type SplitFunc func([]byte) []Symbol

// A CodeBuilder builds A [Code] from a sequence of bytes.
// Call [NewCodeBuilder] to construct one, then write the bytes to it with [CodeBuilder.Write].
// Call [CodeBuilder.Code] to retrieve the finished Code.
type CodeBuilder struct {
	split SplitFunc
	freqs []int
}

// NewCodeBuilder constructs a [CodeBuilder].
// If split is nil, each byte of the input is a separate symbol.
// Otherwise, split is called to split the input bytes into symbols.
func NewCodeBuilder(split SplitFunc) *CodeBuilder {
	return &CodeBuilder{split: split}
}

// Write adds the data to the sequence of symbols used to construct the [Code].
// It always returns a nil error.
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

// Code returns the constructed [Code].
func (cb *CodeBuilder) Code() (*Code, error) {
	return NewCode(cb.freqs)
}

// An Encoder encodes symbols with a [Code] and writes them to an [io.Writer].
// Create one with [NewEncoder], then add data with the Write, WriteBytes, WriteSymbol and WriteSymbols
// methods. Finally, call Close to flush remaining data to the io.Writer.
type Encoder struct {
	c     *Code
	bw    *bitWriter
	split SplitFunc
}

// NewEncoder constructs an [Encoder].
// If split is nil, the [Code] must not have more than 256 symbols (one for each possible byte value).
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

// WriteBytes writes the bytes to the encoder as separate symbols.
// The encoder's split function must be nil, and every byte in the argument
// must have a valid encoding.
func (e *Encoder) WriteBytes(bs []byte) {
	if e.split != nil {
		panic("huffman.Encoder.WriteBytes called with no split function")
	}
	for _, b := range bs {
		e.WriteSymbol(Symbol(b))
	}
}

// WriteSymbol writes a symbol to the encoder.
// It panics if there is no code for the given symbol.
func (e *Encoder) WriteSymbol(s Symbol) {
	// TODO: faster to have a specialized bits(byte)?
	b := e.c.code(s)
	if b.len == 0 {
		panic(fmt.Sprintf("no code for symbol %d", s))
	}
	// TODO: benchmark if WriteBits takes a uint8, or bits.len is an int.
	e.bw.writeBits(b.val, int(b.len))
}

// WriteSymbols calls [WriteSymbol] repeatedly.
func (e *Encoder) WriteSymbols(syms []Symbol) {
	for _, s := range syms {
		e.WriteSymbol(s)
	}
}

// Close writes remaining data to the encoder's writer.
func (e *Encoder) Close() error {
	return e.bw.Close()
}

// A Decoder decodes data encoded by an Encoder.
type Decoder struct {
	table *table
}

func (c *Code) NewDecoder() *Decoder {
	// TODO: build the table once
	return &Decoder{
		table: buildTable(c.codes),
	}
}

// A table maps bytes to actions.
// The index byte might represent a complete 8-bit code, or one that is shorter or longer.
// If 8 or shorter, action.len gives the length in bits, telling the Decoder how much of
// the byte to consume from the input. A new byte is then constructed from the remainder of the
// old byte with additional input bits, and then the table is indexed again.
//
// If the code length exceeds 8 bits, the action points to another table, populated with values
// using the code's remaining bits.
type table [256]action

type action struct {
	sym   Symbol // the symbol that this code represents
	len   uint32 // the length of the code
	table *table // if non-nil, then sym==0, len==8, and the code continues to the next table
}

func buildTable(codes []bitcode) *table {
	t := &table{}
	for s, c := range codes {
		t.add(c.val, c.len, Symbol(s))
	}
	return t
}

func (t *table) add(val, len uint32, sym Symbol) {
	if len <= 8 {
		for i := range 1 << (8 - len) {
			t[int(val)+i] = action{sym: sym, len: len}
		}
	} else {
		panic("unimp")
	}
}

func (d *Decoder) Read(buf []byte) (int, error) {
	return 0, nil
}

func (d *Decoder) DecodeSymbols(buf []Symbol) (int, error) { return 0, nil }

func (d *Decoder) SetEncoded(e []byte) {}
