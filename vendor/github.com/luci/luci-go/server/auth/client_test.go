// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/server/auth/delegation"
	"github.com/luci/luci-go/server/auth/identity"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetRPCTransport(t *testing.T) {
	Convey("GetRPCTransport works", t, func() {
		ctx := context.Background()
		mock := &clientRPCTransportMock{}
		ctx = ModifyConfig(ctx, func(cfg *Config) {
			cfg.AccessTokenProvider = mock.getAccessToken
			cfg.AnonymousTransport = mock.getTransport
		})

		Convey("in NoAuth mode", func(c C) {
			t, err := GetRPCTransport(ctx, NoAuth)
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com"))
			So(err, ShouldBeNil)

			So(len(mock.calls), ShouldEqual, 0)
			So(len(mock.reqs[0].Header), ShouldEqual, 0)
		})

		Convey("in AsSelf mode", func(c C) {
			t, err := GetRPCTransport(ctx, AsSelf, WithScopes("A", "B"))
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com"))
			So(err, ShouldBeNil)

			So(mock.calls[0], ShouldResemble, []string{"A", "B"})
			So(mock.reqs[0].Header, ShouldResemble, http.Header{
				"Authorization": {"Bearer blah"},
			})
		})

		Convey("in AsSelf mode with default scopes", func(c C) {
			t, err := GetRPCTransport(ctx, AsSelf)
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com"))
			So(err, ShouldBeNil)

			So(mock.calls[0], ShouldResemble, []string{"https://www.googleapis.com/auth/userinfo.email"})
			So(mock.reqs[0].Header, ShouldResemble, http.Header{
				"Authorization": {"Bearer blah"},
			})
		})

		Convey("in AsUser mode, authenticated", func(c C) {
			ctx := WithState(ctx, &state{
				user: &User{Identity: "user:abc@example.com"},
			})

			t, err := GetRPCTransport(ctx, AsUser, &rpcMocks{
				MintDelegationToken: func(ic context.Context, p DelegationTokenParams) (*delegation.Token, error) {
					c.So(p, ShouldResemble, DelegationTokenParams{
						TargetHost: "example.com",
						MinTTL:     10 * time.Minute,
					})
					return &delegation.Token{Token: "deleg_tok"}, nil
				},
			})
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com/some-path/sd"))
			So(err, ShouldBeNil)

			So(mock.calls[0], ShouldResemble, []string{"https://www.googleapis.com/auth/userinfo.email"})
			So(mock.reqs[0].Header, ShouldResemble, http.Header{
				"Authorization":         {"Bearer blah"},
				"X-Delegation-Token-V1": {"deleg_tok"},
			})
		})

		Convey("in AsUser mode, anonymous", func(c C) {
			ctx := WithState(ctx, &state{
				user: &User{Identity: identity.AnonymousIdentity},
			})

			t, err := GetRPCTransport(ctx, AsUser, &rpcMocks{
				MintDelegationToken: func(ic context.Context, p DelegationTokenParams) (*delegation.Token, error) {
					panic("must not be called")
				},
			})
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com"))
			So(err, ShouldBeNil)
			So(mock.reqs[0].Header, ShouldResemble, http.Header{})
		})

		Convey("in AsUser mode, with existing token", func(c C) {
			ctx := WithState(ctx, &state{
				user: &User{Identity: identity.AnonymousIdentity},
			})

			t, err := GetRPCTransport(ctx, AsUser, WithDelegationToken("deleg_tok"), &rpcMocks{
				MintDelegationToken: func(ic context.Context, p DelegationTokenParams) (*delegation.Token, error) {
					panic("must not be called")
				},
			})
			So(err, ShouldBeNil)
			_, err = t.RoundTrip(makeReq("https://example.com"))
			So(err, ShouldBeNil)

			So(mock.calls[0], ShouldResemble, []string{"https://www.googleapis.com/auth/userinfo.email"})
			So(mock.reqs[0].Header, ShouldResemble, http.Header{
				"Authorization":         {"Bearer blah"},
				"X-Delegation-Token-V1": {"deleg_tok"},
			})
		})

		Convey("in NoAuth mode with scopes, should error", func(c C) {
			_, err := GetRPCTransport(ctx, NoAuth, WithScopes("A"))
			So(err, ShouldNotBeNil)
		})
	})
}

func makeReq(url string) *http.Request {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

type clientRPCTransportMock struct {
	calls [][]string
	reqs  []*http.Request

	cb func(req *http.Request, body string) string
}

func (m *clientRPCTransportMock) getAccessToken(c context.Context, scopes []string) (auth.Token, error) {
	m.calls = append(m.calls, scopes)
	return auth.Token{AccessToken: "blah", TokenType: "Bearer"}, nil
}

func (m *clientRPCTransportMock) getTransport(c context.Context) http.RoundTripper {
	return m
}

func (m *clientRPCTransportMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.reqs = append(m.reqs, req)
	code := 500
	resp := "internal error"
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		if m.cb != nil {
			code = 200
			resp = m.cb(req, string(body))
		}
	}
	return &http.Response{
		StatusCode: code,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(resp))),
	}, nil
}
