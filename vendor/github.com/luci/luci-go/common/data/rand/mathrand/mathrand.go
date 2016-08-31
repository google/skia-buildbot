// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package mathrand implements a mockable interface for math/rand.Rand.
//
// It is controllable through context.Context. You should use this instead of
// math/rand directly, to allow you to make deterministic tests.
package mathrand

import (
	"math/rand"

	"golang.org/x/net/context"
)

var key = "holds a rand.Rand for mathrand"

// Get gets a *"math/rand".Rand from the context.
//
// If one hasn't been set, this creates a new Rand object with a Source
// initialized from the global randomness source provided by stdlib.
func Get(c context.Context) *rand.Rand {
	if r, ok := c.Value(&key).(*rand.Rand); ok && r != nil {
		return r
	}
	return rand.New(rand.NewSource(rand.Int63()))
}

// Set sets the current *"math/rand".Rand object in the context.
//
// Useful for testing with a quick mock.
func Set(c context.Context, r *rand.Rand) context.Context {
	return context.WithValue(c, &key, r)
}
