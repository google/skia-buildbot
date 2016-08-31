// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"cloud.google.com/go/compute/metadata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/logging"
)

type gceTokenProvider struct {
	account string
}

// NewGCETokenProvider returns TokenProvider that knows how to use GCE metadata server.
func NewGCETokenProvider(c context.Context, account string, scopes []string) (TokenProvider, error) {
	// Ensure account has requested scopes.
	availableScopes, err := metadata.Scopes(account)
	if err != nil {
		return nil, err
	}
	availableSet := stringset.NewFromSlice(availableScopes...)
	for _, requested := range scopes {
		if !availableSet.Has(requested) {
			logging.Warningf(c, "GCE service account %q doesn't have required scope %q", account, requested)
			return nil, ErrInsufficientAccess
		}
	}
	return &gceTokenProvider{account: account}, nil
}

func (p *gceTokenProvider) RequiresInteraction() bool {
	return false
}

func (p *gceTokenProvider) CacheSeed() []byte {
	return nil
}

func (p *gceTokenProvider) MintToken() (*oauth2.Token, error) {
	src := google.ComputeTokenSource(p.account)
	return src.Token()
}

func (p *gceTokenProvider) RefreshToken(*oauth2.Token) (*oauth2.Token, error) {
	// Minting and refreshing on GCE is the same thing: a call to metadata server.
	return p.MintToken()
}
