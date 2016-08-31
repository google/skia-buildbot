// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package tokenminter implements TokenMinter API.
//
// This is main public API of The Token Server.
package tokenminter

import (
	"crypto/x509"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server/gaesigner"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/signing"

	"github.com/luci/luci-go/tokenserver/api"
	"github.com/luci/luci-go/tokenserver/api/admin/v1"
	"github.com/luci/luci-go/tokenserver/api/minter/v1"

	"github.com/luci/luci-go/tokenserver/appengine/certchecker"
	"github.com/luci/luci-go/tokenserver/appengine/machinetoken"
	"github.com/luci/luci-go/tokenserver/appengine/model"
	"github.com/luci/luci-go/tokenserver/appengine/services/admin/serviceaccounts"
)

// Server implements minter.TokenMinterServer RPC interface.
//
// Use NewServer to make one.
type Server struct {
	// mintOAuthToken is mocked in tests.
	//
	// In prod it is serviceaccounts.Server.DoMintAccessToken.
	mintOAuthToken func(context.Context, serviceaccounts.MintAccessTokenParams) (*tokenserver.ServiceAccount, *minter.OAuth2AccessToken, error)

	// certChecker is mocked in tests.
	//
	// In prod it is certchecker.CheckCertificate.
	certChecker func(c context.Context, cert *x509.Certificate) (*model.CA, error)

	// signer is mocked in tests.
	//
	// In prod it is gaesigner.Signer.
	signer signing.Signer

	// isAdmin returns true if current user is an administrator.
	//
	// Mocked in tests. In prod it is 'auth.IsMember'.
	isAdmin func(context.Context) (bool, error)

	// signerServiceAccount returns service account email of the token server.
	//
	// Mocked in tests. In prod it is info.Get().ServiceAccount().
	signerServiceAccount func(context.Context) (string, error)

	// initLock protects ensureInitialized and 'initialized'.
	initLock sync.RWMutex

	// initialized contains a static state set up in 'ensureInitialized'.
	initialized *initializedState
}

type initializedState struct {
	signerServiceAccount string // cached result of 'signerServiceAccount'
	serviceVersion       string // put into 'ServiceVersion' field of TokenResponse
}

// NewServer returns Server configured for real production usage.
func NewServer(sa *serviceaccounts.Server) *Server {
	return &Server{
		mintOAuthToken: sa.DoMintAccessToken,
		certChecker:    certchecker.CheckCertificate,
		signer:         gaesigner.Signer{},
		isAdmin: func(c context.Context) (bool, error) {
			return auth.IsMember(c, "administrators")
		},
		signerServiceAccount: func(c context.Context) (string, error) {
			return info.Get(c).ServiceAccount()
		},
	}
}

// ensureInitialized is called to lazy-initialize the server on first use when
// we have active context.Context.
func (s *Server) ensureInitialized(c context.Context) (out initializedState, err error) {
	// We don't use sync.Once here because if initialization fails with a
	// transient error there's no way to "undo" sync.Once.Do. Instead we use next
	// best thing possible: RWMutex that optimistically assumes mostly reads.
	s.initLock.RLock()
	if s.initialized != nil {
		out = *s.initialized
		s.initLock.RUnlock()
		return out, nil
	}
	s.initLock.RUnlock()

	s.initLock.Lock()
	defer s.initLock.Unlock()
	if s.initialized != nil {
		out = *s.initialized
		return out, nil
	}

	if out.signerServiceAccount, err = s.signerServiceAccount(c); err != nil {
		return out, err
	}

	// serviceVersion is returned in MintTokenResponse, it identifies the app
	// and its version to the client. Used for monitoring purposes.
	i := info.Get(c)
	appVersion := strings.Split(i.VersionID(), ".")[0]
	out.serviceVersion = fmt.Sprintf("%s/%s", i.AppID(), appVersion)

	// Success! Remember this.
	s.initialized = &out
	return out, nil
}

