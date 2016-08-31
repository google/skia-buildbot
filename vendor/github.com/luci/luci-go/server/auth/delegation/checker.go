// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package delegation

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/signing"

	"github.com/luci/luci-go/server/auth/delegation/internal"
)

const (
	// HTTPHeaderName is name of HTTP header that carries the token.
	HTTPHeaderName = "X-Delegation-Token-V1"

	// maxTokenSize is upper bound for expected size of a token (after base64
	// decoding). Larger tokens will be ignored right away.
	maxTokenSize = 8 * 1024

	// allowedClockDriftSec is how much clock difference we accept, in seconds.
	allowedClockDriftSec = int64(30)
)

var (
	// ErrMalformedDelegationToken is returned when delegation token cannot be
	// deserialized.
	ErrMalformedDelegationToken = errors.New("auth: malformed delegation token")

	// ErrUnsignedDelegationToken is returned if token's signature cannot be
	// verified.
	ErrUnsignedDelegationToken = errors.New("auth: unsigned delegation token")

	// ErrForbiddenDelegationToken is returned if token is structurally correct,
	// but some of its constraints prevents it from being used. For example, it is
	// already expired or it was minted for some other services, etc. See logs for
	// details.
	ErrForbiddenDelegationToken = errors.New("auth: forbidden delegation token")
)

// CertificatesProvider is accepted by 'CheckToken'.
type CertificatesProvider interface {
	// GetAuthServiceCertificates returns a bundle with certificates of a primary
	// auth service.
	GetAuthServiceCertificates(c context.Context) (*signing.PublicCertificates, error)
}

// GroupsChecker is accepted by 'CheckToken'.
type GroupsChecker interface {
	// IsMember returns true if the given identity belongs to the given group.
	//
	// Unknown groups are considered empty. May return errors if underlying
	// datastore has issues.
	IsMember(c context.Context, id identity.Identity, group string) (bool, error)
}

// CheckTokenParams is passed to CheckToken.
type CheckTokenParams struct {
	Token                string               // the delegation token to check
	PeerID               identity.Identity    // identity of the caller, as extracted from its credentials
	CertificatesProvider CertificatesProvider // returns auth service certificates
	GroupsChecker        GroupsChecker        // knows how to do group lookups
	OwnServiceIdentity   identity.Identity    // identity of the current service
}

// CheckToken verifies validity of a delegation token.
//
// If the token is valid, it returns the delegated identity (embedded in the
// token).
//
// May return transient errors.
func CheckToken(c context.Context, params CheckTokenParams) (identity.Identity, error) {
	// base64-encoded token -> DelegationToken proto (with signed serialized
	// subtoken).
	tok, err := deserializeToken(params.Token)
	if err != nil {
		logging.Warningf(c, "auth: Failed to deserialize delegation token - %s", err)
		return "", ErrMalformedDelegationToken
	}

	// Signed serialized subtoken -> Subtoken proto.
	subtoken, err := unsealToken(c, tok, params.CertificatesProvider)
	if err != nil {
		if errors.IsTransient(err) {
			logging.Warningf(c, "auth: Transient error when checking delegation token signature - %s", err)
			return "", err
		}
		logging.Warningf(c, "auth: Failed to check delegation token signature - %s", err)
		return "", ErrUnsignedDelegationToken
	}

	// Validate all constrains encoded in the token and derive the delegated
	// identity.
	return checkSubtoken(c, subtoken, &params)
}

