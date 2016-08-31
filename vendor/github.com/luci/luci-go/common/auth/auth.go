// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package auth implements an opinionated wrapper around OAuth2.
//
// It hides configurability of base oauth2 library and instead makes a
// predefined set of choices regarding where the credentials should be stored,
// how they should be cached and how OAuth2 flow should be invoked.
//
// It makes authentication flows look more uniform across tools that use this
// package and allow credentials reuse across multiple binaries.
//
// Also it knows about various environments luci tools are running under
// (GCE, regular datacenter, GAE, developers' machines) and switches default
// authentication scheme accordingly (e.g. on GCE machine the default is to use
// GCE metadata server).
//
// All tools that use auth library share same credentials by default, meaning
// a user needs to authenticate only once to use them all. Credentials are
// cached in ~/.config/chrome_infra/auth/*.tok and may be reused by all
// processes running under the same user account.
package auth

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"cloud.google.com/go/compute/metadata"

	"github.com/mitchellh/go-homedir"

	"github.com/luci/luci-go/common/auth/internal"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
)

var (
	// ErrLoginRequired is returned by Transport() in case long term credentials
	// are not cached and the user must go through interactive login.
	ErrLoginRequired = errors.New("interactive login is required")

	// ErrInsufficientAccess is returned by Login() or Transport() if access_token
	// can't be minted for given OAuth scopes. For example if GCE instance wasn't
	// granted access to requested scopes when it was created.
	ErrInsufficientAccess = internal.ErrInsufficientAccess

	// ErrBadCredentials is returned by authenticating RoundTripper if service
	// account key used to generate access tokens is revoked, malformed or can not
	// be read from disk.
	ErrBadCredentials = internal.ErrBadCredentials
)

// Known Google API OAuth scopes.
const (
	OAuthScopeEmail = "https://www.googleapis.com/auth/userinfo.email"
)

// Method defines a method to use to obtain OAuth access_token.
type Method string

// Supported authentication methods.
const (
	// AutoSelectMethod can be used to allow the library to pick a method most
	// appropriate for current execution environment. It will search for a private
	// key for a service account, then (if running on GCE) will try to query GCE
	// metadata server, and only then pick UserCredentialsMethod that requires
	// interaction with a user.
	AutoSelectMethod Method = ""

	// UserCredentialsMethod is used for interactive OAuth 3-legged login flow.
	UserCredentialsMethod Method = "UserCredentialsMethod"

	// ServiceAccountMethod is used to authenticate as a service account using
	// a private key.
	ServiceAccountMethod Method = "ServiceAccountMethod"

	// GCEMetadataMethod is used on Compute Engine to use tokens provided by
	// Metadata server. See https://cloud.google.com/compute/docs/authentication
	GCEMetadataMethod Method = "GCEMetadataMethod"
)

// LoginMode is used as enum in NewAuthenticator function.
type LoginMode string

const (
	// InteractiveLogin is passed to NewAuthenticator to instruct Authenticator to
	// ignore cached tokens and forcefully rerun full login flow in Transport().
	// Used by 'login' CLI command.
	InteractiveLogin LoginMode = "InteractiveLogin"

	// SilentLogin is passed to NewAuthenticator if authentication must be used
	// and it is NOT OK to run interactive login flow to get the tokens.
	// Transport() call will fail with ErrLoginRequired error if there's no cached
	// tokens. Should normally be used by all CLI tools that need to use
	// authentication. If in doubt what mode to use, pick this one.
	SilentLogin LoginMode = "SilentLogin"

	// OptionalLogin is passed to NewAuthenticator if it is OK not to use
	// authentication if there are no cached credentials or they are revoked or
	// expired. Interactive login will never be called, default unauthenticated
	// client will be used instead if no credentials are present. Should be used
	// by CLI tools where authentication is optional.
	OptionalLogin LoginMode = "OptionalLogin"
)

