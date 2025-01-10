# Huffman encoding

This package implements Huffman encoding with the following features:


- Alphabet sizes up to 2<sup>32</sup>.

- Separate representation of the code (mapping from alphabet to code bits).

- Ability to marshal a code to bytes, and unmarshal it from bytes.

- Encoding tokens as well as bytes.
 
There are many Huffman implementations, but I haven't found one that has all of these features.
