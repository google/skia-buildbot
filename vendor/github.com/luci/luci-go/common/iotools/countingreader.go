// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"io"
)

// CountingReader is an io.Reader that counts the number of bytes that are read.
type CountingReader struct {
	io.Reader // The underlying io.Reader.

	count int64
}

var _ io.Reader = (*CountingReader)(nil)

// Read implements io.Reader.
func (c *CountingReader) Read(buf []byte) (int, error) {
	amount, err := c.Reader.Read(buf)
	c.count += int64(amount)
	return amount, err
}

// ReadByte implements io.ByteReader.
func (c *CountingReader) ReadByte() (byte, error) {
	// If our underlying reader is a ByteReader, use its ReadByte directly.
	if br, ok := c.Reader.(io.ByteReader); ok {
		b, err := br.ReadByte()
		if err == nil {
			c.count++
		}
		return b, err
	}

	data := []byte{0}
	amount, err := c.Reader.Read(data)
	if amount != 0 {
		c.count += int64(amount)
		return data[0], err
	}
	return 0, err
}

// Count returns the number of bytes that have been read.
func (c *CountingReader) Count() int64 {
	return c.count
}
