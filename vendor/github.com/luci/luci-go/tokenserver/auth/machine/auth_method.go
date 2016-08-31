// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package machine implements authentication based on LUCI machine tokens.
package machine

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/signing"

	"github.com/luci/luci-go/tokenserver/api"
)

const (
	// MachineTokenHeader is an HTTP header that carries the machine token.
	MachineTokenHeader = "X-Luci-Machine-Token"

	// TokenServersGroup is name of a group with trusted token servers.
	//
	// This group should contain service account emails of token servers we trust.
	TokenServersGroup = "auth-token-servers"

	// allowedClockDrift is how much clock difference we tolerate.
	allowedClockDrift = 10 * time.Second
)

var (
	// ErrBadToken is returned if the supplied machine token is not valid.
	//
	// See app logs for more details.
	ErrBadToken = errors.New("bad machine token")
)

// MachineTokenAuthMethod implements auth.Method by verifying machine tokens.
//
// It looks at X-Luci-Machine-Token header and verifies that it contains a valid
// non-expired machine token issued by some trusted token server instance.
//
// A list of trusted token servers is specified in 'auth-token-servers' group.
//
// If the token is valid, the request will be authenticated as coming from
// 'bot:<machine_fqdn>', where <machine_fqdn> is extracted from the token. It is
// lowercase FQDN of a machine (as specified in the certificate used to mint the
// token).
type MachineTokenAuthMethod struct {
	// certsFetcher is mocked in unit tests.
	//
	// In prod it is based on signing.FetchCertificatesForServiceAccount.
	certsFetcher func(c context.Context, email string) (*signing.PublicCertificates, error)
}

// Authenticate extracts peer's identity from the incoming request.
//
// It logs detailed errors in log, but returns only generic "bad credential"
// error to the caller, to avoid leaking unnecessary information.
func (m *MachineTokenAuthMethod) Authenticate(c context.Context, r *http.Request) (*auth.User, error) {
	token := r.Header.Get(MachineTokenHeader)
	if token == "" {
		return nil, nil // no token -> the auth method is not applicable
	}

	// Deserialize both envelope and the body.
	envelope, body, err := deserialize(token)
	if err != nil {
		logTokenError(c, r, body, err, "Failed to deserialize the token")
		return nil, ErrBadToken
	}

	// Construct an identity of a token server that signed the token to check that
	// it belongs to "auth-token-servers" group.
	signerServiceAccount, err := identity.MakeIdentity("user:" + body.IssuedBy)
	if err != nil {
		logTokenError(c, r, body, err, "Bad issued_by field - %q", body.IssuedBy)
		return nil, ErrBadToken
	}

	// Reject tokens from unknown token servers right away.
	db, err := auth.GetDB(c)
	if err != nil {
		return nil, errors.WrapTransient(err)
	}
	ok, err := db.IsMember(c, signerServiceAccount, TokenServersGroup)
	if err != nil {
		return nil, errors.WrapTransient(err)
	}
	if !ok {
		logTokenError(c, r, body, nil, "Unknown token issuer - %q", body.IssuedBy)
		return nil, ErrBadToken
	}

	// Check the expiration time before doing any heavier checks.
	if err = checkExpiration(body, clock.Now(c)); err != nil {
		logTokenError(c, r, body, err, "Token has expired or not yet valid")
		return nil, ErrBadToken
	}

	// Check the token was actually signed by the server.
	if err = m.checkSignature(c, body.IssuedBy, envelope); err != nil {
		if errors.IsTransient(err) {
			return nil, err
		}
		logTokenError(c, r, body, err, "Bad signature")
		return nil, ErrBadToken
	}

	// The token is valid. Construct the bot identity.
	botIdent, err := identity.MakeIdentity("bot:" + body.MachineFqdn)
	if err != nil {
		logTokenError(c, r, body, err, "Bad machine_fqdn - %q", body.MachineFqdn)
		return nil, ErrBadToken
	}
	return &auth.User{Identity: botIdent}, nil
}

// logTokenError adds a warning-level log entry with details about the request.
func logTokenError(c context.Context, r *http.Request, tok *tokenserver.MachineTokenBody, err error, msg string, args ...string) {
	fields := logging.Fields{"remoteAddr": r.RemoteAddr}
	if tok != nil {
		// Note that if token wasn't properly signed, these fields may contain
		// garbage.
		fields["machineFqdn"] = tok.MachineFqdn
		fields["issuedBy"] = tok.IssuedBy
		fields["issuedAt"] = tok.IssuedAt
		fields["lifetime"] = tok.Lifetime
		fields["caId"] = tok.CaId
		fields["certSn"] = tok.CertSn
	}
	if err != nil {
		fields[logging.ErrorKey] = err
	}
	fields.Warningf(c, msg, args)
}

// deserialize parses MachineTokenEnvelope and MachineTokenBody.
func deserialize(token string) (*tokenserver.MachineTokenEnvelope, *tokenserver.MachineTokenBody, error) {
	tokenBinBlob, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return nil, nil, err
	}
	envelope := &tokenserver.MachineTokenEnvelope{}
	if err := proto.Unmarshal(tokenBinBlob, envelope); err != nil {
		return nil, nil, err
	}
	body := &tokenserver.MachineTokenBody{}
	if err := proto.Unmarshal(envelope.TokenBody, body); err != nil {
		return envelope, nil, err
	}
	return envelope, body, nil
}

// checkExpiration returns nil if the token is non-expired yet.
//
// Allows some clock drift, see allowedClockDrift.
func checkExpiration(body *tokenserver.MachineTokenBody, now time.Time) error {
	notBefore := time.Unix(int64(body.IssuedAt), 0)
	notAfter := notBefore.Add(time.Duration(body.Lifetime) * time.Second)
	if now.Before(notBefore.Add(-allowedClockDrift)) {
		diff := notBefore.Sub(now)
		return fmt.Errorf("token is not valid yet, will be valid in %s", diff)
	}
	if now.After(notAfter.Add(allowedClockDrift)) {
		diff := now.Sub(notAfter)
		return fmt.Errorf("token expired %s ago", diff)
	}
	return nil
}

// checkSignature verifies the token signature.
func (m *MachineTokenAuthMethod) checkSignature(c context.Context, signerEmail string, envelope *tokenserver.MachineTokenEnvelope) error {
	// Note that FetchCertificatesForServiceAccount implements caching inside.
	fetcher := m.certsFetcher
	if fetcher == nil {
		fetcher = signing.FetchCertificatesForServiceAccount
	}
	certs, err := fetcher(c, signerEmail)
	if err != nil {
		return errors.WrapTransient(err)
	}
	return certs.CheckSignature(envelope.KeyId, envelope.TokenBody, envelope.RsaSha256)
}
