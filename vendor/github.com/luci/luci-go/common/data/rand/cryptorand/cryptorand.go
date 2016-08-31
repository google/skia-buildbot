// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package cryptorand implements a mockable source or crypto strong randomness.
//
// In real world scenario it is same source as provided by crypt/rand. In tests
// it is replaced with reproducible, not really random stream of bytes.
package cryptorand

import (
	crypto "crypto/rand"
	"io"
	math "math/rand"

	"golang.org/x/net/context"
)

var key = "holds crypto.Reader for cryptorand"

// Get returns an io.Reader that emits random stream of bytes.
//
// Usually this returns crypto/rand.Reader, but unit tests may replace it with
// a mock by using 'MockForTest' function.
func Get(c context.Context) io.Reader {
	if r, _ := c.Value(&key).(io.Reader); r != nil {
		return r
	}
	return crypto.Reader
}

// Read is a helper that reads bytes from random source using io.ReadFull.
//
// On return, n == len(b) if and only if err == nil.
func Read(c context.Context, b []byte) (n int, err error) {
	return io.ReadFull(Get(c), b)
}

// MockForTest installs deterministic source of 'randomness' in the context.
//
// Must not be used outside of tests.
func MockForTest(c context.Context, seed int64) context.Context {
	return context.WithValue(c, &key, notRandom{math.New(math.NewSource(seed))})
}

// notRandom is io.Reader that uses math/rand generator.
type notRandom struct {
	*math.Rand
}

func (r notRandom) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(r.Intn(256))
	}
	return len(p), nil
}
