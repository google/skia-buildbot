// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"math/rand"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/rand/mathrand"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTokenCache(t *testing.T) {
	Convey("with mocked cache", t, func() {
		cache := &mockedCache{}

		ctx := context.Background()
		ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)
		ctx = mathrand.Set(ctx, rand.New(rand.NewSource(12345)))
		ctx = SetConfig(ctx, Config{GlobalCache: cache})

		tc := tokenCache{
			Kind:           "testing",
			Version:        1,
			ExpRandPercent: 10,
		}

		Convey("check basic usage", func() {
			itm, err := tc.Fetch(ctx, "some\nkey")
			So(err, ShouldBeNil)
			So(itm, ShouldBeNil)

			tok := cachedToken{
				Key:     "some\nkey",
				Token:   "blah",
				Created: clock.Now(ctx).UTC(),
				Expiry:  clock.Now(ctx).Add(100 * time.Minute).UTC(),
			}
			So(tc.Store(ctx, tok), ShouldBeNil)

			itm, err = tc.Fetch(ctx, "some\nkey")
			So(err, ShouldBeNil)
			So(itm, ShouldResemble, &tok)

			So(len(cache.data), ShouldEqual, 1)
			for key := range cache.data {
				So(key, ShouldEqual, "testing/1/na7cNnVNdP6Ydb9K9UIEST04OV8")
			}
		})

		Convey("check expiration randomization", func() {
			tok := cachedToken{
				Key:     "some\nkey",
				Token:   "blah",
				Created: clock.Now(ctx).UTC(),
				Expiry:  clock.Now(ctx).Add(100 * time.Minute).UTC(),
			}
			So(tc.Store(ctx, tok), ShouldBeNil)

			// 89% of token's life has passed. The token is still alive
			// (no randomization).
			clock.Get(ctx).(testclock.TestClock).Add(89 * time.Minute)
			for i := 0; i < 50; i++ {
				itm, _ := tc.Fetch(ctx, "some\nkey")
				So(itm, ShouldNotBeNil)
			}

			// 91% of token's life has passed. 'Fetch' pretends the token is not
			// there with ~=10% chance.
			clock.Get(ctx).(testclock.TestClock).Add(2 * time.Minute)
			missing := 0
			for i := 0; i < 100; i++ {
				if itm, _ := tc.Fetch(ctx, "some\nkey"); itm == nil {
					missing++
				}
			}
			So(missing, ShouldEqual, 10)

			// Token has expired. 0% chance of seeing it.
			clock.Get(ctx).(testclock.TestClock).Add(9 * time.Minute)
			for i := 0; i < 50; i++ {
				itm, _ := tc.Fetch(ctx, "some\nkey")
				So(itm, ShouldBeNil)
			}
		})
	})
}

// mockedCache is GlobalCache implementation for tests.
type mockedCache struct {
	data map[string][]byte
}

func (mc *mockedCache) Get(c context.Context, key string) ([]byte, error) {
	return mc.data[key], nil
}

func (mc *mockedCache) Set(c context.Context, key string, value []byte, exp time.Duration) error {
	if mc.data == nil {
		mc.data = map[string][]byte{}
	}
	mc.data[key] = value
	return nil
}
