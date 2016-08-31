// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package client

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/info"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/common/data/rand/mathrand"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetAccessToken(t *testing.T) {
	Convey("GetAccessToken works", t, func() {
		c := testContext()

		// Getting initial token.
		ctx := mockAccessTokenRPC(c, []string{"A", "B"}, "access_token_1", testclock.TestRecentTimeUTC.Add(time.Hour))
		tok, err := GetAccessToken(ctx, []string{"B", "B", "A"})
		So(err, ShouldBeNil)
		So(tok, ShouldResemble, auth.Token{
			AccessToken: "access_token_1",
			TokenType:   "Bearer",
			Expiry:      testclock.TestRecentTimeUTC.Add(time.Hour).Add(-expirationMinLifetime),
		})

		// Some time later same cached token is used.
		clock.Get(c).(testclock.TestClock).Add(30 * time.Minute)

		ctx = mockAccessTokenRPC(c, []string{"A", "B"}, "access_token_none", testclock.TestRecentTimeUTC.Add(time.Hour))
		tok, err = GetAccessToken(ctx, []string{"B", "B", "A"})
		So(err, ShouldBeNil)
		So(tok, ShouldResemble, auth.Token{
			AccessToken: "access_token_1",
			TokenType:   "Bearer",
			Expiry:      testclock.TestRecentTimeUTC.Add(time.Hour).Add(-expirationMinLifetime),
		})

		// Closer to expiration, the token is updated, at some random invocation,
		// (depends on the seed, defines the loop limit in the test).
		clock.Get(c).(testclock.TestClock).Add(19 * time.Minute)
		for i := 0; i < 12; i++ {
			ctx = mockAccessTokenRPC(c, []string{"A", "B"}, fmt.Sprintf("access_token_%d", i+2), testclock.TestRecentTimeUTC.Add(2*time.Hour))
			tok, err = GetAccessToken(ctx, []string{"B", "B", "A"})
			So(err, ShouldBeNil)
		}
		So(tok, ShouldResemble, auth.Token{
			AccessToken: "access_token_13",
			TokenType:   "Bearer",
			Expiry:      testclock.TestRecentTimeUTC.Add(2 * time.Hour).Add(-expirationMinLifetime),
		})

		// No randomization for token that are long expired.
		clock.Get(c).(testclock.TestClock).Add(2 * time.Hour)
		ctx = mockAccessTokenRPC(c, []string{"A", "B"}, "access_token_new", testclock.TestRecentTimeUTC.Add(5*time.Hour))
		tok, err = GetAccessToken(ctx, []string{"B", "B", "A"})
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "access_token_new")
	})
}

type mockedInfo struct {
	info.RawInterface

	scopes []string
	tok    string
	exp    time.Time
}

func (m *mockedInfo) AccessToken(scopes ...string) (string, time.Time, error) {
	So(scopes, ShouldResemble, m.scopes)
	return m.tok, m.exp, nil
}

func testContext() context.Context {
	ctx := context.Background()
	ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)
	ctx = memory.Use(ctx)
	ctx = mathrand.Set(ctx, rand.New(rand.NewSource(2)))
	ctx = proccache.Use(ctx, &proccache.Cache{})
	return ctx
}

func mockAccessTokenRPC(c context.Context, scopes []string, tok string, exp time.Time) context.Context {
	return info.AddFilters(c, func(ci context.Context, i info.RawInterface) info.RawInterface {
		return &mockedInfo{i, scopes, tok, exp}
	})
}