// deserializeToken deserializes DelegationToken proto message.
func deserializeToken(token string) (*internal.DelegationToken, error) {
	blob, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	if len(blob) > maxTokenSize {
		return nil, fmt.Errorf("the delegation token is too big (%d bytes)", len(blob))
	}
	tok := &internal.DelegationToken{}
	if err = proto.Unmarshal(blob, tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// unsealToken verifies token's signature and deserializes the subtoken.
//
// May return transient errors.
func unsealToken(c context.Context, tok *internal.DelegationToken, certsProvider CertificatesProvider) (*internal.Subtoken, error) {
	// Grab the public keys of the primary auth service. It is the service that
	// signs tokens.
	//
	// TODO(vadimsh): There's 'signer_id' field in the DelegationToken proto. We
	// ignore it for now. If we ever support multiple trusted signers, we'd need
	// to start using it to pick the correct public key. For now only the central
	// auth service is trusted, so we just grab its certs.
	certs, err := certsProvider.GetAuthServiceCertificates(c)
	if err != nil {
		return nil, err
	}

	// Check the signature on the token.
	err = certs.CheckSignature(tok.GetSigningKeyId(), tok.SerializedSubtoken, tok.Pkcs1Sha256Sig)
	if err != nil {
		return nil, err
	}

	// The signature is correct! Deserialize the subtoken.
	msg := &internal.Subtoken{}
	if err = proto.Unmarshal(tok.SerializedSubtoken, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

// checkSubtoken validates the delegation subtoken.
//
// It extracts and returns original issuer_id.
func checkSubtoken(c context.Context, subtoken *internal.Subtoken, params *CheckTokenParams) (identity.Identity, error) {
	// Do fast checks before heavy ones.
	now := clock.Now(c).Unix()
	if err := checkSubtokenExpiration(subtoken, now); err != nil {
		logging.Warningf(c, "auth: Bad delegation token expiration - %s", err)
		return "", ErrForbiddenDelegationToken
	}
	if err := checkSubtokenServices(subtoken, params.OwnServiceIdentity); err != nil {
		logging.Warningf(c, "auth: Forbidden delegation token - %s", err)
		return "", ErrForbiddenDelegationToken
	}

	// Do the audience check (may use group lookups).
	if err := checkSubtokenAudience(c, subtoken, params.PeerID, params.GroupsChecker); err != nil {
		if errors.IsTransient(err) {
			logging.Warningf(c, "auth: Transient error when checking delegation token audience - %s", err)
			return "", err
		}
		logging.Warningf(c, "auth: Bad delegation token audience - %s", err)
		return "", ErrForbiddenDelegationToken
	}

	// Grab delegated identity.
	ident, err := identity.MakeIdentity(subtoken.GetIssuerId())
	if err != nil {
		logging.Warningf(c, "auth: Invalid issuer_id in the delegation token - %s", err)
		return "", ErrMalformedDelegationToken
	}

	return ident, nil
}

// checkSubtokenExpiration checks 'CreationTime' and 'ValidityDuration' fields.
func checkSubtokenExpiration(t *internal.Subtoken, now int64) error {
	creationTime := t.GetCreationTime()
	if creationTime <= 0 {
		return fmt.Errorf("invalid 'creation_time' field: %d", creationTime)
	}
	dur := int64(t.GetValidityDuration())
	if dur <= 0 {
		return fmt.Errorf("invalid validity_duration: %d", dur)
	}
	if creationTime >= now+allowedClockDriftSec {
		return fmt.Errorf("token is not active yet (created at %d)", creationTime)
	}
	if creationTime+dur < now {
		return fmt.Errorf("token has expired %d sec ago", now-(creationTime+dur))
	}
	return nil
}

// checkSubtokenServices makes sure the token is usable by the current service.
func checkSubtokenServices(t *internal.Subtoken, serviceID identity.Identity) error {
	// Empty services field is not allowed.
	if len(t.Services) == 0 {
		return fmt.Errorf("the token's services list is empty")
	}
	// Else, make sure we are in the 'services' list or it contains '*'.
	for _, allowed := range t.Services {
		if allowed == "*" || allowed == string(serviceID) {
			return nil
		}
	}
	return fmt.Errorf("token is not intended for %s", serviceID)
}

// checkSubtokenAudience makes sure the token is intended for use by given
// identity.
//
// May return transient errors.
func checkSubtokenAudience(c context.Context, t *internal.Subtoken, ident identity.Identity, checker GroupsChecker) error {
	// Empty audience field is not allowed.
	if len(t.Audience) == 0 {
		return fmt.Errorf("the token's audience list is empty")
	}
	// Try to find a direct hit first, to avoid calling expensive group lookups.
	for _, aud := range t.Audience {
		if aud == "*" || aud == string(ident) {
			return nil
		}
	}
	// Search through groups now.
	for _, aud := range t.Audience {
		if strings.HasPrefix(aud, "group:") {
			switch ok, err := checker.IsMember(c, ident, strings.TrimPrefix(aud, "group:")); {
			case err != nil:
				return err // transient error during group lookup
			case ok:
				return nil // success, 'ident' is in the target audience
			}
		}
	}
	return fmt.Errorf("%s is not allowed to use the token", ident)
}
