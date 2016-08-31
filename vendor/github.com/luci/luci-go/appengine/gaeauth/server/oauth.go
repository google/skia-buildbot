// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/gae/service/urlfetch"
	"github.com/luci/gae/service/user"

	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
)

// EmailScope is a scope used to identifies user's email. Present in most tokens
// by default. Can be used as a base scope for authentication.
const EmailScope = "https://www.googleapis.com/auth/userinfo.email"

// OAuth2Method implements auth.Method on top of GAE OAuth2 API. It doesn't
// implement auth.UsersAPI.
type OAuth2Method struct {
	// Scopes is a list of OAuth scopes to check when authenticating the token.
	Scopes []string

	// tokenInfoEndpoint is used in unit test to mock production endpoint.
	tokenInfoEndpoint string
}

// Authenticate extracts peer's identity from the incoming request.
func (m *OAuth2Method) Authenticate(c context.Context, r *http.Request) (*auth.User, error) {
	header := r.Header.Get("Authorization")
	if header == "" || len(m.Scopes) == 0 {
		return nil, nil // this method is not applicable
	}
	if info.Get(c).IsDevAppServer() {
		return m.authenticateDevServer(c, header, m.Scopes)
	}
	return m.authenticateProd(c, m.Scopes)
}

// authenticateProd is called on real appengine.
func (m *OAuth2Method) authenticateProd(c context.Context, scopes []string) (*auth.User, error) {
	// GetOAuthUser RPC is notoriously flaky. Do a bunch of retries on errors.
	var err error
	for attemp := 0; attemp < 4; attemp++ {
		var u *user.User
		u, err = user.Get(c).CurrentOAuth(scopes...)
		if err != nil {
			logging.Warningf(c, "oauth: failed to execute GetOAuthUser - %s", err)
			continue
		}
		if u == nil {
			return nil, nil
		}
		if u.ClientID == "" {
			return nil, fmt.Errorf("oauth: ClientID is unexpectedly empty")
		}
		id, idErr := identity.MakeIdentity("user:" + u.Email)
		if idErr != nil {
			return nil, idErr
		}
		return &auth.User{
			Identity:  id,
			Superuser: u.Admin,
			Email:     u.Email,
			ClientID:  u.ClientID,
		}, nil
	}
	return nil, errors.WrapTransient(err)
}

type tokenCheckCache string

// authenticateDevServer is called on dev server. It is using OAuth2 tokeninfo
// endpoint via URL fetch. It is slow as hell, but good enough for local manual
// testing.
func (m *OAuth2Method) authenticateDevServer(c context.Context, header string, scopes []string) (*auth.User, error) {
	chunks := strings.SplitN(header, " ", 2)
	if len(chunks) != 2 || (chunks[0] != "OAuth" && chunks[0] != "Bearer") {
		return nil, errors.New("oauth: bad Authorization header")
	}
	accessToken := chunks[1]

	// Maybe already verified this token?
	if user, ok := proccache.Get(c, tokenCheckCache(accessToken)); ok {
		return user.(*auth.User), nil
	}

	// Fetch an info dict associated with the token.
	client := http.Client{
		Transport: urlfetch.Get(c),
	}
	logging.Infof(c, "oauth: Querying tokeninfo endpoint")
	endpoint := m.tokenInfoEndpoint
	if endpoint == "" {
		endpoint = "https://www.googleapis.com/oauth2/v3/tokeninfo"
	}
	resp, err := client.Get(endpoint + "?access_token=" + url.QueryEscape(accessToken))
	if err != nil {
		return nil, errors.WrapTransient(err)
	}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 500 {
		return nil, errors.WrapTransient(
			fmt.Errorf("oauth: transient error when validating token (HTTP %d)", resp.StatusCode))
	}

	// Deserialize, perform some basic checks.
	var tokenInfo struct {
		Audience      string `json:"aud"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Error         string `json:"error_description"`
		ExpiresIn     string `json:"expires_in"`
		Scope         string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, fmt.Errorf("oauth: failed to deserialize token info JSON - %s", err)
	}
	switch {
	case tokenInfo.Error != "":
		return nil, fmt.Errorf("oauth: bad token - %s", tokenInfo.Error)
	case tokenInfo.Email == "":
		return nil, fmt.Errorf("oauth: token is not associated with an email")
	case tokenInfo.EmailVerified != "true":
		return nil, fmt.Errorf("oauth: email %s is not verified", tokenInfo.Email)
	}
	expiresIn, err := strconv.Atoi(tokenInfo.ExpiresIn)
	if err != nil || expiresIn <= 0 {
		return nil, fmt.Errorf("oauth: 'expires_in' field is not a positive integer")
	}

	// Verify `scopes` is subset of tokenInfo.Scope.
	tokenScopes := map[string]bool{}
	for _, s := range strings.Split(tokenInfo.Scope, " ") {
		tokenScopes[s] = true
	}
	for _, s := range scopes {
		if !tokenScopes[s] {
			return nil, fmt.Errorf("oauth: token doesn't have scope %q", s)
		}
	}

	// Good enough.
	id, err := identity.MakeIdentity("user:" + tokenInfo.Email)
	if err != nil {
		return nil, err
	}
	u := &auth.User{
		Identity: id,
		Email:    tokenInfo.Email,
		ClientID: tokenInfo.Audience,
	}
	proccache.Put(c, tokenCheckCache(accessToken), u, time.Duration(expiresIn)*time.Second)
	return u, nil
}
