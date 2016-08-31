// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/luci/luci-go/common/auth/internal"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/errors"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	now    = time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)
	past   = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	future = now.Add(24 * time.Hour)
)

func TestAuthenticatedClient(t *testing.T) {
	Convey("Test login required", t, func() {
		auth, _ := newTestAuthenticator(InteractiveLogin, &fakeTokenProvider{
			interactive: true,
		}, nil)
		t, err := auth.Transport()
		So(err, ShouldBeNil)
		So(t, ShouldNotEqual, http.DefaultTransport)
	})

	Convey("Test login not required", t, func() {
		auth, _ := newTestAuthenticator(OptionalLogin, &fakeTokenProvider{
			interactive: true,
		}, nil)
		t, err := auth.Transport()
		So(err, ShouldBeNil)
		So(t, ShouldEqual, http.DefaultTransport)
	})
}

func TestRefreshToken(t *testing.T) {
	Convey("Test non interactive auth (no cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive: false,
			tokenToMint: &oauth2.Token{AccessToken: "minted"},
		}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, nil)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Token is lazily loaded below.
		So(auth.currentToken(), ShouldBeNil)

		// The token is minted on first request.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "minted")
	})

	Convey("Test non interactive auth (with non-expired cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive: false,
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: future}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Token is lazily loaded below.
		So(auth.currentToken(), ShouldBeNil)

		// Cached token is used.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "cached")
	})

	Convey("Test non interactive auth (with expired cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:    false,
			tokenToRefresh: &oauth2.Token{AccessToken: "refreshed"},
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Token is lazily loaded below.
		So(auth.currentToken(), ShouldBeNil)

		// The usage triggers refresh procedure.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "refreshed")
	})

	Convey("Test interactive auth (no cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive: true,
			tokenToMint: &oauth2.Token{AccessToken: "minted"},
		}

		// No token cached, login is required.
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, nil)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldEqual, ErrLoginRequired)
		So(auth.currentToken(), ShouldBeNil)

		// Do it.
		err = auth.Login()
		So(err, ShouldBeNil)
		_, err = auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Minted initial token.
		So(auth.currentToken().AccessToken, ShouldEqual, "minted")

		// And it is actually used.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "minted")
	})

	Convey("Test interactive auth (with non-expired cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive: true,
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: future}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)

		// No need to login, already have a token.
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Loaded cached token.
		So(auth.currentToken().AccessToken, ShouldEqual, "cached")

		// And it is actually used.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "cached")
	})

	Convey("Test interactive auth (with expired cache)", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:    true,
			tokenToRefresh: &oauth2.Token{AccessToken: "refreshed"},
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)

		// No need to login, already have a token. Only its "access_token" part is
		// expired. Refresh token part is still valid, so no login is required.
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Loaded cached token.
		So(auth.currentToken().AccessToken, ShouldEqual, "cached")

		// Attempting to use it triggers a refresh.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "refreshed")
	})

	Convey("Test revoked refresh_token", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:  true,
			revokedToken: true,
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)

		// No need to login, already have a token. Only its "access_token" part is
		// expired. Refresh token part is still presumably valid, there's no way to
		// detect that it has been revoked without attempting to use it.
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Loaded cached token.
		So(auth.currentToken().AccessToken, ShouldEqual, "cached")

		// Attempting to use it triggers a refresh that fails.
		_, err = auth.GetAccessToken(time.Minute)
		So(err, ShouldEqual, ErrLoginRequired)
	})

	Convey("Test revoked credentials", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:  false,
			revokedCreds: true,
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Attempting to use expired cached token triggers a refresh that fails.
		_, err = auth.GetAccessToken(time.Minute)
		So(err, ShouldEqual, ErrBadCredentials)
	})

	Convey("Test transient errors when refreshing, success", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:            false,
			transientRefreshErrors: 5,
			tokenToRefresh:         &oauth2.Token{AccessToken: "refreshed"},
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, _ := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Attempting to use expired cached token triggers a refresh that fails a
		// bunch of times, but the succeeds.
		tok, err := auth.GetAccessToken(time.Minute)
		So(err, ShouldBeNil)
		So(tok.AccessToken, ShouldEqual, "refreshed")

		// All calls were actually made.
		So(tokenProvider.transientRefreshErrors, ShouldEqual, 0)
	})

	Convey("Test transient errors when refreshing, timeout", t, func() {
		tokenProvider := &fakeTokenProvider{
			interactive:            false,
			transientRefreshErrors: 5000, // never succeeds
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: past}
		auth, ctx := newTestAuthenticator(SilentLogin, tokenProvider, tokenInCache)
		_, err := auth.TransportIfAvailable()
		So(err, ShouldBeNil)

		// Attempting to use expired cached token triggers a refresh that constantly
		// fails. Eventually we give up (on 17th attempt).
		before := clock.Now(ctx)
		_, err = auth.GetAccessToken(time.Minute)
		So(err, ShouldErrLike, "transient error")
		after := clock.Now(ctx)

		// It took reasonable amount of time and number of attempts.
		So(after.Sub(before), ShouldBeLessThan, 15*time.Second)
		So(5000-tokenProvider.transientRefreshErrors, ShouldEqual, 17)
	})
}