// minAcceptedLifetime is minimal lifetime of a token returned by the token
// source or put into authentication headers.
//
// If token is expected to live less than this duration, it will be refreshed.
const minAcceptedLifetime = 2 * time.Minute

// Options are used by NewAuthenticator call.
//
// All fields are optional and have sane default values.
type Options struct {
	// Transport is underlying round tripper to use for requests.
	//
	// If nil, will be extracted from the context. If not there,
	// http.DefaultTransport will be used.
	Transport http.RoundTripper

	// Method defaults to AutoSelectMethod.
	Method Method

	// Scopes is a list of OAuth scopes to request, defaults to [OAuthScopeEmail].
	Scopes []string

	// ClientID is OAuth client_id to use with UserCredentialsMethod.
	//
	// Default: provided by DefaultClient().
	ClientID string

	// ClientID is OAuth client_secret to use with UserCredentialsMethod.
	//
	// Default: provided by DefaultClient().
	ClientSecret string

	// ServiceAccountJSONPath is a path to a JSON blob with a private key to use.
	//
	// Used only with ServiceAccountMethod.
	// Default: ~/.config/chrome_infra/auth/service_account.json.
	ServiceAccountJSONPath string

	// ServiceAccountJSON is a body of JSON key file to use.
	//
	// Overrides ServiceAccountJSONPath if given.
	ServiceAccountJSON []byte

	// GCEAccountName is an account name to query to fetch token for from metadata
	// server when GCEMetadataMethod is used.
	//
	// If given account wasn't granted required set of scopes during instance
	// creation time, Transport() call fails with ErrInsufficientAccess.
	// Default: "default" account.
	GCEAccountName string

	// TokenCacheFactory is a factory method to use to grab TokenCache object.
	//
	// If not set, a file system cache will be used.
	TokenCacheFactory func(entryName string) (TokenCache, error)

	// SecretsDir can be used to override a path to a directory where tokens
	// are cached and default service account key is located.
	//
	// If not set, SecretsDir() will be used.
	SecretsDir string

	// customTokenProvider is used in unit tests.
	customTokenProvider internal.TokenProvider
}

// Authenticator is a factory for http.RoundTripper objects that know how to use
// cached OAuth credentials.
//
// Authenticator also knows how to run interactive login flow, if required.
type Authenticator struct {
	// Immutable members.
	loginMode LoginMode
	opts      *Options
	transport http.RoundTripper
	ctx       context.Context

	// Mutable members.
	lock     sync.RWMutex
	cache    TokenCache
	provider internal.TokenProvider
	err      error
	token    *oauth2.Token
}

// TokenCache implements a scheme to cache access tokens between the calls. It
// will be used concurrently from multiple threads. Authenticator takes care
// of token serialization, TokenCache operates in terms of byte buffers.
type TokenCache interface {
	// Read returns the data previously stored with Write or (nil, nil) if not
	// there or expired. It is also OK if token cache doesn't do expiration check
	// itself. Cached byte buffer includes expiration time too.
	Read() ([]byte, error)

	// Write puts token data into the cache.
	Write(tok []byte, expiry time.Time) error

	// Clear clears the cache.
	Clear() error
}

// Token represents OAuth2 access token.
type Token struct {
	// AccessToken is actual token that authorizes and authenticates the requests.
	AccessToken string `json:"access_token"`

	// Expiry is the expiration time of the token or zero if it does not expire.
	Expiry time.Time `json:"expiry"`

	// TokenType is the type of token (e.g. "Bearer", which is default).
	TokenType string `json:"token_type,omitempty"`
}

// TokenMinter is a factory for access tokens when using CustomMode. It is
// invoked only if token is not in cache or has expired.
type TokenMinter interface {
	// MintToken return new access token for given scopes.
	MintToken(scopes []string) (Token, error)

	// CacheSeed is used to derive cache key name for tokens minted via
	// this object.
	CacheSeed() []byte
}

