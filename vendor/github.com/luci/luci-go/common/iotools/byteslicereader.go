// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"io"
)

// ByteSliceReader is an io.Reader and io.ByteReader implementation that reads
// and mutates an underlying byte slice.
type ByteSliceReader []byte

var _ interface {
	io.Reader
	io.ByteReader
} = (*ByteSliceReader)(nil)

// Read implements io.Reader.
func (r *ByteSliceReader) Read(buf []byte) (int, error) {
	count := copy(buf, *r)
	*r = (*r)[count:]
	if len(*r) == 0 {
		*r = nil
	}
	if count < len(buf) {
		return count, io.EOF
	}
	return count, nil
}

// ReadByte implements io.ByteReader.
func (r *ByteSliceReader) ReadByte() (byte, error) {
	d := []byte{0}
	_, err := r.Read(d)
	return d[0], err
}
