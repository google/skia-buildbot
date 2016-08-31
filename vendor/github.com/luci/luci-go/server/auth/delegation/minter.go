// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package delegation

import (
	"encoding/gob"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/internal"
)

// TokenRequest describes parameters of a new delegation token.
type TokenRequest struct {
	// AuthServiceURL is root URL (e.g. https://<host>) of the service to use for
	// minting the delegation token.
	//
	// The token will be signed by the service's private key. Only services that
	// trust this auth service would be able to accept the new token.
	//
	// Required.
	AuthServiceURL string

	// Audience is to whom caller's identity is delegated.
	//
	// Only clients that can prove they are intended audience (e.g. by presenting
	// valid access token) would be able to use the delegation token.
	//
	// Must be empty if UnlimitedAudience is true.
	Audience []identity.Identity

	// AudienceGroups can be used to specify a group (or a bunch of groups) as
	// a target audience of the token.
	//
	// It works in addition to Audience.
	//
	// Must be empty if UnlimitedAudience is true.
	AudienceGroups []string

	// UnlimitedAudience, if true, indicates that the delegation token can be
	// used by any bearer.
	//
	// Tokens with unlimited audience are roughly as powerful as OAuth access
	// tokens and should be used only if absolutely necessary.
	//
	// Prefer to limit token's audience as much as possible. See Audience and
	// AudienceGroups fields above.
	//
	// If UnlimitedAudience is true, Audience and AudienceGroups must be empty.
	UnlimitedAudience bool

	// TargetServices is a list of 'service:...' identities with services that
	// accept the token.
	//
	// This can be used to limit the scope of the token.
	//
	// Must be empty if Untargeted is true.
	TargetServices []identity.Identity

	// Untargeted, if true, indicates that the delegation token should be accepted
	// by any LUCI service.
	//
	// Use this if you are preparing a token in advance, not yet knowing where it
	// is going to be used.
	//
	// If Untargeted is true, TargetServices must be empty.
	Untargeted bool

	// Impersonate defines on whose behalf the token is being created.
	//
	// By default the token delegates an identity of whoever requested it. This
	// identity is extracted from credentials that accompany token minting request
	// (e.g. OAuth access token).
	//
	// By using 'Impersonate' a caller can make a token that delegates someone
	// else's identity, effectively producing an impersonation token.
	//
	// Only limited set of callers can do this. The set of who can be impersonated
	// is also limited. The rules are enforced by the auth service.
	Impersonate identity.Identity

	// ValidityDuration defines for how long the token would be valid.
	//
	// Maximum theoretical TTL is limited by the lifetime of the signing key of
	// the auth service (~= 24 hours). Minimum acceptable value is 30 sec.
	//
	// Required.
	ValidityDuration time.Duration

	// Intent is a reason why the token is created.
	//
	// Used only for logging purposes on the auth service, will be indexed. Should
	// be a short identifier-like string.
	//
	// Optional.
	Intent string
}

// Token is actual delegation token with its expiration time and ID.
type Token struct {
	Token      string    // base64-encoded URL-safe blob with the token
	Expiry     time.Time // UTC time when it expires (also encoded in Token)
	SubtokenID string    // identifier of the token (also encoded in Token)
}

// CreateToken makes a request to the auth service to generate the token.
//
// If uses current service's credentials to authenticate the request.
//
// If req.Impersonate is not used, the identity encoded in the authentication
// credentials will be delegated by the token, otherwise the service will check
// that caller is allowed to do the impersonation and will return a token that
// delegates the identity specified by req.Impersonate.
func CreateToken(c context.Context, req TokenRequest) (*Token, error) {
	// See https://github.com/luci/luci-py/blob/master/appengine/auth_service/delegation.py.
	var params struct {
		Audience         []string `json:"audience,omitempty"`
		Services         []string `json:"services,omitempty"`
		ValidityDuration int      `json:"validity_duration"`
		Impersonate      string   `json:"impersonate,omitempty"`
		Intent           string   `json:"intent,omitempty"`
	}

	// Audience.
	params.Audience = make([]string, 0, len(req.Audience)+len(req.AudienceGroups))
	for _, aud := range req.Audience {
		if err := aud.Validate(); err != nil {
			return nil, err
		}
		params.Audience = append(params.Audience, string(aud))
	}
	for _, group := range req.AudienceGroups {
		params.Audience = append(params.Audience, "group:"+group)
	}
	switch {
	case req.UnlimitedAudience && len(params.Audience) != 0:
		return nil, fmt.Errorf("delegation: can't specify audience for UnlimitedAudience=true token")
	case !req.UnlimitedAudience && len(params.Audience) == 0:
		return nil, fmt.Errorf("delegation: either Audience/AudienceGroups or UnlimitedAudience=true are required")
	}
	if req.UnlimitedAudience {
		params.Audience = []string{"*"}
	}

	// Services.
	params.Services = make([]string, 0, len(req.TargetServices))
	for _, srv := range req.TargetServices {
		if err := srv.Validate(); err != nil {
			return nil, err
		}
		params.Services = append(params.Services, string(srv))
	}
	switch {
	case req.Untargeted && len(params.Services) != 0:
		return nil, fmt.Errorf("delegation: can't specify TargetServices for Untargeted=true token")
	case !req.Untargeted && len(params.Services) == 0:
		return nil, fmt.Errorf("delegation: either TargetServices or Untargeted=true are required")
	}
	if req.Untargeted {
		params.Services = []string{"*"}
	}

	// Validity duration.
	params.ValidityDuration = int(req.ValidityDuration / time.Second)
	if params.ValidityDuration < 30 {
		return nil, fmt.Errorf("ValidityDuration must be >= 30 sec, got %s", req.ValidityDuration)
	}
	if params.ValidityDuration > 24*3600 {
		return nil, fmt.Errorf("ValidityDuration must be <= 24h, got %s", req.ValidityDuration)
	}

	// The rest of the fields.
	if req.Impersonate != "" {
		if err := req.Impersonate.Validate(); err != nil {
			return nil, err
		}
		params.Impersonate = string(req.Impersonate)
	}
	params.Intent = req.Intent

	var response struct {
		DelegationToken  string `json:"delegation_token,omitempty"`
		ValidityDuration int    `json:"validity_duration,omitempty"`
		SubtokenID       string `json:"subtoken_id,omitempty"`
		Text             string `json:"text,omitempty"` // for error responses
	}

	httpReq := internal.Request{
		Method: "POST",
		URL:    req.AuthServiceURL + "/auth_service/api/v1/delegation/token/create",
		Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
		Body:   &params,
		Out:    &response,
	}
	if err := httpReq.Do(c); err != nil {
		if response.Text != "" {
			err = fmt.Errorf("%s - %s", err, response.Text)
		}
		return nil, err
	}

	return &Token{
		Token:      response.DelegationToken,
		Expiry:     clock.Now(c).Add(time.Duration(response.ValidityDuration) * time.Second).UTC(),
		SubtokenID: response.SubtokenID,
	}, nil
}

func init() {
	// For token cache.
	gob.Register(Token{})
}