// MintMachineToken generates a new token for an authenticated machine.
func (s *Server) MintMachineToken(c context.Context, req *minter.MintMachineTokenRequest) (*minter.MintMachineTokenResponse, error) {
	// Parse serialized portion of the request and do minimal validation before
	// checking the signature to reject obviously bad requests.
	if len(req.SerializedTokenRequest) == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "empty request")
	}
	tokenReq := minter.MachineTokenRequest{}
	if err := proto.Unmarshal(req.SerializedTokenRequest, &tokenReq); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "failed to unmarshal TokenRequest - %s", err)
	}

	switch tokenReq.TokenType {
	case minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN:
	case minter.TokenType_LUCI_MACHINE_TOKEN:
		// supported
	default:
		return s.mintingErrorResponse(
			c, minter.ErrorCode_UNSUPPORTED_TOKEN_TYPE,
			"token_type %s is not supported", tokenReq.TokenType)
	}

	// Timestamp is required.
	issuedAt := tokenReq.IssuedAt.Time()
	if issuedAt.IsZero() {
		return s.mintingErrorResponse(c, minter.ErrorCode_BAD_TIMESTAMP, "issued_at is required")
	}

	// It should be within acceptable range.
	now := clock.Now(c)
	notBefore := now.Add(-10 * time.Minute)
	notAfter := now.Add(10 * time.Minute)
	if issuedAt.Before(notBefore) || issuedAt.After(notAfter) {
		return s.mintingErrorResponse(
			c, minter.ErrorCode_BAD_TIMESTAMP,
			"issued_at timestamp is not within acceptable range, check your clock")
	}

	// The certificate must be valid.
	cert, err := x509.ParseCertificate(tokenReq.Certificate)
	if err != nil {
		return s.mintingErrorResponse(
			c, minter.ErrorCode_BAD_CERTIFICATE_FORMAT,
			"failed to parse the certificate (expecting x509 cert DER)")
	}

	// Check the signature before proceeding. Use switch when picking an algo
	// as a reminder to add a new branch if new signature scheme is added.
	var algo x509.SignatureAlgorithm
	switch tokenReq.SignatureAlgorithm {
	case minter.SignatureAlgorithm_SHA256_RSA_ALGO:
		algo = x509.SHA256WithRSA
	default:
		return s.mintingErrorResponse(
			c, minter.ErrorCode_UNSUPPORTED_SIGNATURE,
			"signature_algorithm %s is not supported", tokenReq.SignatureAlgorithm)
	}
	err = cert.CheckSignature(algo, req.SerializedTokenRequest, req.Signature)
	if err != nil {
		return s.mintingErrorResponse(
			c, minter.ErrorCode_BAD_SIGNATURE,
			"signature verification failed - %s", err)
	}

	// At this point we know the request was signed by the holder of a private key
	// that matches the certificate.
	//
	// Let's make sure the token server knows about that key, i.e. the certificate
	// itself is signed by some trusted CA, it is valid (not expired), and it
	// hasn't been revoked yet. CertChecker does these checks.
	ca, err := s.certChecker(c, cert)

	// Recognize error codes related to CA cert checking. Everything else is
	// transient errors.
	if err != nil {
		if certchecker.IsCertInvalidError(err) {
			return s.mintingErrorResponse(c, minter.ErrorCode_UNTRUSTED_CERTIFICATE, "%s", err)
		}
		return nil, grpc.Errorf(codes.Internal, "failed to check the certificate - %s", err)
	}

	// At this point we trust what's in MachineTokenRequest, proceed with
	// generating the token.
	args := mintTokenArgs{
		Config:  ca.ParsedConfig,
		Cert:    cert,
		Request: &tokenReq,
	}
	switch tokenReq.TokenType {
	case minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN:
		return s.mintGoogleOAuth2AccessToken(c, args)
	case minter.TokenType_LUCI_MACHINE_TOKEN:
		return s.mintLuciMachineToken(c, args)
	default:
		panic("impossible") // there's a check above
	}
}

