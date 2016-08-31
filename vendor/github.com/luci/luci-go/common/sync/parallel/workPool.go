// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"github.com/luci/luci-go/common/errors"
)

// WorkPool creates a fixed-size pool of worker goroutines. A supplied generator
// method creates task functions and passes them through to the work pool.
//
// WorkPool will use at most workers goroutines to execute the supplied tasks.
// If workers is <= 0, WorkPool will be unbounded and behave like FanOutIn.
//
// WorkPool blocks until all the generator completes and all workers have
// finished their tasks.
func WorkPool(workers int, gen func(chan<- func() error)) error {
	return errors.MultiErrorFromErrors(Run(workers, gen))
}

// FanOutIn is useful to quickly parallelize a group of tasks.
//
// You pass it a function which is expected to push simple `func() error`
// closures into the provided chan. Each function will be executed in parallel
// and their error results will be collated.
//
// The function blocks until all functions are executed, and an
// errors.MultiError is returned if one or more of your fan-out tasks failed,
// otherwise this function returns nil.
//
// This function is equivalent to WorkPool(0, gen).
func FanOutIn(gen func(chan<- func() error)) error {
	return WorkPool(0, gen)
}
