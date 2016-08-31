// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package internal contains code used internally by common/auth.
package internal

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
)

var (
	// ErrInsufficientAccess is returned by MintToken() if token can't be minted
	// for given OAuth scopes. For example, if GCE instance wasn't granted access
	// to requested scopes when it was created.
	ErrInsufficientAccess = errors.New("can't get access token for given scopes")

	// ErrBadRefreshToken is returned by RefreshToken if refresh token was revoked
	// or otherwise invalid. It means MintToken must be used to get a new refresh
	// token.
	ErrBadRefreshToken = errors.New("refresh_token is not valid")

	// ErrBadCredentials is returned by MintToken or RefreshToken if provided
	// offline credentials (like service account key) are invalid.
	ErrBadCredentials = errors.New("invalid service account credentials")
)

// TokenProvider knows how to mint new tokens, refresh existing ones, marshal
// and unmarshal tokens to byte buffers.
type TokenProvider interface {
	// RequiresInteraction is true if provider may start user interaction
	// in MintToken.
	RequiresInteraction() bool

	// CacheSeed is an optional byte string to use when constructing cache entry
	// name for access token cache. Different seeds will result in different
	// cache entries.
	CacheSeed() []byte

	// MintToken launches authentication flow (possibly interactive) and returns
	// a new refreshable token (or error). It must never return (nil, nil).
	MintToken() (*oauth2.Token, error)

	// RefreshToken takes existing token (probably expired, but not necessarily)
	// and returns a new refreshed token. It should never do any user interaction.
	// If a user interaction is required, a error should be returned instead.
	RefreshToken(*oauth2.Token) (*oauth2.Token, error)
}

// TransportFromContext returns http.RoundTripper buried inside the given
// context.
func TransportFromContext(ctx context.Context) http.RoundTripper {
	// When nil is passed to NewClient it skips all OAuth stuff and returns
	// client extracted from the context or http.DefaultClient.
	c := oauth2.NewClient(ctx, nil)
	if c == http.DefaultClient {
		return http.DefaultTransport
	}
	return c.Transport
}

// MarshalToken converts a token into byte buffer.
func MarshalToken(tok *oauth2.Token) ([]byte, error) {
	// TODO(vadimsh): Remove 'flavor' and 'version' when new code that doesn't use
	// them is deployed everywhere.
	flavor := "service_account"
	if tok.RefreshToken != "" {
		flavor = "user"
	}
	return json.MarshalIndent(&tokenOnDisk{
		Version:      "1",    // not actually used, exists for backward compatibility
		Flavor:       flavor, // same
		AccessToken:  tok.AccessToken,
		TokenType:    tok.Type(),
		RefreshToken: tok.RefreshToken,
		ExpiresAtSec: tok.Expiry.Unix(),
	}, "", "\t")
}

// UnmarshalToken takes byte buffer produced by MarshalToken and returns
// original token.
func UnmarshalToken(data []byte) (*oauth2.Token, error) {
	onDisk := tokenOnDisk{}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken:  onDisk.AccessToken,
		TokenType:    onDisk.TokenType,
		RefreshToken: onDisk.RefreshToken,
		Expiry:       time.Unix(onDisk.ExpiresAtSec, 0),
	}, nil
}

// TokenExpiresIn returns True if the token is not valid or expires within given
// duration.
func TokenExpiresIn(ctx context.Context, t *oauth2.Token, lifetime time.Duration) bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.Expiry.IsZero() {
		return false
	}
	expiry := t.Expiry.Add(-lifetime)
	return expiry.Before(clock.Now(ctx))
}

// EqualTokens returns true if both token object have same access token.
//
// 'nil' token corresponds to an empty access token.
func EqualTokens(a, b *oauth2.Token) bool {
	if a == b {
		return true
	}
	aTok := ""
	if a != nil {
		aTok = a.AccessToken
	}
	bTok := ""
	if b != nil {
		bTok = b.AccessToken
	}
	return aTok == bTok
}

// tokenOnDisk describes JSON produced by MarshalToken.
type tokenOnDisk struct {
	Version      string `json:"version,omitempty"` // not actually used, exists for backward compatibility
	Flavor       string `json:"flavor,omitempty"`  // same
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresAtSec int64  `json:"expires_at,omitempty"`
}

// isBadTokenError sniffs out HTTP 400 from token source errors.
func isBadTokenError(err error) bool {
	// See https://github.com/golang/oauth2/blob/master/internal/token.go.
	// Unfortunately, fmt.Errorf is used there, so there's no other way to
	// differentiate between bad tokens and transient errors.
	return err != nil && strings.Contains(err.Error(), "400 Bad Request")
}

// grabToken uses token source to create a new token.
//
// It recognizes transient errors.
func grabToken(src oauth2.TokenSource) (*oauth2.Token, error) {
	switch tok, err := src.Token(); {
	case isBadTokenError(err):
		return nil, err
	case err != nil:
		// More often than not errors here are transient (network connectivity
		// errors, HTTP 500 responses, etc). It is difficult to categorize them,
		// since oauth2 library uses fmt.Errorf(...) for errors. Retrying a fatal
		// error a bunch of times is not very bad, so pick safer approach and assume
		// any error is transient. Revoked refresh token or bad credentials (most
		// common source of fatal errors) is already handled above.
		return nil, errors.WrapTransient(err)
	default:
		return tok, nil
	}
}
