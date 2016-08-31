// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package openid

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/internal"
	"github.com/luci/luci-go/server/tokens"
)

// openIDStateToken is used to generate `state` parameter used in OpenID flow to
// pass state between our app and authentication backend.
var openIDStateToken = tokens.TokenKind{
	Algo:       tokens.TokenAlgoHmacSHA256,
	Expiration: 30 * time.Minute,
	SecretKey:  "openid_state_token",
	Version:    1,
}

// discoveryDoc describes subset of OpenID Discovery JSON document.
// See https://developers.google.com/identity/protocols/OpenIDConnect#discovery.
type discoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type discoveryDocCacheKey string

// fetchDiscoveryDoc fetches discovery document from given URL. It is cached in
// local memory for 24 hours.
func fetchDiscoveryDoc(c context.Context, url string) (*discoveryDoc, error) {
	if url == "" {
		return nil, ErrNotConfigured
	}

	fetcher := func() (interface{}, time.Duration, error) {
		doc := &discoveryDoc{}
		req := internal.Request{
			Method: "GET",
			URL:    url,
			Out:    doc,
		}
		if err := req.Do(c); err != nil {
			return nil, 0, err
		}
		return doc, time.Hour * 24, nil
	}

	// Cache the document in the process cache.
	cached, err := proccache.GetOrMake(c, discoveryDocCacheKey(url), fetcher)
	if err != nil {
		return nil, err
	}
	return cached.(*discoveryDoc), nil
}

// authenticationURI returns an URI to redirect a user to in order to
// authenticate via OpenID.
//
// This is step 1 of the authentication flow. Generate authentication URL and
// redirect user's browser to it. After consent screen, redirect_uri will be
// called (via user's browser) with `state` and authorization code passed to it,
// eventually resulting in a call to 'handle_authorization_code'.
func authenticationURI(c context.Context, cfg *Settings, state map[string]string) (string, error) {
	if cfg.ClientID == "" || cfg.RedirectURI == "" {
		return "", ErrNotConfigured
	}

	// Grab authorization URL from discovery doc.
	discovery, err := fetchDiscoveryDoc(c, cfg.DiscoveryURL)
	if err != nil {
		return "", err
	}
	if discovery.AuthorizationEndpoint == "" {
		return "", errors.New("openid: bad discovery doc, empty authorization_endpoint")
	}

	// Wrap state into HMAC-protected token.
	stateTok, err := openIDStateToken.Generate(c, nil, state, 0)
	if err != nil {
		return "", err
	}

	// Generate final URL.
	v := url.Values{}
	v.Set("client_id", cfg.ClientID)
	v.Set("redirect_uri", cfg.RedirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "openid email profile")
	v.Set("prompt", "select_account")
	v.Set("state", stateTok)
	return discovery.AuthorizationEndpoint + "?" + v.Encode(), nil
}

// validateStateToken validates 'state' token passed to redirect_uri. Returns
// whatever `state` was passed to authenticationURI.
func validateStateToken(c context.Context, stateTok string) (map[string]string, error) {
	return openIDStateToken.Validate(c, stateTok, nil)
}

// handleAuthorizationCode exchange `code` for user ID token and user profile.
func handleAuthorizationCode(c context.Context, cfg *Settings, code string) (uid string, u *auth.User, err error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" {
		return "", nil, ErrNotConfigured
	}

	// Grab endpoint for auth code exchange from discovery doc.
	discovery, err := fetchDiscoveryDoc(c, cfg.DiscoveryURL)
	if err != nil {
		return "", nil, err
	}
	if discovery.TokenEndpoint == "" {
		return "", nil, errors.New("openid: bad discovery doc, empty token_endpoint")
	}
	if discovery.UserinfoEndpoint == "" {
		return "", nil, errors.New("openid: bad discovery doc, empty userinfo_endpoint")
	}

	v := url.Values{}
	v.Set("code", code)
	v.Set("client_id", cfg.ClientID)
	v.Set("client_secret", cfg.ClientSecret)
	v.Set("redirect_uri", cfg.RedirectURI)
	v.Set("grant_type", "authorization_code")
	payload := v.Encode()

	// Send POST to token endpoint with URL-encoded parameters to get back
	// access token dict.
	var token struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}
	req := internal.Request{
		Method: "POST",
		URL:    discovery.TokenEndpoint,
		Body:   []byte(payload),
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Out: &token,
	}
	if err := req.Do(c); err != nil {
		return "", nil, err
	}

	// Use access token to fetch user profile.
	var profile struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	req = internal.Request{
		Method: "GET",
		URL:    discovery.UserinfoEndpoint,
		Headers: map[string]string{
			"Authorization": token.TokenType + " " + token.AccessToken,
		},
		Out: &profile,
	}
	if err := req.Do(c); err != nil {
		return "", nil, err
	}

	// Ignore non https:// URLs for pictures. We serve all pages over HTTPS and
	// don't want to break this rule just for a pretty picture.
	picture := profile.Picture
	if picture != "" && !strings.HasPrefix(picture, "https://") {
		picture = ""
	}

	// Google avatars sometimes look weird if used directly. Resized version
	// always looks fine. 's64' is documented, for example, here:
	// https://cloud.google.com/appengine/docs/python/images
	if picture != "" && strings.HasSuffix(picture, "/photo.jpg") {
		picture = picture[:len(picture)-len("/photo.jpg")] + "/s64/photo.jpg"
	}

	// Validate email is not malicious.
	id, err := identity.MakeIdentity("user:" + profile.Email)
	if err != nil {
		return "", nil, err
	}

	return profile.Sub, &auth.User{
		Identity: id,
		Email:    profile.Email,
		Name:     profile.Name,
		Picture:  picture,
	}, nil
}