type mintTokenArgs struct {
	Config  *admin.CertificateAuthorityConfig
	Cert    *x509.Certificate
	Request *minter.MachineTokenRequest
}

func (s *Server) mintGoogleOAuth2AccessToken(c context.Context, args mintTokenArgs) (*minter.MintMachineTokenResponse, error) {
	// Validate FQDN, scopes, check they are whitelisted (the whitelist is part of
	// the config).
	params := serviceaccounts.MintAccessTokenParams{
		Config: args.Config,
		FQDN:   strings.ToLower(args.Cert.Subject.CommonName),
		Scopes: args.Request.Oauth2Scopes,
	}
	if err := params.Validate(); err != nil {
		return s.mintingErrorResponse(c, minter.ErrorCode_BAD_TOKEN_ARGUMENTS, "%s", err)
	}

	// Grab 'serviceVersion' needed below.
	state, err := s.ensureInitialized(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't initialize service guts - %s", err)
	}

	// Grab the token. It returns grpc error already, but we need to convert it
	// to MintMachineTokenResponse, unless it is a transient error (then we pass
	// it through as is, to retain its "transience").
	account, token, err := s.mintOAuthToken(c, params)
	switch {
	case err == nil:
		return &minter.MintMachineTokenResponse{
			ServiceVersion: state.serviceVersion,
			TokenResponse: &minter.MachineTokenResponse{
				ServiceAccount: account,
				ServiceVersion: state.serviceVersion,
				TokenType: &minter.MachineTokenResponse_GoogleOauth2AccessToken{
					GoogleOauth2AccessToken: token,
				},
			},
		}, nil
	case grpc.Code(err) == codes.Internal:
		return nil, err
	default:
		return s.mintingErrorResponse(c, minter.ErrorCode_TOKEN_MINTING_ERROR, "%s", err)
	}
}

func (s *Server) mintLuciMachineToken(c context.Context, args mintTokenArgs) (*minter.MintMachineTokenResponse, error) {
	state, err := s.ensureInitialized(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't initialize service guts - %s", err)
	}

	// Validate FQDN and whether it is allowed by config.
	params := machinetoken.MintParams{
		FQDN:                 strings.ToLower(args.Cert.Subject.CommonName),
		Cert:                 args.Cert,
		Config:               args.Config,
		SignerServiceAccount: state.signerServiceAccount,
		Signer:               s.signer,
	}
	if err := params.Validate(); err != nil {
		return s.mintingErrorResponse(c, minter.ErrorCode_BAD_TOKEN_ARGUMENTS, "%s", err)
	}

	// Make the token.
	switch body, signedToken, err := machinetoken.Mint(c, params); {
	case err == nil:
		expiry := time.Unix(int64(body.IssuedAt), 0).Add(time.Duration(body.Lifetime) * time.Second)
		return &minter.MintMachineTokenResponse{
			ServiceVersion: state.serviceVersion,
			TokenResponse: &minter.MachineTokenResponse{
				ServiceVersion: state.serviceVersion,
				TokenType: &minter.MachineTokenResponse_LuciMachineToken{
					LuciMachineToken: &minter.LuciMachineToken{
						MachineToken: signedToken,
						Expiry:       google.NewTimestamp(expiry),
					},
				},
			},
		}, nil
	case errors.IsTransient(err):
		return nil, grpc.Errorf(codes.Internal, "failed to generate machine token - %s", err)
	default:
		return s.mintingErrorResponse(c, minter.ErrorCode_TOKEN_MINTING_ERROR, "%s", err)
	}
}

func (s *Server) mintingErrorResponse(c context.Context, code minter.ErrorCode, msg string, args ...interface{}) (*minter.MintMachineTokenResponse, error) {
	state, err := s.ensureInitialized(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't initialize service guts - %s", err)
	}
	return &minter.MintMachineTokenResponse{
		ErrorCode:      code,
		ErrorMessage:   fmt.Sprintf(msg, args...),
		ServiceVersion: state.serviceVersion,
	}, nil
}

