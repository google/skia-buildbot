// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/rand/mathrand"
	"github.com/luci/luci-go/server/auth/delegation"
	"github.com/luci/luci-go/server/auth/signing"
	"github.com/luci/luci-go/server/auth/signing/signingtest"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMintDelegationToken(t *testing.T) {
	Convey("MintDelegationToken works", t, func() {
		ctx := context.Background()
		ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)
		ctx = mathrand.Set(ctx, rand.New(rand.NewSource(12345)))

		tokenCache := &mockedCache{}

		subtokenID := "123"
		mintingReq := ""
		transport := &clientRPCTransportMock{
			cb: func(r *http.Request, body string) string {
				if r.URL.String() == "https://hostname.example.com/auth/api/v1/server/info" {
					return `{"app_id":"hostname"}`
				}
				if r.URL.String() == "https://auth.example.com/auth_service/api/v1/delegation/token/create" {
					mintingReq = body
					return fmt.Sprintf(`{
						"delegation_token": "tok",
						"validity_duration": 43200,
						"subtoken_id": "%s"
					}`, subtokenID)
				}
				return "unknown URL"
			},
		}

		ctx = ModifyConfig(ctx, func(cfg *Config) {
			cfg.AccessTokenProvider = transport.getAccessToken
			cfg.AnonymousTransport = transport.getTransport
			cfg.GlobalCache = tokenCache
			cfg.Signer = signingtest.NewSigner(0, &signing.ServiceInfo{
				ServiceAccountName: "service@example.com",
			})
		})

		ctx = WithState(ctx, &state{
			user: &User{Identity: "user:abc@example.com"},
			db:   &fakeDB{authServiceURL: "https://auth.example.com"},
		})

		Convey("Works (including caching)", func(c C) {
			tok, err := MintDelegationToken(ctx, DelegationTokenParams{
				TargetHost: "hostname.example.com",
				MinTTL:     time.Hour,
				Intent:     "intent",
			})
			So(err, ShouldBeNil)
			So(tok, ShouldResemble, &delegation.Token{
				Token:      "tok",
				SubtokenID: "123",
				Expiry:     testclock.TestRecentTimeUTC.Add(MaxDelegationTokenTTL),
			})
			So(mintingReq, ShouldEqual,
				`{"audience":["user:service@example.com"],`+
					`"services":["service:hostname"],"validity_duration":43200,`+
					`"impersonate":"user:abc@example.com","intent":"intent"}`)

			// Cached now.
			So(len(tokenCache.data), ShouldEqual, 1)
			for k := range tokenCache.data {
				So(k, ShouldEqual, "delegation/2/R5RJ9yppAB8IK0GNB-UyjVrYoBw")
			}

			// On subsequence request the cached token is used.
			subtokenID = "456"
			tok, err = MintDelegationToken(ctx, DelegationTokenParams{
				TargetHost: "hostname.example.com",
				MinTTL:     time.Hour,
				Intent:     "intent",
			})
			So(err, ShouldBeNil)
			So(tok.SubtokenID, ShouldResemble, "123") // old one

			// Unless it expires sooner than requested TTL.
			clock.Get(ctx).(testclock.TestClock).Add(MaxDelegationTokenTTL - 30*time.Minute)
			tok, err = MintDelegationToken(ctx, DelegationTokenParams{
				TargetHost: "hostname.example.com",
				MinTTL:     time.Hour,
				Intent:     "intent",
			})
			So(err, ShouldBeNil)
			So(tok.SubtokenID, ShouldResemble, "456") // new one
		})

		Convey("Untargeted token works", func(c C) {
			tok, err := MintDelegationToken(ctx, DelegationTokenParams{
				Untargeted: true,
				MinTTL:     time.Hour,
				Intent:     "intent",
			})
			So(err, ShouldBeNil)
			So(tok, ShouldResemble, &delegation.Token{
				Token:      "tok",
				SubtokenID: "123",
				Expiry:     testclock.TestRecentTimeUTC.Add(MaxDelegationTokenTTL),
			})
			So(mintingReq, ShouldEqual,
				`{"audience":["user:service@example.com"],`+
					`"services":["*"],"validity_duration":43200,`+
					`"impersonate":"user:abc@example.com","intent":"intent"}`)

			// Cached now.
			So(len(tokenCache.data), ShouldEqual, 1)
			for k := range tokenCache.data {
				So(k, ShouldEqual, "delegation/2/tjYIGNrwFvKa0FT5juu7ThjpxBo")
			}
		})
	})
}
