// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mocks

import (
	time "time"

	"go.skia.org/infra/go/now"
)

func NewTimeTickerFunc(ch <-chan time.Time) now.NewTimeTickerFunc {
	return func(unused time.Duration) now.TimeTicker {
		rv := &TimeTicker{}
		rv.On("C").Return(ch)
		return rv
	}
}
