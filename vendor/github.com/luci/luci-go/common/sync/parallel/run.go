// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

// Run executes a generator function, dispatching each generated task to the
// Runner. Run returns immediately with an error channel that can be used to
// reap the results of those tasks.
//
// The returned error channel must be consumed, or it can block additional
// functions from being run from gen. A common consumption function is
// errors.MultiErrorFromErrors, which will buffer all non-nil errors into an
// errors.MultiError. Other functions to consider are Must and Ignore (in this
// package).
//
// Note that there is no association between error channel's error order and
// the generated task order. However, the channel will return exactly one error
// result for each generated task.
//
// If workers is <= 0, it will be unbounded; otherwise, a pool of at most
// workers sustained goroutines will be used to execute the task.
func Run(workers int, gen func(chan<- func() error)) <-chan error {
	r := Runner{
		Maximum:   workers,
		Sustained: workers,
	}
	return r.runThen(gen, r.Close)
}
