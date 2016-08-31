// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/rand/mathrand"
)

// delegationTokenCache is used to store delegation tokens in the global cache.
var delegationTokenCache = tokenCache{
	Kind:                "delegation",
	Version:             2,
	ExpRandPercent:      10,
	MinAcceptedLifetime: 5 * time.Minute,
}

// tokenCache knows how to keep tokens in the global cache.
//
// Uses Config.GlobalCache as a storage backend.
//
// It implements probabilistic early expiration to workaround cache stampede
// problem for hot items, see Fetch.
type tokenCache struct {
	// Kind defines the token kind. Will be used as part of the cache key.
	Kind string

	// Version defines format of the data. Will be used as part of the cache key.
	//
	// If you change a type behind interface{} in Token field, you MUST bump the
	// version. It will also "invalidate" all existing cached entries (they will
	// just become inaccessible and eventually will be evicted from the cache).
	Version int

	// ExpRandPercent defines when expiration randomizations kicks in.
	//
	// For example, value of '10' means when token is within 10% of its
	// expiration it may randomly (on each access) be considered already expired.
	ExpRandPercent int

	// MinAcceptedLifetime defines minimal lifetime of a token.
	//
	// If a token lives less than given time, Store(...) will refuse to save it.
	MinAcceptedLifetime time.Duration
}

// cachedToken is stored in a tokenCache.
type cachedToken struct {
	Key     string      // cache key, must be unique (no other restrictions)
	Token   interface{} // the actual token, must be Gob-serializable
	Created time.Time   // when it was created, required, UTC
	Expiry  time.Time   // its expiration time, required, UTC
}

// Store unconditionally puts the token in the cache.
func (tc *tokenCache) Store(c context.Context, tok cachedToken) error {
	switch {
	case tok.Created.IsZero() || tok.Created.Location() != time.UTC:
		panic(fmt.Errorf("tok.Created is not a valid UTC time - %v", tok.Created))
	case tok.Expiry.IsZero() || tok.Expiry.Location() != time.UTC:
		panic(fmt.Errorf("tok.Expiry is not a valid UTC time - %v", tok.Expiry))
	}
	ttl := tok.Expiry.Sub(tok.Created)
	if ttl < tc.MinAcceptedLifetime {
		return fmt.Errorf("refusing to store a token that expires in %s", ttl)
	}
	blob, err := tc.marshal(&tok)
	if err != nil {
		return err
	}
	cfg := GetConfig(c)
	if cfg == nil || cfg.GlobalCache == nil {
		return ErrNotConfigured
	}
	return cfg.GlobalCache.Set(c, tc.itemKey(tok.Key), blob, ttl)
}

// Fetch grabs cached token if it hasn't expired yet.
//
// Returns (nil, nil) if no such token or it has expired already. If the cached
// token is close to expiration, this function will randomly return a cache
// miss. That way if multiple concurrent processes all constantly use the same
// token, only the most unlucky one will refresh it.
func (tc *tokenCache) Fetch(c context.Context, key string) (*cachedToken, error) {
	cfg := GetConfig(c)
	if cfg == nil || cfg.GlobalCache == nil {
		return nil, ErrNotConfigured
	}

	blob, err := cfg.GlobalCache.Get(c, tc.itemKey(key))
	switch {
	case err != nil:
		return nil, err
	case blob == nil:
		return nil, nil
	}

	tok, err := tc.unmarshal(blob)
	if err != nil {
		return nil, err
	}
	if tok.Key != key {
		return nil, fmt.Errorf("SHA1 collision in the token cache: %q vs %q", key, tok.Key)
	}

	// Don't use expiration randomization if the item is far from expiration.
	now := clock.Now(c).UTC()
	exp := tok.Expiry.Sub(tok.Created) * time.Duration(tc.ExpRandPercent) / 100
	switch {
	case now.After(tok.Expiry):
		return nil, nil // already expired
	case now.Add(exp).Before(tok.Expiry):
		return tok, nil
	}

	// The expiration is close enough. Do the randomization.
	// TODO(vadimsh): The distribution was picked randomly. Maybe use exponential
	// one, as some literature suggests?
	rnd := time.Duration(mathrand.Get(c).Int63n(int64(exp)))
	if now.Add(rnd).After(tok.Expiry) {
		return nil, nil
	}
	return tok, nil
}

// itemKey derives the short key to use in the underlying cache.
func (tc *tokenCache) itemKey(key string) string {
	digest := sha1.Sum([]byte(key))
	asStr := base64.RawURLEncoding.EncodeToString(digest[:])
	return fmt.Sprintf("%s/%d/%s", tc.Kind, tc.Version, asStr)
}

// marshal converts cachedToken struct to byte blob.
func (tc *tokenCache) marshal(t *cachedToken) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// unmarshal is reverse of marshal.
func (tc *tokenCache) unmarshal(blob []byte) (*cachedToken, error) {
	dec := gob.NewDecoder(bytes.NewReader(blob))
	out := &cachedToken{}
	if err := dec.Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}
