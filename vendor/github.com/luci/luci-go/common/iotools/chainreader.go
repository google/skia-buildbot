// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"errors"
	"io"
)

// ChainReader is an io.Reader that consumes data sequentially from independent
// arrays of data to appear as if they were one single concatenated data source.
//
// The underlying io.Reader will be mutated during operation.
type ChainReader []io.Reader

var _ interface {
	io.Reader
	io.ByteReader
} = (*ChainReader)(nil)

// Read implements io.Reader.
func (cr *ChainReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	consumed := 0
	defer func() {
		*cr = (*cr)[consumed:]
	}()

	total := 0
	for idx, source := range *cr {
		if source == nil {
			consumed++
			continue
		}

		count, err := source.Read(p)
		total += count
		if err == io.EOF {
			(*cr)[idx] = nil
			consumed++
		} else if err != nil {
			return total, err
		}

		p = p[count:]
		if len(p) == 0 {
			return total, nil
		}
	}
	return total, io.EOF
}

// ReadByte implements io.ByteReader.
func (cr ChainReader) ReadByte() (byte, error) {
	d := []byte{0}
	_, err := cr.Read(d)
	return d[0], err
}

// Remaining calculates the amount of data left in the ChainReader. It will
// panic if an error condition in RemainingErr is encountered.
func (cr ChainReader) Remaining() int64 {
	result, err := cr.RemainingErr()
	if err != nil {
		panic(err)
	}
	return result
}

// RemainingErr returns the amount of data left in the ChainReader. An error is
// returned if any reader in the chain is not either nil or a bytes.Reader.
//
// Note that this method iterates over all readers in the chain each time that
// it's called.
func (cr ChainReader) RemainingErr() (int64, error) {
	result := int64(0)
	for _, source := range cr {
		if source == nil {
			continue
		}
		r, ok := source.(interface {
			Len() int
		})
		if !ok {
			return 0, errors.New("chainreader: can only calculate Remaining for instances implementing Len()")
		}
		result += int64(r.Len())
	}
	return result, nil
}
