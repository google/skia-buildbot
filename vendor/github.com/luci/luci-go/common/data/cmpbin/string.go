// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cmpbin

import (
	"bytes"
	"errors"
	"io"
	"math"
)

// ReadByteLimit is the limit of how many bytes ReadBytes and ReadString are
// willing to deserialize before returning ErrByteLimitExceeded. It is currently
// set to allow 2MB of user data (taking encoding size overhead into account).
var ReadByteLimit = int(math.Ceil(2 * 1024 * 1024 * 8 / 7))

// ErrByteLimitExceeded is returned from ReadBytes and ReadString when they
// attempt to read more than ReadByteLimit bytes.
var ErrByteLimitExceeded = errors.New("cmbpin: too big! tried to read > cmpbin.ReadByteLimit")

// WriteString writes an encoded string to buf, returning the number of bytes
// written, and any write error encountered.
func WriteString(buf io.ByteWriter, s string) (n int, err error) {
	return WriteBytes(buf, []byte(s))
}

// ReadString reads an encoded string from buf, returning the number of bytes
// read, and any read error encountered.
func ReadString(buf io.ByteReader) (ret string, n int, err error) {
	b, n, err := ReadBytes(buf)
	if err != nil {
		return
	}
	ret = string(b)
	return
}

// WriteBytes writes an encoded []byte to buf, returning the number of bytes
// written, and any write error encountered.
func WriteBytes(buf io.ByteWriter, data []byte) (n int, err error) {
	wb := func(b byte) (err error) {
		if err = buf.WriteByte(b); err == nil {
			n++
		}
		return
	}

	acc := byte(0)
	for i := 0; i < len(data); i++ {
		m := uint(i % 7)
		b := data[i]
		if err = wb(acc | 1 | ((b & (0xff << (m + 1))) >> m)); err != nil {
			return
		}
		acc = (b << (7 - m))
		if m == 6 {
			if err = wb(acc | 1); err != nil {
				return
			}
			acc = 0
		}
	}
	err = wb(acc)
	return
}

// ReadBytes reads an encoded []byte from buf, returning the number of bytes
// read, and any read error encountered.
func ReadBytes(buf io.ByteReader) (ret []byte, n int, err error) {
	tmpBuf := bytes.Buffer{}
	acc := byte(0)
	for i := 0; i < ReadByteLimit; i++ {
		o := byte(0)
		if o, err = buf.ReadByte(); err != nil {
			return
		}
		n++

		b := o & 0xfe // user data
		m := uint(i % 8)

		if m == 0 {
			acc = b
		} else {
			// ignore err since bytes.Buffer.WriteByte can never return one.
			_ = tmpBuf.WriteByte(acc | (b >> (8 - m)))
			acc = (b << m)
		}

		if o&1 == 0 { // stop bit is 0
			ret = tmpBuf.Bytes()
			return
		}
	}
	err = ErrByteLimitExceeded
	return
}
