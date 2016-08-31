// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

// Must can be used to consume the channel from Run. It asserts that none of
// the functions run return an error. If one returns non-nil, this will panic
// with the first error encountered (which may cause the channel to remain open
// and unprocessed, blocking other tasks).
func Must(ch <-chan error) {
	for err := range ch {
		if err != nil {
			panic(err)
		}
	}
}

// Ignore can be used to consume the channel from Run. It blocks on all errors
// in the channel and discards them.
func Ignore(ch <-chan error) {
	for range ch {
	}
}
