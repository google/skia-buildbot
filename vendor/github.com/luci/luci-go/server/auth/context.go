// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/router"
)

type authenticatorKey int

// SetAuthenticator injects copy of Authenticator (list of auth methods) into
// the context to use by default in LoginURL, LogoutURL and Authenticate.
// Usually installed into the context by some base middleware.
func SetAuthenticator(c context.Context, a Authenticator) context.Context {
	return context.WithValue(c, authenticatorKey(0), append(Authenticator(nil), a...))
}

// Use is a middleware that simply puts given Authenticator into the context.
func Use(a Authenticator) router.Middleware {
	return func(c *router.Context, next router.Handler) {
		c.Context = SetAuthenticator(c.Context, a)
		next(c)
	}
}

// getAuthenticator extracts instance of Authenticator (list of auth methods)
// from the context. Returns nil if no authenticator is set.
func getAuthenticator(c context.Context) Authenticator {
	if a, ok := c.Value(authenticatorKey(0)).(Authenticator); ok {
		return a
	}
	return nil
}

// LoginURL returns a URL that, when visited, prompts the user to sign in,
// then redirects the user to the URL specified by dest. It is wrapper around
// LoginURL method of Authenticator in the context.
func LoginURL(c context.Context, dest string) (string, error) {
	return getAuthenticator(c).LoginURL(c, dest)
}

// LogoutURL returns a URL that, when visited, signs the user out,
// then redirects the user to the URL specified by dest. It is wrapper around
// LogoutURL method of Authenticator in the context.
func LogoutURL(c context.Context, dest string) (string, error) {
	return getAuthenticator(c).LogoutURL(c, dest)
}

// Authenticate is a middleware that performs authentication (using Authenticator
// in the context) and calls next handler.
func Authenticate(c *router.Context, next router.Handler) {
	a := getAuthenticator(c.Context)
	if a == nil {
		replyError(c.Context, c.Writer, 500, "Authentication middleware is not configured")
		return
	}
	ctx, err := a.Authenticate(c.Context, c.Request)
	switch {
	case errors.IsTransient(err):
		replyError(c.Context, c.Writer, 500, fmt.Sprintf("Transient error during authentication - %s", err))
	case err != nil:
		replyError(c.Context, c.Writer, 401, fmt.Sprintf("Authentication error - %s", err))
	default:
		c.Context = ctx
		next(c)
	}
}

// Autologin is a middleware that redirects the user to login page if the user
// is not signed in yet or authentication methods do not recognize user
// credentials. Uses Authenticator instance in the context.
func Autologin(c *router.Context, next router.Handler) {
	a := getAuthenticator(c.Context)
	if a == nil {
		replyError(c.Context, c.Writer, 500, "Authentication middleware is not configured")
		return
	}
	ctx, err := a.Authenticate(c.Context, c.Request)

	switch {
	case errors.IsTransient(err):
		replyError(c.Context, c.Writer, 500, fmt.Sprintf("Transient error during authentication - %s", err))

	case err != nil:
		replyError(c.Context, c.Writer, 401, fmt.Sprintf("Authentication error - %s", err))

	case CurrentIdentity(ctx).Kind() == identity.Anonymous:
		dest := c.Request.RequestURI
		if dest == "" {
			// Make r.URL relative.
			destURL := *c.Request.URL
			destURL.Host = ""
			destURL.Scheme = ""
			dest = destURL.String()
		}
		url, err := a.LoginURL(c.Context, dest)
		if err != nil {
			if errors.IsTransient(err) {
				replyError(c.Context, c.Writer, 500, fmt.Sprintf("Transient error during authentication - %s", err))
			} else {
				replyError(c.Context, c.Writer, 401, fmt.Sprintf("Authentication error - %s", err))
			}
			return
		}
		http.Redirect(c.Writer, c.Request, url, 302)

	default:
		c.Context = ctx
		next(c)
	}
}

// replyError logs the error and writes it to ResponseWriter.
func replyError(c context.Context, rw http.ResponseWriter, code int, msg string) {
	logging.Errorf(c, "HTTP %d: %s", code, msg)
	http.Error(rw, msg, code)
}
