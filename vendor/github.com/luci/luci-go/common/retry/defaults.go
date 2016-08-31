// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"time"

	"golang.org/x/net/context"
)

// defaultIterator defines a template for the default retry parameters that
// should be used throughout the program.
var defaultIteratorTemplate = ExponentialBackoff{
	Limited: Limited{
		Delay:   200 * time.Millisecond,
		Retries: 10,
	},
	MaxDelay:   10 * time.Second,
	Multiplier: 2,
}

// Default is a Factory that returns a new instance of the default iterator
// configuration.
func Default() Iterator {
	it := defaultIteratorTemplate
	return &it
}

type noneItTemplate struct{}

func (noneItTemplate) Next(context.Context, error) time.Duration { return Stop }

// None is a Factory that returns an Iterator that explicitly calls Stop after
// the first try. This is helpful to pass to libraries which use retry.Default
// if given nil, but where you don't want any retries at all (e.g. tests).
func None() Iterator {
	return noneItTemplate{}
}
