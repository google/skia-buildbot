// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package cmpbin provides binary serialization routines which ensure that the
// serialized objects maintain the same sort order of the original inputs when
// sorted bytewise (i.e. with memcmp).  Additionally, serialized objects are
// concatenatable, and the concatenated items will behave as if they're compared
// field-to-field. So, for example, comparing each string in a []string would
// compare the same way as comparing the concatenation of those strings encoded
// with cmpbin. Simply concatenating the strings without encoding them will
// NOT retain this property, as you could not distinguish []string{"a", "aa"}
// from []string{"aa", "a"}. With cmpbin, these two would unambiguously sort as
// ("a", "aa") < ("aa", "a").
//
// Notes on particular serialization schemes:
//
// - Numbers:
// The number encoding is less efficient on average than Varint
// ("encoding/binary") for small numbers (it has a minimum encoded size of
// 2 bytes), but is more efficient for large numbers (it has a maximum encoded
// size of 9 bytes for a 64 bit int, unlike the largest Varint which has a 10b
// representation).
//
// Both signed and unsigned numbers are encoded with the same scheme, and will
// sort together as signed numbers. Decoding with the incorrect routine will
// result in an ErrOverflow/ErrUnderflow error if the actual value is out of
// range.
//
// The scheme works like:
//   - given an 2's compliment value V
//   - extract the sign (S) and magnitude (M) of V
//   - Find the position of the highest bit (P), minus 1.
//   - write (bits):
//     - SPPPPPPP MMMMMMMM MM000000
//     - S is 1
//     - P's are the log2(M)-1
//     - M's are the magnitude of V
//     - 0's are padding
//   - Additionally, if the number is negative, invert the bits of all the bytes
//     (e.g. XOR 0xFF). This makes the sign bit S 0 for negative numbers, and
//     makes the ordering of the numbers correct when compared bytewise.
//
// - Strings/[]byte
// Each byte in the encoded stream reserves the least significant bit as a stop
// bit (1 means that the string continues, 0 means that the string ends). The
// actual user data is shifted into the top 7 bits of every encoded byte. This
// results in a data inflation rate of 12.5%, but this overhead is constant
// (doesn't vary by the encoded content). Note that if space efficiency is very
// important and you are storing large strings on average, you could reduce the
// overhead by only placing the stop bit on every other byte or every 4th byte,
// etc. This would reduce the overhead to 6.25% or 3.125% accordingly (but would
// cause every string to round out to 2 or 4 byte chunks), and it would make
// the algorithm implementation more complex. The current implementation was
// chosen as good enough in light of the fact that pre-compressing regular data
// could save more than 12.5% overall, and that for incompressable data a
// commonly used encoding scheme (base64) has a full 25% overhead (and a
// generally more complex implementation).
//
// - Floats
// Floats are tricky (really tricky) because they have lots of weird
// non-sortable special cases (like NaN). That said, for the majority of
// non-weird cases, the implementation here will sort real numbers the way that
// you would expect.
//
// The implementation is derived from http://stereopsis.com/radix.html, and full
// credit for the original algorithm goes to Michael Herf. The algorithm is
// essentially:
//
//   - if the number is positive, flip the top bit
//   - if the number is negative, flip all the bits
//
// Floats are not varint encoded, you could varint encode the mantissa
// (significand). This is only a 52 bit section, meaning that it is normally
// encoded with 6.5 bytes (a nybble is stolen from the second exponent byte).
// Assuming you used the numerical encoding above, shifted left by 4 bits,
// discarding the sign bit (since its already the MSb on the float, and then
// using 6 bits (instead of 7) to represent the number of significant bits in
// the mantissa (since there are only a maximum of 52), you could expect to see
// small-mantissa floats (of any characteristic) encoded in 3 bytes (this has
// 6 bits of mantissa), and the largest floats would have an encoded size of
// 9 bytes (with 2 wasted bits). However the implementation complexity would be
// higher.
//
// The actual encoded values for special cases are (sorted high to low):
//   - QNaN                    - 0xFFF8000000000000
//     // note that golang doesn't seem to actually have SNaN?
//   - SNaN                    - 0xFFF0000000000001
//   - +inf                    - 0xFFF0000000000000
//   - MaxFloat64              - 0xFFEFFFFFFFFFFFFF
//   - SmallestNonzeroFloat64  - 0x8000000000000001
//   - 0                       - 0x8000000000000000
//   - -0                      - 0x7FFFFFFFFFFFFFFF
//   - -SmallestNonzeroFloat64 - 0x7FFFFFFFFFFFFFFE
//   - -MaxFloat64             - 0x0010000000000000
//   - -inf                    - 0x000FFFFFFFFFFFFF
package cmpbin
