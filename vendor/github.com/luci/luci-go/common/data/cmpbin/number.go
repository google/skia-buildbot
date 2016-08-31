// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cmpbin

import (
	"errors"
	"io"
	"math"
)

// MaxIntLenN is the maximum length of a cmpbin-encoded N-bit integer
// (signed or unsigned).
const (
	MaxIntLen16 = 3
	MaxIntLen32 = 5
	MaxIntLen64 = 9
)

// ErrOverflow is returned when reading an number which is too large for the
// destination type (either uint64 or int64)
var ErrOverflow = errors.New("cmpbin: varint overflows")

// ErrUnderflow is returned when reading an number which is too small for the
// destination type (either uint64 or int64)
var ErrUnderflow = errors.New("cmpbin: uvarint underflows")

var paddingMasks = [...]uint64{
	0xFFFFFFFF00000000,
	0xFFFF0000,
	0xFF00,
	0xF0,
	0xC,
	0x2,
	0x1,
}

// Calculate the log2 of the unsigned value v.
//
// This is used to find the position of the highest-set bit in v.
//
// from https://graphics.stanford.edu/~seander/bithacks.html#IntegerLog
// 32 bit implementation extended to 64 bits
func uint64Log2(v uint64) uint {
	log := uint(0)
	for i, m := range paddingMasks {
		if v&m != 0 {
			shift := uint(1<<uint(len(paddingMasks)-2)) >> uint(i)
			v >>= shift
			log |= shift
		}
	}
	return log + 1
}

// WriteInt val as a cmpbin Int to the ByteWriter. Returns the number of bytes
// written. Only returns an error if the underlying ByteWriter returns an error.
func WriteInt(w io.ByteWriter, val int64) (int, error) {
	var inv byte
	if val < 0 {
		inv = 0xff
	}
	mag := uint64(val)
	if inv != 0 {
		mag = -mag
	}
	return writeSignMag(w, mag, inv)
}

// WriteUint writes mag to the ByteWriter. Returns the number of bytes written.
// Only returns an error if the underlying ByteWriter returns an error.
func WriteUint(w io.ByteWriter, mag uint64) (int, error) {
	return writeSignMag(w, mag, 0)
}

// ReadInt decodes a cmpbin-encoded number from a ByteReader. It returns the
// decoded value and the number of bytes read. The error may be
// Err{Over,Under}flow if the number is out of bounds. It may also return an
// error if the ByteReader returns an error.
func ReadInt(r io.ByteReader) (ret int64, n int, err error) {
	pos, sigs, mag, n, err := readSignMag(r)
	if err != nil {
		return
	}
	if pos {
		if sigs > 63 {
			err = ErrOverflow
		} else {
			ret = int64(mag)
		}
	} else {
		if mag > uint64(-math.MinInt64) {
			err = ErrUnderflow
		} else {
			ret = int64(-mag)
		}
	}
	return
}

// ReadUint decodes a cmpbin-encoded positive number from a ByteReader.  It
// returns the decoded value and the number of bytes read. The erorr may be
// Err{Over,Under}flow if the number is out of bounds. It may also return an
// error if the ByteReader returns an error.
func ReadUint(r io.ByteReader) (mag uint64, n int, err error) {
	pos, _, mag, n, err := readSignMag(r)
	if err != nil {
		return
	}
	if !pos {
		err = ErrUnderflow
	}
	return
}

func writeSignMag(w io.ByteWriter, mag uint64, inv byte) (n int, err error) {
	sigs := uint64Log2(mag)

	wb := func(b byte) error {
		n++
		return w.WriteByte(b)
	}

	if err = wb(byte(0x80|(sigs-1)) ^ inv); err != nil {
		return
	}

	for sigs > 8 {
		sigs -= 8

		if err = wb(byte(mag>>sigs) ^ inv); err != nil {
			return
		}
	}
	if sigs != 0 {
		if err = wb(byte(mag<<(8-sigs)) ^ inv); err != nil {
			return
		}
	}

	return
}

func readSignMag(r io.ByteReader) (positive bool, sigs uint, mag uint64, n int, err error) {
	var inv byte

	rb := func() (byte, error) {
		n++
		return r.ReadByte()
	}

	b0, err := rb()
	if err != nil {
		return
	}
	positive = true
	if b0&0x80 == 0 {
		positive = false
		inv = 0xff
	}

	sigs = uint((b0^inv)&0x7f) + 1
	if sigs > 64 {
		err = ErrOverflow
		return
	}

	numBytes := int((sigs+7)>>3) + 1

	var b byte
	shift := uint(64 - 8)
	for i := 1; i < numBytes; i++ {
		b, err = rb()
		if err != nil {
			return
		}
		mag |= uint64(b^inv) << shift
		shift -= 8
	}
	mag >>= 64 - sigs

	return
}
