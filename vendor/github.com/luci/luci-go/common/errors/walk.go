// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

// Walk performs a depth-first traversal of the supplied error, unfolding it and
// invoke the supplied callback for each layered error recursively. If the
// callback returns true, Walk will continue its traversal.
//
//	- If walk encounters a MultiError, the callback is called once for the
//	  outer MultiError, then once for each inner error.
//	- If walk encounters a Wrapped error, the callback is called for the outer
//	  and inner error.
//	- If an inner error is, itself, a container, Walk will recurse into it.
//
// If err is nil, the callback will not be invoked.
func Walk(err error, fn func(error) bool) {
	_ = walkVisit(err, fn)
}

func walkVisit(err error, fn func(error) bool) bool {
	if err == nil {
		return true
	}

	if !fn(err) {
		return false
	}

	switch t := err.(type) {
	case MultiError:
		for _, e := range t {
			if !walkVisit(e, fn) {
				return false
			}
		}

	case Wrapped:
		return walkVisit(t.InnerError(), fn)
	}

	return true
}

// Any performs a Walk traversal of an error, returning true (and
// short-circuiting) if the supplied filter function returns true for any
// visited erorr.
//
// If err is nil, Any will return false.
func Any(err error, fn func(error) bool) (any bool) {
	Walk(err, func(err error) bool {
		any = fn(err)
		return !any
	})
	return
}

// Contains performs a Walk traversal of an error, returning true if it is or
// contains the supplied sentinel error.
func Contains(err, sentinel error) bool {
	return Any(err, func(err error) bool {
		return err == sentinel
	})
}
