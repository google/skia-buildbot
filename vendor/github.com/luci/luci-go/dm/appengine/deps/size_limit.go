// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"sync/atomic"
)

type sizeLimit struct {
	// current must only be accessed via sync/atomic
	current uint32

	// is read-only and may be accessed directly.
	max uint32
}

func (s *sizeLimit) PossiblyOK(amount uint32) bool {
	if s.max == 0 {
		return true
	}
	if amount > s.max {
		return false
	}
	// 0 <= 10 - 6
	return atomic.LoadUint32(&s.current) <= (s.max - amount)
}

func (s *sizeLimit) Add(amount uint32) bool {
	if s.max == 0 {
		return true
	}
	if amount > s.max {
		return false
	}
	lim := s.max - amount
	for {
		cur := atomic.LoadUint32(&s.current)
		if cur > lim {
			return false
		}
		if atomic.CompareAndSwapUint32(&s.current, cur, cur+amount) {
			return true
		}
	}
}
