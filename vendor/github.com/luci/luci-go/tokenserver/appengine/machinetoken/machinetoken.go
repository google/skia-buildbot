// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package machinetoken implements generation of LUCI machine tokens.
package machinetoken

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/server/auth/signing"

	"github.com/luci/luci-go/tokenserver/api"
	"github.com/luci/luci-go/tokenserver/api/admin/v1"
)

var maxUint64 = big.NewInt(0).SetUint64(math.MaxUint64)

// MintParams is passed to Mint.
type MintParams struct {
	// FQDN is a full name of a host to mint a token for.
	//
	// Must be in lowercase.
	FQDN string

	// Cert is the certificate used when authenticating the token requester.
	//
	// It's serial number will be put in the token.
	Cert *x509.Certificate

	// Config is a chunk of configuration related to the machine domain.
	//
	// It describes parameters for the token. Fetched from luci-config as part of
	// CA configuration.
	Config *admin.CertificateAuthorityConfig

	// SignerServiceAccount is GAE service account email of the token server.
	//
	// It will be put into the token (as issued_by field). Token consumers will
	// use it to fetch public keys from Google backends to verify the token
	// signature.
	SignerServiceAccount string

	// Signer produces RSA-SHA256 signatures using a token server key.
	//
	// Usually it is using SignBytes GAE API.
	Signer signing.Signer
}

// Validate checks that token minting parameters are allowed.
func (p *MintParams) Validate() error {
	// Check FDQN.
	if p.FQDN != strings.ToLower(p.FQDN) {
		return fmt.Errorf("expecting FQDN in lowercase, got %q", p.FQDN)
	}
	chunks := strings.SplitN(p.FQDN, ".", 2)
	if len(chunks) != 2 {
		return fmt.Errorf("not a valid FQDN %q", p.FQDN)
	}
	host, domain := chunks[0], chunks[1]
	if strings.ContainsRune(host, '@') {
		return fmt.Errorf("forbidden character '@' in hostname %q", host)
	}

	// Check DomainConfig for given domain.
	domainCfg := domainConfig(p.Config, domain)
	if domainCfg == nil {
		return fmt.Errorf("the domain %q is not whitelisted in the config", domain)
	}
	if domainCfg.MachineTokenLifetime <= 0 {
		return fmt.Errorf("machine tokens for machines in domain %q are not allowed", domain)
	}

	// Make sure cert serial number fits into uint64. We don't support negative or
	// giant SNs.
	sn := p.Cert.SerialNumber
	if sn.Sign() <= 0 || sn.Cmp(maxUint64) >= 0 {
		return fmt.Errorf("invalid certificate serial number: %s", sn)
	}
	return nil
}

// domainConfig returns DomainConfig for a domain.
//
// Returns nil if there's no such config.
func domainConfig(cfg *admin.CertificateAuthorityConfig, domain string) *admin.DomainConfig {
	for _, domainCfg := range cfg.KnownDomains {
		for _, domainInCfg := range domainCfg.Domain {
			if domainInCfg == domain {
				return domainCfg
			}
		}
	}
	return nil
}

// Mint generates a new machine token.
//
// Returns its body as a proto, and as a signed base64-encoded final token.
func Mint(c context.Context, params MintParams) (*tokenserver.MachineTokenBody, string, error) {
	if err := params.Validate(); err != nil {
		return nil, "", err
	}
	chunks := strings.SplitN(params.FQDN, ".", 2)
	if len(chunks) != 2 {
		panic("impossible") // checked in Validate already
	}
	cfg := domainConfig(params.Config, chunks[1])
	if cfg == nil {
		panic("impossible") // checked in Validate already
	}

	body := tokenserver.MachineTokenBody{
		MachineFqdn: params.FQDN,
		IssuedBy:    params.SignerServiceAccount,
		IssuedAt:    uint64(clock.Now(c).Unix()),
		Lifetime:    uint64(cfg.MachineTokenLifetime),
		CaId:        params.Config.UniqueId,
		CertSn:      params.Cert.SerialNumber.Uint64(), // already validated, fits uint64
	}
	serializedBody, err := proto.Marshal(&body)
	if err != nil {
		return nil, "", err
	}

	keyID, signature, err := params.Signer.SignBytes(c, serializedBody)
	if err != nil {
		return nil, "", errors.WrapTransient(err)
	}

	tokenBinBlob, err := proto.Marshal(&tokenserver.MachineTokenEnvelope{
		TokenBody: serializedBody,
		KeyId:     keyID,
		RsaSha256: signature,
	})
	if err != nil {
		return nil, "", err
	}
	return &body, base64.RawStdEncoding.EncodeToString(tokenBinBlob), nil
}

// Parse takes serialized MachineTokenEnvelope and parses it.
//
// It returns both deserialized envelope and deserialized MachineTokenBody. It
// does not check the signature or expiration.
//
// If the envelope can't be deserialized, returns (nil, nil, error).
// If MachineTokenBody can't be deserialized returns (envelope, nil, error).
func Parse(token string) (*tokenserver.MachineTokenEnvelope, *tokenserver.MachineTokenBody, error) {
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

// CheckSignature verifies the token was signed by some of the given keys.
func CheckSignature(envelope *tokenserver.MachineTokenEnvelope, certs *signing.PublicCertificates) error {
	return certs.CheckSignature(envelope.KeyId, envelope.TokenBody, envelope.RsaSha256)
}

// IsExpired returns true if the token is expired.
//
// Allows 10 sec clock drift.
func IsExpired(body *tokenserver.MachineTokenBody, now time.Time) bool {
	notBefore := time.Unix(int64(body.IssuedAt), 0)
	notAfter := notBefore.Add(time.Duration(body.Lifetime) * time.Second)
	return now.Before(notBefore.Add(-10*time.Second)) || now.After(notAfter.Add(10*time.Second))
}
