# Huffman encoding

This package implements Huffman encoding with the following features.
There are many Huffman implementations, but I haven't found one that has all of these features.

- Alphabet sizes up to 2<sup>32</sup>.

- Separate representation of the code (that is, the mapping from alphabet to code bits).

- Ability to marshal/unmarshal a code to/from bytes.

- Encoding tokens as well as bytes.
