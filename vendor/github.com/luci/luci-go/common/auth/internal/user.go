// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
)

type userAuthTokenProvider struct {
	ctx    context.Context
	config *oauth2.Config
}

// NewUserAuthTokenProvider returns TokenProvider that can perform 3-legged
// OAuth flow involving interaction with a user.
func NewUserAuthTokenProvider(ctx context.Context, clientID, clientSecret string, scopes []string) (TokenProvider, error) {
	return &userAuthTokenProvider{
		ctx: ctx,
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
			RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
			Scopes:      scopes,
		},
	}, nil
}

func (p *userAuthTokenProvider) RequiresInteraction() bool {
	return true
}

func (p *userAuthTokenProvider) CacheSeed() []byte {
	return nil
}

func (p *userAuthTokenProvider) MintToken() (*oauth2.Token, error) {
	// Grab the authorization code by redirecting a user to a consent screen.
	url := p.config.AuthCodeURL("", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Visit the URL to get authorization code.\n\n%s\n\n", url)
	fmt.Printf("Authorization code: ")
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, err
	}
	// Exchange it for a token.
	return p.config.Exchange(p.ctx, code)
}

func (p *userAuthTokenProvider) RefreshToken(tok *oauth2.Token) (*oauth2.Token, error) {
	// Clear expiration time to force token refresh. Do not use 0 since it means
	// that token never expires.
	t := *tok
	t.Expiry = time.Unix(1, 0)
	switch newTok, err := grabToken(p.config.TokenSource(p.ctx, &t)); {
	case err == nil:
		return newTok, nil
	case errors.IsTransient(err):
		logging.Warningf(p.ctx, "Transient error when refreshing the token - %s", err)
		return nil, err
	default:
		logging.Warningf(p.ctx, "Bad refresh token - %s", err)
		return nil, ErrBadRefreshToken
	}
}
