// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	cauth "github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth"
)

// MockAuthConfig configures auth library for unit tests environment.
//
// If modifies the configure stored in the context. See auth.SetConfig for more
// info.
func MockAuthConfig(c context.Context) context.Context {
	return auth.ModifyConfig(c, func(cfg *auth.Config) {
		cfg.AnonymousTransport = func(context.Context) http.RoundTripper {
			return http.DefaultTransport
		}
		cfg.AccessTokenProvider = func(ic context.Context, scopes []string) (cauth.Token, error) {
			return cauth.Token{
				AccessToken: "fake_token",
				TokenType:   "Bearer",
				Expiry:      clock.Now(ic).Add(time.Hour).UTC(),
			}, nil
		}
	})
}
