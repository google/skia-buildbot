// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package serviceaccounts implements ServiceAccounts API.
//
// Code defined here is either invoked by an administrator or by the service
// itself (from implementation of other services).
package serviceaccounts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/jws"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/server/auth"

	"github.com/luci/luci-go/tokenserver/api"
	"github.com/luci/luci-go/tokenserver/api/admin/v1"
	"github.com/luci/luci-go/tokenserver/api/minter/v1"

	"github.com/luci/luci-go/tokenserver/appengine/certchecker"
	"github.com/luci/luci-go/tokenserver/appengine/model"
)

// accountIDRegexp is regular expression for allowed service account emails.
var accountIDRegexp = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])$`)

var (
	googleTokenURL = "https://www.googleapis.com/oauth2/v4/token"
	jwtGrantType   = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	jwtHeader      = &jws.Header{Algorithm: "RS256", Typ: "JWT"}
)

// Server implements admin.ServiceAccountsServer RPC interface.
//
// It assumes authorization has happened already.
type Server struct {
	transport       http.RoundTripper // used in unit tests to mock OAuth
	iamBackendURL   string            // used in unit tests to mock IAM API
	tokenBackendURL string            // used in unit tests to mock OAuth token endpoint
	ownEmail        string            // used in unit tests to mock GAE Info API
}

// CreateServiceAccount creates Google Cloud IAM service account associated
// with given CN.
func (s *Server) CreateServiceAccount(c context.Context, r *admin.CreateServiceAccountRequest) (*admin.CreateServiceAccountResponse, error) {
	// Validate FQDN and load proper config.
	accountID, domain, err := validateFQDN(r.Fqdn)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "%s", err)
	}
	cfg, err := s.getCAConfig(c, r.Ca)
	if err != nil {
		return nil, err
	}
	domainCfg, err := domainConfig(cfg, domain)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "%s", err)
	}

	// Create the service account.
	account, err := s.ensureServiceAccount(c, serviceAccountParams{
		Project:   domainCfg.CloudProjectName,
		AccountID: accountID,
		FQDN:      r.Fqdn,
		Force:     r.Force,
	})
	if err != nil {
		return nil, err
	}
	return &admin.CreateServiceAccountResponse{
		ServiceAccount: account,
	}, nil
}

// serviceAccountParams is passed to ensureServiceAccount.
type serviceAccountParams struct {
	Project   string // cloud project name to create account in
	AccountID string // account ID, the email will be <accountID>@...
	FQDN      string // FQDN of a host to create an account for
	Force     bool   // true to skip datastore check and call IAM API no matter what
}

// ensureServiceAccount does the actual job of CreateServiceAccount.
//
// Assumes 'params' is validated already. Returns grpc.Errorf on errors.
func (s *Server) ensureServiceAccount(c context.Context, params serviceAccountParams) (*tokenserver.ServiceAccount, error) {
	// Normalize fqdn for comparison.
	fqdn := strings.ToLower(params.FQDN)

	// Checks the datastore, perhaps we already registered it.
	if !params.Force {
		account, err := fetchAccountInfo(c, params.Project, params.AccountID)
		if err != nil {
			return nil, grpc.Errorf(codes.Internal, "transient error when fetching account info from datastore - %s", err)
		}
		if account != nil {
			// Two FQDN's happen to have identical hostname? This is a big deal and
			// generally not supported.
			if account.Fqdn != fqdn {
				return nil, grpc.Errorf(codes.FailedPrecondition, "the account is already assigned to %q", account.Fqdn)
			}
			return account, nil
		}
	}

	// Account display name must be < 100 bytes. We don't use it for anything
	// very important, so just trim.
	displayName := fqdn
	if len(fqdn) >= 100 {
		displayName = fqdn[:100]
	}

	// Attempt to create. May fail with http.StatusConflict if already exists.
	expectedEmail := serviceAccountEmail(params.Project, params.AccountID)
	logging.Infof(c, "Creating account %q", expectedEmail)
	account, err := s.iamCreateAccount(c, params.Project, params.AccountID, displayName)

	switch apiErr, _ := err.(*googleapi.Error); {
	case err == nil:
		logging.Infof(c, "Account %q created", expectedEmail)
		if account.Email != expectedEmail {
			// Log the error, but proceed anyway, since the expectedEmail is
			// important only when dealing with rare race conditions. Returning error
			// here would block the token server operations completely in case Google
			// decides to unexpectedly change service account email format.
			logging.Errorf(
				c, "Real account email %q doesn't match expected one %q",
				account.Email, expectedEmail)
		}

	case apiErr != nil && apiErr.Code == http.StatusConflict:
		logging.Warningf(c, "Account %q already exists, fetching its details", expectedEmail)
		account, err = s.iamGetAccount(c, params.Project, expectedEmail)
		if err != nil {
			return nil, grpc.Errorf(
				codes.Internal, "unexpected error when fetching account details for %q - %s",
				expectedEmail, err)
		}
		// In an unlikely event of a race condition when creating two accounts with
		// identical hostnames but different FQDNs, the display names here will not
		// match. Ask the client to retry (to read account state from the datastore
		// and fail properly).
		if account.DisplayName != displayName {
			return nil, grpc.Errorf(
				codes.Internal, "expected account %q to have display name %q, but it is %q",
				expectedEmail, displayName, account.DisplayName)
		}

	case err != nil:
		return nil, grpc.Errorf(codes.Internal, "cloud IAM call to create a service account failed - %s", err)
	}

	// Store the account state in the datastore to avoid redoing all this again.
	result, err := storeAccountInfo(c, params.Project, params.AccountID, fqdn, account)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "failed to save account info - %s", err)
	}
	return result, nil
}

// MintAccessTokenParams is passed to DoMintAccessToken.
type MintAccessTokenParams struct {
	Config *admin.CertificateAuthorityConfig // domain configs, scopes whitelist
	FQDN   string                            // FQDN of a host to mint a token for
	Scopes []string                          // OAuth2 scopes to grant the token
}

// Validate returns nil if the parameters are valid.
//
// It checks FQDN matches some known domain, account ID matches the regexp, and
// requests scopes are all whitelisted.
func (p *MintAccessTokenParams) Validate() error {
	_, domain, err := validateFQDN(p.FQDN)
	if err != nil {
		return err
	}
	domainCfg, err := domainConfig(p.Config, domain)
	if err != nil {
		return err
	}
	return validateOAuthScopes(domainCfg, p.Scopes)
}

// DoMintAccessToken makes an OAuth access token for the given service account.
//
// Return grpc.Error on errors.
func (s *Server) DoMintAccessToken(c context.Context, params MintAccessTokenParams) (*tokenserver.ServiceAccount, *minter.OAuth2AccessToken, error) {
	if err := params.Validate(); err != nil {
		return nil, nil, grpc.Errorf(codes.InvalidArgument, "%s", err)
	}

	// Load proper domain config.
	accountID, domain, err := validateFQDN(params.FQDN)
	if err != nil {
		panic("impossible") // params where validated already
	}
	domainCfg, err := domainConfig(params.Config, domain)
	if err != nil {
		panic("impossible") // params where validated already
	}

	// Create the corresponding service account if necessary.
	account, err := s.ensureServiceAccount(c, serviceAccountParams{
		Project:   domainCfg.CloudProjectName,
		AccountID: accountID,
		FQDN:      params.FQDN,
	})
	if err != nil {
		return nil, nil, err
	}

	// Mint the access token.
	//
	// See https://developers.google.com/identity/protocols/OAuth2ServiceAccount#authorizingrequests
	// Also https://github.com/golang/oauth2/blob/master/jwt/jwt.go.

	// Prepare a claim set to be signed by the service account key. Note that
	// Google backend seems to ignore Exp field and always gives one-hour long
	// tokens.
	//
	// Also revert time back for machines whose time is not perfectly in sync.
	// If client machine's time is in the future according to Google servers, an
	// access token will not be issued.
	now := clock.Now(c).Add(-10 * time.Second)
	claimSet := &jws.ClaimSet{
		Iat:   now.Unix(),
		Exp:   now.Add(time.Hour).Unix(),
		Iss:   account.Email,
		Scope: strings.Join(params.Scopes, " "),
		Aud:   googleTokenURL,
	}

	// Sign it, thus obtaining so called 'assertion'. Note that with Google gRPC
	// endpoints, an assertion by itself can be used as an access token (with few
	// small changes). It doesn't work for GAE though.
	assertion, err := jws.EncodeWithSigner(
		jwtHeader, claimSet, s.iamSigner(c, account.ProjectId, account.Email))
	if err != nil {
		return nil, nil, grpc.Errorf(codes.Internal, "error when signing JWT - %s", err)
	}

	// Exchange the assertion for the access token.
	token, err := s.fetchAccessToken(c, assertion)
	if err != nil {
		return nil, nil, grpc.Errorf(codes.Internal, "error when calling token endpoint - %s", err)
	}
	return account, token, nil
}

// fetchAccessToken does the final part of OAuth 2-legged flow.
func (s *Server) fetchAccessToken(c context.Context, assertion string) (*minter.OAuth2AccessToken, error) {
	// The client without auth, the token endpoint doesn't need auth.
	client, err := s.httpClient(c, false)
	if err != nil {
		return nil, err
	}
	tokenURL := s.tokenBackendURL
	if tokenURL == "" {
		tokenURL = googleTokenURL
	}

	// This is mostly copy-pasta from https://github.com/golang/oauth2/blob/master/jwt/jwt.go.
	v := url.Values{}
	v.Set("grant_type", jwtGrantType)
	v.Set("assertion", assertion)
	resp, err := client.PostForm(tokenURL, v)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %s", err)
	}
	if c := resp.StatusCode; c < 200 || c > 299 {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %s\nResponse: %s", resp.Status, body)
	}
	var tokenRes struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		IDToken     string `json:"id_token"`
		ExpiresIn   int64  `json:"expires_in"` // relative seconds from now
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %s", err)
	}
	// end of copy-pasta.

	// The Google endpoint always returns 'expires_in'.
	if tokenRes.ExpiresIn == 0 {
		return nil, fmt.Errorf("invalid token endpoint response, 'expires_in' is not set")
	}
	expiry := clock.Now(c).Add(time.Duration(tokenRes.ExpiresIn) * time.Second)
	return &minter.OAuth2AccessToken{
		AccessToken: tokenRes.AccessToken,
		TokenType:   tokenRes.TokenType,
		Expiry:      google.NewTimestamp(expiry),
	}, nil
}

// getCAConfig returns CertificateAuthorityConfig for a CA.
//
// Returns grpc.Error on errors.
func (s *Server) getCAConfig(c context.Context, ca string) (*admin.CertificateAuthorityConfig, error) {
	var entity *model.CA

	checker, err := certchecker.GetCertChecker(c, ca)
	if err != nil {
		goto fail
	}
	entity, err = checker.GetCA(c)
	if err != nil {
		goto fail
	}
	return entity.ParsedConfig, nil

fail:
	if errors.IsTransient(err) {
		return nil, grpc.Errorf(codes.Internal, "transient error when fetching CA config - %s", err)
	}
	return nil, grpc.Errorf(codes.Unknown, "error when fetching CA config - %s", err)
}

// httpClient returns a HTTP client with deadline set to 20 sec.
//
// Note that on GAE the only way to set a deadline for HTTP request is to grab
// a new transport based on a context with deadline.
//
// context/ctxhttp does not work, since it attempts to cancel in-flight requests
// on timeout and GAE ignores that.
func (s *Server) httpClient(c context.Context, withAuth bool) (*http.Client, error) {
	transport := s.transport
	if transport != nil {
		return &http.Client{Transport: transport}, nil
	}
	c, _ = clock.WithTimeout(c, 20*time.Second)
	var err error
	if withAuth {
		transport, err = auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(iam.CloudPlatformScope))
	} else {
		transport, err = auth.GetRPCTransport(c, auth.NoAuth)
	}
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}

// iamService returns configured *iam.Service to talk to Service Accounts API.
func (s *Server) iamService(c context.Context) (*iam.Service, error) {
	client, err := s.httpClient(c, true)
	if err != nil {
		return nil, err
	}
	service, err := iam.New(client)
	if err != nil {
		return nil, err
	}
	if s.iamBackendURL != "" {
		service.BasePath = s.iamBackendURL
	}
	return service, nil
}

// iamCreateAccount wraps ServiceAccounts.Create IAM API call.
//
// Requires at least 'Editor' role in the cloud project.
func (s *Server) iamCreateAccount(c context.Context, project, accountID, displayName string) (*iam.ServiceAccount, error) {
	service, err := s.iamService(c)
	if err != nil {
		return nil, err
	}
	call := service.Projects.ServiceAccounts.Create(
		"projects/"+project,
		&iam.CreateServiceAccountRequest{
			AccountId: accountID,
			ServiceAccount: &iam.ServiceAccount{
				DisplayName: displayName,
			},
		})
	return call.Context(c).Do()
}

// iamGetAccount wraps ServiceAccounts.Get IAM API call.
func (s *Server) iamGetAccount(c context.Context, project, email string) (*iam.ServiceAccount, error) {
	service, err := s.iamService(c)
	if err != nil {
		return nil, err
	}
	call := service.Projects.ServiceAccounts.Get(serviceAccountResource(project, email))
	return call.Context(c).Do()
}

// iamSigner returns a function that can RSA sign blobs using service account
// key of given service account.
//
// The token server's own service account should have 'serviceAccountActor' role
// in the service account resource or in the project that owns the service
// account.
func (s *Server) iamSigner(c context.Context, project, email string) func([]byte) ([]byte, error) {
	return func(data []byte) ([]byte, error) {
		service, err := s.iamService(c)
		if err != nil {
			return nil, err
		}
		call := service.Projects.ServiceAccounts.SignBlob(
			serviceAccountResource(project, email),
			&iam.SignBlobRequest{
				BytesToSign: base64.StdEncoding.EncodeToString(data),
			})
		resp, err := call.Context(c).Do()
		if err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(resp.Signature)
	}
}

////////////////////////////////////////////////////////////////////////////////

// validateFQDN splits FQDN into hostname and domain name, validating them.
func validateFQDN(fqdn string) (host, domain string, err error) {
	fqdn = strings.ToLower(fqdn)
	chunks := strings.SplitN(fqdn, ".", 2)
	if len(chunks) != 2 {
		return "", "", fmt.Errorf("not a valid FQDN %q", fqdn)
	}
	// Host name is used as service account ID. Validate it.
	// See https://cloud.google.com/iam/reference/rest/v1/projects.serviceAccounts/create.
	// The account ID is also limited in length to 6-30 chars (this is not
	// documented though).
	host, domain = chunks[0], chunks[1]
	if len(host) < 6 || len(host) >= 30 || !accountIDRegexp.MatchString(host) {
		return "", "", fmt.Errorf(
			"the hostname (%q) must match %q and be 6-30 characters long, it doesn't",
			host, accountIDRegexp.String())
	}
	return host, domain, nil
}

// validateOAuthScopes checks the scopes are in the whitelist.
func validateOAuthScopes(cfg *admin.DomainConfig, scopes []string) error {
	if len(scopes) == 0 {
		return fmt.Errorf("at least one OAuth2 scope must be specified")
	}
	for _, scope := range scopes {
		ok := false
		for _, wl := range cfg.AllowedOauth2Scope {
			if scope == wl {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("OAuth2 scope %q is not whitelisted in the config", scope)
		}
	}
	return nil
}

// domainConfig returns DomainConfig for a domain.
//
// Returns grpc.Error when there's no such config.
func domainConfig(cfg *admin.CertificateAuthorityConfig, domain string) (*admin.DomainConfig, error) {
	for _, domainCfg := range cfg.KnownDomains {
		for _, domainInCfg := range domainCfg.Domain {
			if domainInCfg == domain {
				return domainCfg, nil
			}
		}
	}
	return nil, fmt.Errorf("the domain %q is not whitelisted in the config", domain)
}

// serviceAccountEmail returns expected email address of a service account.
//
// Unfortunately we had to hardcode this rule here to properly handle race
// conditions when creating an account: /v1/projects.serviceAccounts/get API
// accepts emails, not short account IDs we pass to 'create' API call. So if
// 'create' call fails with "Already exists" error, we had to guess an email to
// fetch details of the existing account.
func serviceAccountEmail(project, accountID string) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", accountID, project)
}

// serviceAccountResource returns full resource name of a service accounts.
func serviceAccountResource(project, email string) string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s", project, email)
}

// storeAccountInfo puts details about a service account in the datastore.
//
// Returns the resulting tokenserver.ServiceAccount object.
func storeAccountInfo(c context.Context, project, accountID, fqdn string, acc *iam.ServiceAccount) (*tokenserver.ServiceAccount, error) {
	entity := model.ServiceAccount{
		ID:             project + "/" + accountID,
		ProjectID:      project,
		UniqueID:       acc.UniqueId,
		Email:          acc.Email,
		DisplayName:    acc.DisplayName,
		OAuth2ClientID: acc.Oauth2ClientId,
		FQDN:           fqdn,
		Registered:     clock.Now(c).UTC(),
	}
	if err := datastore.Get(c).Put(&entity); err != nil {
		return nil, err
	}
	return entity.GetProto(), nil
}

// fetchAccountInfo returns service account details stored in the datastore.
//
// Returns (nil, nil) if no such service account was registered yet.
func fetchAccountInfo(c context.Context, project, accountID string) (*tokenserver.ServiceAccount, error) {
	entity := model.ServiceAccount{
		ID: project + "/" + accountID,
	}
	switch err := datastore.Get(c).Get(&entity); {
	case err == datastore.ErrNoSuchEntity:
		return nil, nil
	case err != nil:
		return nil, errors.WrapTransient(err)
	}
	return entity.GetProto(), nil
}
