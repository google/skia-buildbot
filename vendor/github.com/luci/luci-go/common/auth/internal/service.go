// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"crypto/sha1"
	"io/ioutil"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
)

type serviceAccountTokenProvider struct {
	ctx     context.Context
	jsonKey []byte
	path    string
	scopes  []string
}

// NewServiceAccountTokenProvider returns TokenProvider that uses service
// account private key (on disk or in memory) to make access tokens.
func NewServiceAccountTokenProvider(ctx context.Context, jsonKey []byte, path string, scopes []string) (TokenProvider, error) {
	return &serviceAccountTokenProvider{
		ctx:     ctx,
		jsonKey: jsonKey,
		path:    path,
		scopes:  scopes,
	}, nil
}

func (p *serviceAccountTokenProvider) jwtConfig() (*jwt.Config, error) {
	jsonKey := p.jsonKey
	if p.path != "" {
		var err error
		logging.Debugf(p.ctx, "Reading private key from %s", p.path)
		jsonKey, err = ioutil.ReadFile(p.path)
		if err != nil {
			return nil, err
		}
	}
	return google.JWTConfigFromJSON(jsonKey, p.scopes...)
}

func (p *serviceAccountTokenProvider) RequiresInteraction() bool {
	return false
}

func (p *serviceAccountTokenProvider) CacheSeed() []byte {
	// If using file on disk, associate cache entry with this file (since the key
	// inside may change in runtime). Otherwise associate cache entry with exact
	// private key passed (since it won't change over time).
	seed := sha1.New()
	if p.path != "" {
		seed.Write([]byte(p.path))
	} else {
		seed.Write(p.jsonKey)
	}
	return seed.Sum(nil)[:]
}

func (p *serviceAccountTokenProvider) MintToken() (*oauth2.Token, error) {
	cfg, err := p.jwtConfig()
	if err != nil {
		logging.Warningf(p.ctx, "Failed to load private key JSON - %s", err)
		return nil, ErrBadCredentials
	}
	switch newTok, err := grabToken(cfg.TokenSource(p.ctx)); {
	case err == nil:
		return newTok, nil
	case errors.IsTransient(err):
		logging.Warningf(p.ctx, "Transient error when creating access token - %s", err)
		return nil, err
	default:
		logging.Warningf(p.ctx, "Invalid or revoked service account key - %s", err)
		return nil, ErrBadCredentials
	}
}

func (p *serviceAccountTokenProvider) RefreshToken(*oauth2.Token) (*oauth2.Token, error) {
	// JWT tokens are self sufficient, there's no need for refresh_token. Minting
	// a token and "refreshing" it is a same thing.
	return p.MintToken()
}