// NewAuthenticator returns a new instance of Authenticator given its options.
//
// The authenticator is essentially a factory for http.RoundTripper that knows
// how to use OAuth2 tokens. It is bound to the given context: uses its logger,
// clock, transport and deadline.
func NewAuthenticator(ctx context.Context, loginMode LoginMode, opts Options) *Authenticator {
	ctx = logging.SetField(ctx, "pkg", "auth")

	// Add default scope, sort scopes.
	if len(opts.Scopes) == 0 {
		opts.Scopes = []string{OAuthScopeEmail}
	} else {
		opts.Scopes = append([]string(nil), opts.Scopes...) // copy
		sort.Strings(opts.Scopes)
	}

	// Fill in blanks with default values.
	if opts.ClientID == "" || opts.ClientSecret == "" {
		opts.ClientID, opts.ClientSecret = DefaultClient()
	}
	if opts.GCEAccountName == "" {
		opts.GCEAccountName = "default"
	}
	if opts.Transport == nil {
		// Note: TransportFromContext returns http.DefaultTransport if the context
		// doesn't have a transport set.
		opts.Transport = internal.TransportFromContext(ctx)
	}

	// See ensureInitialized for the rest of the initialization.
	auth := &Authenticator{
		loginMode: loginMode,
		opts:      &opts,
		ctx:       ctx,
	}
	auth.transport = NewModifyingTransport(opts.Transport, auth.authTokenInjector)
	return auth
}

// Transport optionally performs a login and returns http.RoundTripper. It is
// high level wrapper around Login() and TransportIfAvailable() calls. See
// documentation for LoginMode for more details.
func (a *Authenticator) Transport() (http.RoundTripper, error) {
	if a.loginMode == InteractiveLogin {
		if err := a.PurgeCredentialsCache(); err != nil {
			return nil, err
		}
	}
	transport, err := a.TransportIfAvailable()
	switch {
	case err == nil:
		return transport, nil
	case err == ErrInsufficientAccess && a.loginMode == OptionalLogin:
		return a.opts.Transport, nil
	case err != ErrLoginRequired || a.loginMode == SilentLogin:
		return nil, err
	case a.loginMode == OptionalLogin:
		return a.opts.Transport, nil
	case a.loginMode != InteractiveLogin:
		return nil, fmt.Errorf("invalid mode argument: %s", a.loginMode)
	}
	if err = a.Login(); err != nil {
		return nil, err
	}
	return a.TransportIfAvailable()
}

// Client optionally performs a login and returns http.Client. It uses transport
// returned by Transport(). See documentation for LoginMode for more details.
func (a *Authenticator) Client() (*http.Client, error) {
	transport, err := a.Transport()
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}

// TransportIfAvailable returns http.RoundTripper that adds authentication
// details to each request. An interactive authentication flow (if required)
// must be complete before making a transport, otherwise ErrLoginRequired is
// returned.
func (a *Authenticator) TransportIfAvailable() (http.RoundTripper, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	err := a.ensureInitialized()
	if err != nil {
		return nil, err
	}

	// No cached token and token provider requires interaction with a user: need
	// to login. Only non-interactive token providers are allowed to mint tokens
	// on the fly, see refreshToken.
	if a.token == nil && a.provider.RequiresInteraction() {
		return nil, ErrLoginRequired
	}

	return a.transport, nil
}

// Login perform an interaction with the user to get a long term refresh token
// and cache it. Blocks for user input, can use stdin. It overwrites currently
// cached credentials, if any.
func (a *Authenticator) Login() error {
	a.lock.Lock()
	defer a.lock.Unlock()

	err := a.ensureInitialized()
	if err != nil {
		return err
	}
	if !a.provider.RequiresInteraction() {
		return nil
	}

	// Create initial token. This may require interaction with a user. Do not do
	// retries here, since Login is called when user is looking, let user do the
	// retries (since if MintToken() interacts with the user, retrying it
	// automatically will be extra confusing).
	a.token, err = a.provider.MintToken()
	if err != nil {
		return err
	}

	// Store the initial token in the cache. Don't abort if it fails, the token
	// is still usable from the memory.
	if err = a.cacheToken(a.token); err != nil {
		logging.Warningf(a.ctx, "Failed to write token to cache: %s", err)
	}

	return nil
}