// InspectMachineToken decodes a machine token and verifies it is valid.
func (s *Server) InspectMachineToken(c context.Context, req *minter.InspectMachineTokenRequest) (*minter.InspectMachineTokenResponse, error) {
	isAdmin, err := s.isAdmin(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't check group membership - %s", err)
	}
	if !isAdmin {
		logging.Errorf(c, "InspectMachineToken is used by non-admin: %s", auth.CurrentIdentity(c))
		return nil, grpc.Errorf(codes.PermissionDenied, "not authorized")
	}

	// Only LUCI_MACHINE_TOKEN is supported currently.
	switch req.TokenType {
	case minter.TokenType_LUCI_MACHINE_TOKEN:
		// supported
	default:
		return nil, grpc.Errorf(codes.InvalidArgument, "unsupported token type %s", req.TokenType)
	}

	// Deserialize the token, don't do any validity checks yet.
	envelope, body, err := machinetoken.Parse(req.Token)
	switch {
	case envelope == nil:
		return &minter.InspectMachineTokenResponse{
			InvalidityReason: fmt.Sprintf("bad envelope format - %s", err),
		}, nil
	case body == nil:
		return &minter.InspectMachineTokenResponse{
			SigningKeyId:     envelope.KeyId,
			InvalidityReason: fmt.Sprintf("bad body format - %s", err),
		}, nil
	case err != nil:
		return &minter.InspectMachineTokenResponse{
			InvalidityReason: fmt.Sprintf("parsing error - %s", err),
		}, nil
	}

	resp := &minter.InspectMachineTokenResponse{
		InvalidityReason: "unknown", // will be replaced below
		SigningKeyId:     envelope.KeyId,
		TokenType: &minter.InspectMachineTokenResponse_LuciMachineToken{
			LuciMachineToken: body,
		},
	}

	// Check that the token was signed by our private key.
	certs, err := s.signer.Certificates(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't fetch service certificates - %s", err)
	}
	err = machinetoken.CheckSignature(envelope, certs)
	if err != nil {
		resp.InvalidityReason = fmt.Sprintf("can't validate signature - %s", err)
		return resp, nil
	}
	resp.Signed = true

	// Check the expiration time. Allow 10 sec clock drift.
	resp.NonExpired = !machinetoken.IsExpired(body, clock.Now(c))

	// Check revocation status. Find CA name that signed the certificate used when
	// minting the token.
	caName, err := model.GetCAByUniqueID(c, body.CaId)
	switch {
	case err != nil:
		return nil, grpc.Errorf(codes.Internal, "can't resolve ca_id to CA name - %s", err)
	case caName == "":
		resp.InvalidityReason = "no CA with given ID"
		return resp, nil
	}
	resp.CertCaName = caName

	// Grab CertChecker for this CA. It has CRL cached.
	certChecker, err := certchecker.GetCertChecker(c, caName)
	switch {
	case errors.IsTransient(err):
		return nil, grpc.Errorf(codes.Internal, "can't fetch CRL - %s", err)
	case err != nil:
		resp.InvalidityReason = fmt.Sprintf("can't fetch CRL - %s", err)
		return resp, nil
	}

	// Check that certificate SN is not in the revocation list.
	sn := big.NewInt(0).SetUint64(body.CertSn)
	revoked, err := certChecker.CRL.IsRevokedSN(c, sn)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't check CRL - %s", err)
	}
	resp.NonRevoked = !revoked

	// Pick invalidity reason (if any).
	switch {
	case !resp.NonExpired:
		resp.InvalidityReason = "expired"
	case !resp.NonRevoked:
		resp.InvalidityReason = "revoked"
	default:
		resp.Valid = true
		resp.InvalidityReason = ""
	}

	return resp, nil
}
