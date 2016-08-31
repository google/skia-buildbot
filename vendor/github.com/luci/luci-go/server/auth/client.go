// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/server/auth/delegation"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/internal"
)

// RPCAuthorityKind defines under whose authority RPCs are made.
type RPCAuthorityKind int

const (
	// NoAuth is used for outbound RPCs that don't have any implicit auth headers.
	NoAuth RPCAuthorityKind = iota

	// AsSelf is used for outbound RPCs sent with the authority of the current
	// service itself.
	//
	// RPC requests done in this mode will have 'Authorization' header set to the
	// OAuth2 access token of the service's own service account.
	//
	// By default uses "https://www.googleapis.com/auth/userinfo.email" API scope.
	// Can be customized with WithScopes() options.
	AsSelf

	// AsUser is used for outbound RPCs that inherit the authority of a user
	// that initiated the request that is currently being handled.
	//
	// If the current request was initiated by an anonymous caller, the RPC will
	// have no auth headers (just like in NoAuth mode).
	//
	// Can also be used together with MintDelegationToken to make requests on
	// user behalf asynchronously. For example, to associate end-user authority
	// with some delayed task, call MintDelegationToken (in a context of a user
	// initiated request) when this task is created and store the resulting token
	// along with the task. Then, to make an RPC on behalf of the user from the
	// task use GetRPCTransport(ctx, AsUser, WithDelegationToken(token)).
	//
	// The implementation is based on LUCI-specific protocol that uses special
	// delegation tokens. Only LUCI backends can understand them.
	AsUser
)

// RPCOption is an option for GetRPCTransport or GetPerRPCCredentials functions.
type RPCOption interface {
	apply(opts *rpcOptions)
}

// WithScopes can be used to customize OAuth scopes for outbound RPC requests.
//
// If not used, the requests are made with "userinfo.email" scope.
func WithScopes(scopes ...string) RPCOption {
	return oauthScopesOption{scopes: scopes}
}

type oauthScopesOption struct {
	scopes []string
}

func (o oauthScopesOption) apply(opts *rpcOptions) {
	opts.scopes = append(opts.scopes, o.scopes...)
}

// WithDelegationToken can be used to attach an existing delegation token to
// requests made in AsUser mode.
//
// The token can be obtained earlier via MintDelegationToken call. The transport
// doesn't attempt to validate it and just blindly sends it to the other side.
func WithDelegationToken(token string) RPCOption {
	return delegationTokenOption{token: token}
}

type delegationTokenOption struct {
	token string
}

func (o delegationTokenOption) apply(opts *rpcOptions) {
	opts.delegationToken = o.token
}