func TestTransport(t *testing.T) {
	Convey("Test transport works", t, func(c C) {
		calls := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			switch r.URL.Path {
			case "/1":
				c.So(r.Header.Get("Authorization"), ShouldEqual, "Bearer minted")
			case "/2":
				c.So(r.Header.Get("Authorization"), ShouldEqual, "Bearer minted")
			case "/3":
				c.So(r.Header.Get("Authorization"), ShouldEqual, "Bearer refreshed")
			default:
				c.So(r.URL.Path, ShouldBeBlank) // just fail in some helpful way
			}
			w.WriteHeader(200)
		}))
		defer ts.Close()

		tokenProvider := &fakeTokenProvider{
			interactive:    false,
			tokenToMint:    &oauth2.Token{AccessToken: "minted", Expiry: now.Add(time.Hour)},
			tokenToRefresh: &oauth2.Token{AccessToken: "refreshed", Expiry: now.Add(2 * time.Hour)},
		}
		auth, ctx := newTestAuthenticator(SilentLogin, tokenProvider, nil)
		client, err := auth.Client()
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)

		// Initial call will mint new token.
		resp, err := client.Get(ts.URL + "/1")
		So(err, ShouldBeNil)
		ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		// Minted token is now cached.
		So(auth.currentToken().AccessToken, ShouldEqual, "minted")
		cached, err := internal.UnmarshalToken(auth.cache.(*fakeTokenCache).cache)
		So(err, ShouldBeNil)
		So(cached.AccessToken, ShouldEqual, "minted")

		// 40 minutes later it is still OK to use.
		clock.Get(ctx).(testclock.TestClock).Add(40 * time.Minute)
		resp, err = client.Get(ts.URL + "/2")
		So(err, ShouldBeNil)
		ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		// 30 min later (70 min since the start) it is expired and refreshed.
		clock.Get(ctx).(testclock.TestClock).Add(30 * time.Minute)
		resp, err = client.Get(ts.URL + "/3")
		So(err, ShouldBeNil)
		ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		So(auth.currentToken().AccessToken, ShouldEqual, "refreshed")

		// All calls are actually made.
		So(calls, ShouldEqual, 3)
	})
}