// PurgeCredentialsCache removes cached tokens.
func (a *Authenticator) PurgeCredentialsCache() error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if err := a.ensureInitialized(); err != nil {
		return err
	}
	a.token = nil
	return a.cache.Clear()
}

// GetAccessToken returns a valid access token with specified minimum lifetime.
//
// Does not interact with the user. May return ErrLoginRequired.
func (a *Authenticator) GetAccessToken(lifetime time.Duration) (Token, error) {
	a.lock.Lock()
	if err := a.ensureInitialized(); err != nil {
		a.lock.Unlock()
		return Token{}, err
	}
	tok := a.token
	a.lock.Unlock()

	if tok == nil || internal.TokenExpiresIn(a.ctx, tok, lifetime) {
		var err error
		tok, err = a.refreshToken(tok, lifetime)
		if err != nil {
			return Token{}, err
		}
		if internal.TokenExpiresIn(a.ctx, tok, lifetime) {
			return Token{}, fmt.Errorf("auth: failed to refresh the token")
		}
	}
	return Token{
		AccessToken: tok.AccessToken,
		Expiry:      tok.Expiry,
		TokenType:   tok.Type(),
	}, nil
}

// TokenSource returns oauth2.TokenSource implementation for interoperability
// with libraries that use it.
func (a *Authenticator) TokenSource() oauth2.TokenSource {
	return tokenSource{a}
}

////////////////////////////////////////////////////////////////////////////////
// oauth2.TokenSource implementation.

type tokenSource struct {
	a *Authenticator
}

