// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"time"

	"github.com/luci/luci-go/common/errors"
	"golang.org/x/net/context"
)

// transientOnlyIterator is an Iterator implementation that only retries errors
// if they are transient.
//
// (See errors.IsTransient).
type transientOnlyIterator struct {
	Iterator // The wrapped Iterator.
}

func (i *transientOnlyIterator) Next(ctx context.Context, err error) time.Duration {
	if !errors.IsTransient(err) {
		return Stop
	}
	return i.Iterator.Next(ctx, err)
}

// TransientOnly returns an Iterator that wraps another Iterator. It will fall
// through to the wrapped Iterator if a transient error is encountered;
// otherwise, it will not retry.
// Returns nil if f is nil.
func TransientOnly(f Factory) Factory {
	if f == nil {
		return nil
	}
	return wrap(f, func(it Iterator) Iterator {
		return &transientOnlyIterator{it}
	})
}