func TestOptionalLogin(t *testing.T) {
	Convey("Test optional login works", t, func(c C) {
		// This test simulates following scenario for OptionalLogin mode:
		//   1. There's existing cached access token.
		//   2. At some point it expires.
		//   3. Refresh fails with ErrBadRefreshToken (refresh token is revoked).
		//   4. Authenticator switches to anonymous calls.
		calls := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			switch r.URL.Path {
			case "/1":
				c.So(r.Header.Get("Authorization"), ShouldEqual, "Bearer cached")
			case "/2":
				c.So(r.Header.Get("Authorization"), ShouldEqual, "")
			default:
				c.So(r.URL.Path, ShouldBeBlank) // just fail in some helpful way
			}
			w.WriteHeader(200)
		}))
		defer ts.Close()

		tokenProvider := &fakeTokenProvider{
			interactive:  true,
			revokedToken: true,
		}
		tokenInCache := &oauth2.Token{AccessToken: "cached", Expiry: now.Add(time.Hour)}
		auth, ctx := newTestAuthenticator(OptionalLogin, tokenProvider, tokenInCache)
		client, err := auth.Client()
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)

		// Initial call uses existing cached token.
		resp, err := client.Get(ts.URL + "/1")
		So(err, ShouldBeNil)
		ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		// It expires at ~60 minutes, refresh fails, authenticator switches to
		// anonymous access.
		clock.Get(ctx).(testclock.TestClock).Add(65 * time.Minute)
		resp, err = client.Get(ts.URL + "/2")
		So(err, ShouldBeNil)
		ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		// Bad token is removed from the cache.
		So(auth.currentToken(), ShouldBeNil)
		So(auth.cache.(*fakeTokenCache).cache, ShouldBeNil)

		// All calls are actually made.
		So(calls, ShouldEqual, 2)
	})
}

func newTestAuthenticator(loginMode LoginMode, p internal.TokenProvider, cached *oauth2.Token) (*Authenticator, context.Context) {
	// Use auto-advancing fake time.
	ctx := context.Background()
	ctx, tc := testclock.UseTime(ctx, now)
	tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
		tc.Add(d)
	})

	// Use temporary directory to store secrets.
	tempDir, err := ioutil.TempDir("", "auth_test")
	So(err, ShouldBeNil)
	Reset(func() {
		So(os.RemoveAll(tempDir), ShouldBeNil)
	})

	// Populate initial token cache.
	var initialCache []byte
	if cached != nil {
		var err error
		initialCache, err = internal.MarshalToken(cached)
		if err != nil {
			panic(err)
		}
	}

	return NewAuthenticator(ctx, loginMode, Options{
		TokenCacheFactory: func(string) (TokenCache, error) {
			return &fakeTokenCache{initialCache}, nil
		},
		SecretsDir:          tempDir,
		customTokenProvider: p,
	}), ctx
}

////////////////////////////////////////////////////////////////////////////////

type fakeTokenProvider struct {
	interactive            bool
	revokedCreds           bool
	revokedToken           bool
	transientRefreshErrors int
	tokenToMint            *oauth2.Token
	tokenToRefresh         *oauth2.Token
}

func (p *fakeTokenProvider) RequiresInteraction() bool {
	return p.interactive
}

func (p *fakeTokenProvider) CacheSeed() []byte {
	return nil
}

func (p *fakeTokenProvider) MintToken() (*oauth2.Token, error) {
	if p.revokedCreds {
		return nil, internal.ErrBadCredentials
	}
	if p.tokenToMint != nil {
		return p.tokenToMint, nil
	}
	return &oauth2.Token{AccessToken: "some minted token"}, nil
}

func (p *fakeTokenProvider) RefreshToken(*oauth2.Token) (*oauth2.Token, error) {
	if p.transientRefreshErrors != 0 {
		p.transientRefreshErrors--
		return nil, errors.WrapTransient(fmt.Errorf("transient error"))
	}
	if p.revokedCreds {
		return nil, internal.ErrBadCredentials
	}
	if p.revokedToken {
		return nil, internal.ErrBadRefreshToken
	}
	if p.tokenToRefresh != nil {
		return p.tokenToRefresh, nil
	}
	return &oauth2.Token{AccessToken: "some refreshed token"}, nil
}

////////////////////////////////////////////////////////////////////////////////

type fakeTokenCache struct {
	cache []byte
}

func (c *fakeTokenCache) Read() ([]byte, error) {
	return c.cache, nil
}

func (c *fakeTokenCache) Write(tok []byte, expiry time.Time) error {
	c.cache = append([]byte(nil), tok...)
	return nil
}

func (c *fakeTokenCache) Clear() error {
	c.cache = nil
	return nil
}