// Token is part of oauth2.TokenSource inteface.
func (s tokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.a.GetAccessToken(minAcceptedLifetime)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: tok.AccessToken,
		Expiry:      tok.Expiry,
		TokenType:   tok.TokenType,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Authenticator private methods.

// ensureInitialized instantiates TokenProvider and reads token from cache.
//
// It is supposed to be called under the lock.
func (a *Authenticator) ensureInitialized() error {
	if a.err != nil || a.provider != nil {
		return a.err
	}

	// selectDefaultMethod may do heavy calls, call it lazily here rather than in
	// NewAuthenticator.
	if a.opts.Method == AutoSelectMethod {
		a.opts.Method = selectDefaultMethod(a.opts)
	}
	a.provider, a.err = makeTokenProvider(a.ctx, a.opts)
	if a.err != nil {
		return a.err
	}

	// Setup the cache only when Method is known, cache filename depends on it.
	cacheName := cacheEntryName(a.opts, a.provider)
	if a.opts.TokenCacheFactory != nil {
		a.cache, a.err = a.opts.TokenCacheFactory(cacheName)
		if a.err != nil {
			return a.err
		}
	} else {
		a.cache = &tokenFileCache{
			path: filepath.Join(a.opts.secretsDir(), cacheName+".tok"),
			ctx:  a.ctx,
		}
	}

	// Interactive providers need to know whether there's a cached token (to ask
	// to run interactive login if there's none). Non-interactive providers do not
	// care about state of the cache that much (they know how to update it
	// themselves). So examine the cache here only when using interactive
	// provider. Non interactive providers will do it lazily on a first
	// refreshToken(...) call. It saves one memcache call in certain situations
	// when auth library is used from GAE.
	if a.provider.RequiresInteraction() {
		// Broken token cache is not a fatal error. So just log it and forget, a new
		// token will be minted.
		var err error
		a.token, err = a.readTokenCache()
		if err != nil {
			logging.Warningf(a.ctx, "Failed to read token from cache: %s", err)
		}
	}

	return nil
}

// readTokenCache reads token from cache (if it is there) and unmarshals it.
//
// It may be called with a.lock held or not held. It works either way. Returns
// (nil, nil) if cache is empty.
func (a *Authenticator) readTokenCache() (*oauth2.Token, error) {
	// 'Read' returns (nil, nil) if cache is empty.
	buf, err := a.cache.Read()
	if err != nil || buf == nil {
		return nil, err
	}
	token, err := internal.UnmarshalToken(buf)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// cacheToken marshals the token and puts it in the cache.
//
// It may be called with a.lock held or not held. It works either way.
func (a *Authenticator) cacheToken(tok *oauth2.Token) error {
	buf, err := internal.MarshalToken(tok)
	if err != nil {
		return err
	}
	return a.cache.Write(buf, tok.Expiry)
}

// currentToken returns currently loaded token (or nil).
//
// It lock a.lock inside. It MUST NOT be called when a.lock is held.
func (a *Authenticator) currentToken() *oauth2.Token {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.token
}

// refreshToken compares current token to 'prev' and launches token refresh
// procedure if they still match.
//
// Returns a refreshed token (if a refresh procedure happened) or the current
// token, if it's already different from 'prev'. Acts as "Compare-And-Swap"
// where "Swap" is a token refresh procedure.
//
// If token can't be refreshed (e.g. it was revoked), sets current token to nil
// and returns ErrLoginRequired or ErrBadCredentials.
func (a *Authenticator) refreshToken(prev *oauth2.Token, lifetime time.Duration) (*oauth2.Token, error) {
	// Refresh the token under the lock.
	tok, cache, err := func() (*oauth2.Token, bool, error) {
		a.lock.Lock()
		defer a.lock.Unlock()

		// Some other goroutine already updated the token, just return new token.
		if a.token != nil && !internal.EqualTokens(a.token, prev) {
			return a.token, false, nil
		}

		// Rescan the cache. Maybe some other process updated the token. This branch
		// is also responsible for lazy-loading of token for non-interactive
		// providers, see ensureInitialized().
		if cached, _ := a.readTokenCache(); cached != nil {
			a.token = cached
			if !internal.EqualTokens(cached, prev) && !internal.TokenExpiresIn(a.ctx, cached, lifetime) {
				return cached, false, nil
			}
		}

		// Mint a new token or refresh the existing one.
		var err error
		if a.token == nil {
			// Can't do user interaction outside of Login.
			if a.provider.RequiresInteraction() {
				return nil, false, ErrLoginRequired
			}
			logging.Debugf(a.ctx, "Minting a new token, using %s", a.opts.Method)
			a.token, err = a.mintTokenWithRetries()
			if err != nil {
				logging.Warningf(a.ctx, "Failed to mint a token: %s", err)
				return nil, false, err
			}
		} else {
			logging.Debugf(a.ctx, "Refreshing the token")
			a.token, err = a.refreshTokenWithRetries(a.token)
			if err != nil {
				logging.Warningf(a.ctx, "Failed to refresh the token: %s", err)
				if err == internal.ErrBadRefreshToken && a.loginMode == OptionalLogin {
					logging.Warningf(a.ctx, "Switching to anonymous calls")
				}
				return nil, false, err
			}
		}
		lifetime := a.token.Expiry.Sub(clock.Now(a.ctx))
		logging.Debugf(a.ctx, "Token expires in %s", lifetime)
		return a.token, true, nil
	}()

	if err == internal.ErrBadRefreshToken || err == internal.ErrBadCredentials {
		// Do not keep broken token in the cache. It is unusable.
		if err := a.cache.Clear(); err != nil {
			logging.Warningf(a.ctx, "Failed to remove broken token from the cache: %s", err)
		}
		// Bad refresh token can be fixed by interactive logic. Bad credentials are
		// not fixable, so return the error as is in that case.
		if err == internal.ErrBadRefreshToken {
			err = ErrLoginRequired
		}
	}

	if err != nil {
		return nil, err
	}

	// Update the cache outside the lock, no need for callers to wait for this.
	// Do not die if failed, token is still usable from the memory.
	if cache && tok != nil {
		if err = a.cacheToken(tok); err != nil {
			logging.Warningf(a.ctx, "Failed to write refreshed token to the cache: %s", err)
		}
	}

	return tok, nil
}

////////////////////////////////////////////////////////////////////////////////
// Handling of transient errors.

// retryParams defines retry strategy for handling transient errors when minting
// or refreshing tokens.
func retryParams() retry.Iterator {
	return &retry.ExponentialBackoff{
		Limited: retry.Limited{
			Delay:    10 * time.Millisecond,
			Retries:  50,
			MaxTotal: 10 * time.Second,
		},
		Multiplier: 1.5,
	}
}

// mintTokenWithRetries calls provider's MintToken() retrying on transient
// errors a bunch of times. Called only for non-interactive providers.
func (a *Authenticator) mintTokenWithRetries() (tok *oauth2.Token, err error) {
	err = retry.Retry(a.ctx, retry.TransientOnly(retryParams), func() error {
		tok, err = a.provider.MintToken()
		return err
	}, nil)
	return
}

// refreshTokenWithRetries calls providers' RefreshToken(...) retrying on
// transient errors a bunch of times.
func (a *Authenticator) refreshTokenWithRetries(t *oauth2.Token) (tok *oauth2.Token, err error) {
	err = retry.Retry(a.ctx, retry.TransientOnly(retryParams), func() error {
		tok, err = a.provider.RefreshToken(a.token)
		return err
	}, nil)
	return
}

// authTokenInjector injects an authentication token into request headers.
//
// Used as a callback for NewModifyingTransport.
func (a *Authenticator) authTokenInjector(req *http.Request) error {
	// Attempt to refresh the token, if required. Revert to non-authed call if
	// token can't be refreshed and running in OptionalLogin mode.
	tok := a.currentToken()
	if tok == nil || internal.TokenExpiresIn(a.ctx, tok, minAcceptedLifetime) {
		var err error
		tok, err = a.refreshToken(tok, minAcceptedLifetime)
		switch {
		case err == ErrLoginRequired && a.loginMode == OptionalLogin:
			return nil // skip auth, no need for modifications
		case err != nil:
			return err
		case internal.TokenExpiresIn(a.ctx, tok, minAcceptedLifetime):
			return fmt.Errorf("auth: failed to refresh the token")
		}
	}
	tok.SetAuthHeader(req)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// tokenFileCache implementation.

// tokenFileCache implements TokenCache on top of the file system file.
//
// It caches only single token.
type tokenFileCache struct {
	path string
	ctx  context.Context
	lock sync.Mutex
}

func (c *tokenFileCache) Read() (buf []byte, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	buf, err = ioutil.ReadFile(c.path)
	if err != nil && os.IsNotExist(err) {
		err = nil
	}
	return
}

func (c *tokenFileCache) Write(buf []byte, exp time.Time) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	logging.Debugf(c.ctx, "Writing token to %s", c.path)
	err := os.MkdirAll(filepath.Dir(c.path), 0700)
	if err != nil {
		return err
	}
	// TODO(vadimsh): Make it atomic across multiple processes.
	return ioutil.WriteFile(c.path, buf, 0600)
}

func (c *tokenFileCache) Clear() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	err := os.Remove(c.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Utility functions.

// secretsDir returns directory with token cache files.
func (opts *Options) secretsDir() string {
	if opts.SecretsDir != "" {
		return opts.SecretsDir
	}
	return SecretsDir()
}

// cacheEntryName constructs a name of cache entry from data that identifies
// requested credential, to allow multiple differently configured instances of
// Authenticator to coexist.
func cacheEntryName(opts *Options, p internal.TokenProvider) string {
	sum := sha1.New()
	sum.Write([]byte(opts.Method))
	sum.Write([]byte{0})
	sum.Write([]byte(opts.ClientID))
	sum.Write([]byte{0})
	sum.Write([]byte(opts.ClientSecret))
	sum.Write([]byte{0})
	for _, scope := range opts.Scopes {
		sum.Write([]byte(scope))
		sum.Write([]byte{0})
	}
	sum.Write([]byte(opts.GCEAccountName))
	sum.Write(p.CacheSeed())
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

// pickServiceAccount returns a path to a JSON key to load.
//
// It is either the one specified in options or default one.
func pickServiceAccount(opts *Options) string {
	if opts.ServiceAccountJSONPath != "" {
		return opts.ServiceAccountJSONPath
	}
	return filepath.Join(opts.secretsDir(), "service_account.json")
}

// selectDefaultMethod is invoked in AutoSelectMethod mode.
//
// It looks at the options and the environment and picks the most appropriate
// authentication method.
func selectDefaultMethod(opts *Options) Method {
	if len(opts.ServiceAccountJSON) != 0 {
		return ServiceAccountMethod
	}
	serviceAccountPath := pickServiceAccount(opts)
	info, _ := os.Stat(serviceAccountPath)
	if info != nil && info.Mode().IsRegular() {
		return ServiceAccountMethod
	}
	if metadata.OnGCE() {
		return GCEMetadataMethod
	}
	return UserCredentialsMethod
}

// makeTokenProvider creates TokenProvider implementation based on options.
//
// Called by ensureInitialized.
func makeTokenProvider(ctx context.Context, opts *Options) (internal.TokenProvider, error) {
	// customTokenProvider is set only in unit tests.
	if opts.customTokenProvider != nil {
		return opts.customTokenProvider, nil
	}

	// Note: on appengine context.Context actually carries the transport used to
	// run OAuth2 flows. oauth2 library provides no way to customize it. So on
	// appengine opts.Context must be set and it must be derived from GAE request
	// context. opts.Transport is not used for token exchange flow itself, only
	// for authorized requests.
	switch opts.Method {
	case UserCredentialsMethod:
		return internal.NewUserAuthTokenProvider(
			ctx,
			opts.ClientID,
			opts.ClientSecret,
			opts.Scopes)
	case ServiceAccountMethod:
		serviceAccountPath := ""
		if len(opts.ServiceAccountJSON) == 0 {
			serviceAccountPath = pickServiceAccount(opts)
		}
		return internal.NewServiceAccountTokenProvider(
			ctx,
			opts.ServiceAccountJSON,
			serviceAccountPath,
			opts.Scopes)
	case GCEMetadataMethod:
		return internal.NewGCETokenProvider(ctx, opts.GCEAccountName, opts.Scopes)
	default:
		return nil, fmt.Errorf("auth: unrecognized authentication method: %s", opts.Method)
	}
}

// DefaultClient returns OAuth client_id and client_secret to use for 3 legged
// OAuth flow. Note that client_secret is not really a secret since it's
// hardcoded into the source code (and binaries). It's totally fine, as long
// as it's callback URI is configured to be 'localhost'. If someone decides to
// reuse such client_secret they have to run something on user's local machine
// to get the refresh_token.
func DefaultClient() (clientID string, clientSecret string) {
	clientID = "446450136466-2hr92jrq8e6i4tnsa56b52vacp7t3936.apps.googleusercontent.com"
	clientSecret = "uBfbay2KCy9t4QveJ-dOqHtp"
	return
}

// SecretsDir returns an absolute path to a directory to keep secret files in.
func SecretsDir() string {
	home, err := homedir.Dir()
	if err != nil {
		panic(err.Error())
	}
	return filepath.Join(home, ".config", "chrome_infra", "auth")
}
