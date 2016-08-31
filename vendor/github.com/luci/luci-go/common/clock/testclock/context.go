// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testclock

import (
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"
)

// TestTimeUTC is an arbitrary time point in UTC for testing.
//
// It corresponds to a negative Unix timestamp, so it might be inappropriate for
// some tests. Use 'TestRecentTimeUTC' for more "real" test time in this case.
var TestTimeUTC = time.Date(1, time.February, 3, 4, 5, 6, 7, time.UTC)

// TestRecentTimeUTC is like TestTimeUTC, but in the recent past.
var TestRecentTimeUTC = time.Date(2016, time.February, 3, 4, 5, 6, 7, time.UTC)

// TestTimeLocal is an arbitrary time point in the 'Local' time zone for
// testing.
//
// It corresponds to a negative Unix timestamp, so it might be inappropriate for
// some tests. Use 'TestRecentTimeLocal' for more "real" test time in this case.
var TestTimeLocal = time.Date(1, time.February, 3, 4, 5, 6, 7, time.Local)

// TestRecentTimeLocal is like TestTimeLocal, but in the recent past.
var TestRecentTimeLocal = time.Date(2016, time.February, 3, 4, 5, 6, 7, time.Local)

// UseTime instantiates a TestClock and returns a Context that is configured to
// use that clock, as well as the instantiated clock.
func UseTime(ctx context.Context, now time.Time) (context.Context, TestClock) {
	tc := New(now)
	return clock.Set(ctx, tc), tc
}