// GetRPCTransport returns http.RoundTripper to use for outbound HTTP RPC
// requests.
//
// Usage:
//    tr, err := auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes("..."))
//    if err != nil {
//      return err
//    }
//    client := &http.Client{Transport: tr}
//    ...
func GetRPCTransport(c context.Context, kind RPCAuthorityKind, opts ...RPCOption) (http.RoundTripper, error) {
	options, err := makeRpcOptions(kind, opts)
	if err != nil {
		return nil, err
	}
	config := GetConfig(c)
	if config == nil || config.AnonymousTransport == nil {
		return nil, ErrNotConfigured
	}
	baseTransport := config.AnonymousTransport(c)
	if options.kind == NoAuth {
		return baseTransport, nil
	}
	return auth.NewModifyingTransport(baseTransport, func(req *http.Request) error {
		headers, err := options.getRPCHeaders(c, req.URL.String(), options)
		if err != nil {
			return err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return nil
	}), nil
}

// GetPerRPCCredentials returns gRPC's PerRPCCredentials implementation.
//
// It can be used to authenticate outbound gPRC RPC's.
func GetPerRPCCredentials(kind RPCAuthorityKind, opts ...RPCOption) (credentials.PerRPCCredentials, error) {
	options, err := makeRpcOptions(kind, opts)
	if err != nil {
		return nil, err
	}
	return perRPCCreds{options}, nil
}

type perRPCCreds struct {
	options *rpcOptions
}

func (creds perRPCCreds) GetRequestMetadata(c context.Context, uri ...string) (map[string]string, error) {
	if len(uri) == 0 {
		panic("perRPCCreds: no URI given")
	}
	return creds.options.getRPCHeaders(c, uri[0], creds.options)
}

func (creds perRPCCreds) RequireTransportSecurity() bool {
	return true
}

////////////////////////////////////////////////////////////////////////////////
// Internal stuff.

func init() {
	// This is needed to allow packages imported by 'server/auth' to make
	// authenticated calls. They can't use GetRPCTransport directly, since they
	// can't import 'server/auth' (it creates an import cycle).
	internal.RegisterClientFactory(func(c context.Context, scopes []string) (*http.Client, error) {
		var t http.RoundTripper
		var err error
		if len(scopes) == 0 {
			t, err = GetRPCTransport(c, NoAuth)
		} else {
			t, err = GetRPCTransport(c, AsSelf, WithScopes(scopes...))
		}
		if err != nil {
			return nil, err
		}
		return &http.Client{Transport: t}, nil
	})
}

// rpcMocks are used exclusively in unit tests.
type rpcMocks struct {
	MintDelegationToken func(context.Context, DelegationTokenParams) (*delegation.Token, error)
}

// apply implements RPCOption interface.
func (o *rpcMocks) apply(opts *rpcOptions) {
	opts.rpcMocks = o
}

var defaultOAuthScopes = []string{auth.OAuthScopeEmail}

type headersGetter func(c context.Context, uri string, opts *rpcOptions) (map[string]string, error)

type rpcOptions struct {
	kind            RPCAuthorityKind
	scopes          []string
	delegationToken string
	getRPCHeaders   headersGetter
	rpcMocks        *rpcMocks
}

// makeRpcOptions applies all options and validates them.
func makeRpcOptions(kind RPCAuthorityKind, opts []RPCOption) (*rpcOptions, error) {
	options := &rpcOptions{kind: kind}
	for _, o := range opts {
		o.apply(options)
	}
	// Validate options.
	switch {
	case options.kind == AsSelf && len(options.scopes) == 0:
		options.scopes = defaultOAuthScopes
	case options.kind != AsSelf && len(options.scopes) != 0:
		return nil, fmt.Errorf("auth: WithScopes can only be used with AsSelf authorization kind")
	}
	if options.delegationToken != "" && options.kind != AsUser {
		return nil, fmt.Errorf("auth: WithDelegationToken can only be used with AsUser authorization kind")
	}
	// Validate 'kind' and pick correct implementation of getRPCHeaders.
	switch options.kind {
	case NoAuth:
		options.getRPCHeaders = noAuthHeaders
	case AsSelf:
		options.getRPCHeaders = asSelfHeaders
	case AsUser:
		options.getRPCHeaders = asUserHeaders
	default:
		return nil, fmt.Errorf("auth: unknown RPCAuthorityKind %d", options.kind)
	}
	return options, nil
}

// noAuthHeaders is getRPCHeaders for NoAuth mode.
func noAuthHeaders(c context.Context, uri string, opts *rpcOptions) (map[string]string, error) {
	return nil, nil
}

// asSelfHeaders returns a map of authentication headers to add to outbound
// RPC requests done in AsSelf mode.
//
// This will be called by the transport layer on each request.
func asSelfHeaders(c context.Context, uri string, opts *rpcOptions) (map[string]string, error) {
	cfg := GetConfig(c)
	if cfg == nil || cfg.AccessTokenProvider == nil {
		return nil, ErrNotConfigured
	}
	tok, err := cfg.AccessTokenProvider(c, opts.scopes)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Authorization": tok.TokenType + " " + tok.AccessToken,
	}, nil
}

// asUserHeaders returns a map of authentication headers to add to outbound
// RPC requests done in AsUser mode.
//
// This will be called by the transport layer on each request.
func asUserHeaders(c context.Context, uri string, opts *rpcOptions) (map[string]string, error) {
	cfg := GetConfig(c)
	if cfg == nil || cfg.AccessTokenProvider == nil {
		return nil, ErrNotConfigured
	}

	delegationToken := ""
	if opts.delegationToken != "" {
		delegationToken = opts.delegationToken // WithDelegationToken was used
	} else {
		// Outbound RPC calls in the context of a request from anonymous caller are
		// anonymous too. No need to use any authentication headers.
		userIdent := CurrentIdentity(c)
		if userIdent == identity.AnonymousIdentity {
			return nil, nil
		}

		// Grab root URL of the destination service. Only https:// are allowed.
		uri = strings.ToLower(uri)
		if !strings.HasPrefix(uri, "https://") {
			return nil, fmt.Errorf("auth: refusing to use delegation tokens with non-https URL")
		}
		host := uri[len("https://"):]
		if idx := strings.IndexRune(host, '/'); idx != -1 {
			host = host[:idx]
		}

		// Grab a token that's good enough for at least 10 min. Outbound RPCs
		// shouldn't last longer than that.
		mintTokenCall := MintDelegationToken
		if opts.rpcMocks != nil && opts.rpcMocks.MintDelegationToken != nil {
			mintTokenCall = opts.rpcMocks.MintDelegationToken
		}
		tok, err := mintTokenCall(c, DelegationTokenParams{
			TargetHost: host,
			MinTTL:     10 * time.Minute,
		})
		if err != nil {
			return nil, err
		}
		delegationToken = tok.Token
	}

	// Use our own OAuth token too, since the delegation token is bound to us.
	oauthTok, err := cfg.AccessTokenProvider(c, []string{auth.OAuthScopeEmail})
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Authorization":           oauthTok.TokenType + " " + oauthTok.AccessToken,
		delegation.HTTPHeaderName: delegationToken,
	}, nil
}
