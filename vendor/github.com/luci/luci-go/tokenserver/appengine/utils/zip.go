// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"bytes"
	"compress/zlib"
	"io"
)

// ZlibCompress zips a blob using zlib.
func ZlibCompress(in []byte) ([]byte, error) {
	out := bytes.Buffer{}
	w := zlib.NewWriter(&out)
	_, writeErr := w.Write(in)
	closeErr := w.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return out.Bytes(), nil
}

// ZlibDecompress unzips a blob using zlib.
func ZlibDecompress(in []byte) ([]byte, error) {
	out := bytes.Buffer{}
	r, err := zlib.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	_, readErr := io.Copy(&out, r)
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return out.Bytes(), nil
}
