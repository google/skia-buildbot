// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package openid

import (
	"net/http"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/tokens"
	"golang.org/x/net/context"
)

// sessionCookieName is actual cookie name to set.
const sessionCookieName = "oid_session"

// sessionCookieToken is used to generate signed cookies that hold session ID.
var sessionCookieToken = tokens.TokenKind{
	Algo:       tokens.TokenAlgoHmacSHA256,
	Expiration: 30 * 24 * time.Hour, // one month
	SecretKey:  "openid_session_cookie",
	Version:    1,
}

// makeSessionCookie takes a session ID and makes a signed cookie that can be
// put in a response.
func makeSessionCookie(c context.Context, sid string, secure bool) (*http.Cookie, error) {
	tok, err := sessionCookieToken.Generate(c, nil, map[string]string{"sid": sid}, 0)
	if err != nil {
		return nil, err
	}
	// Make cookie expire a bit earlier, to avoid weird "bad token" errors if
	// clocks are out of sync.
	exp := sessionCookieToken.Expiration - 15*time.Minute
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    tok,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true, // no access from Javascript
		Expires:  clock.Now(c).Add(exp),
		MaxAge:   int(exp / time.Second),
	}, nil
}

// decodeSessionCookie takes an incoming request and returns a session ID stored
// in a session cookie, or "" if not there, invalid or expired. Returns errors
// on transient errors only.
func decodeSessionCookie(c context.Context, r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", nil // no such cookie
	}
	payload, err := sessionCookieToken.Validate(c, cookie.Value, nil)
	switch {
	case errors.IsTransient(err):
		return "", err
	case err != nil:
		logging.Warningf(c, "Failed to decode session cookie %q: %s", cookie.Value, err)
		return "", nil
	case payload["sid"] == "":
		logging.Warningf(c, "No 'sid' key in cookie payload %v", payload)
		return "", nil
	}
	return payload["sid"], nil
}
