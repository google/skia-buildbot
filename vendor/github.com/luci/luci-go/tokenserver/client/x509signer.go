// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package client

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/luci/luci-go/common/data/rand/cryptorand"

	"golang.org/x/net/context"
)

// TODO(vadimsh): Adding support for ECDSA should be trivial if needed.

// X509Signer implements Signer interface by using a private key and
// a certificate specified in ANS.1 x509 PEM encoded structures.
//
// It is fine to initialize this struct directly if you have loaded private key
// and certificate already. You can optionally use Validate() to make sure they
// are valid before making other calls (all calls do validation anyhow).
//
// Use LoadX509Signer to load the key and certificate from files on disk.
type X509Signer struct {
	// PrivateKeyPEM is PEM-encoded ASN.1 PKCS#1 private key.
	//
	// See https://openssl.org/docs/manmaster/apps/rsa.html.
	PrivateKeyPEM []byte

	// CertificatePEM is PEM-encoded ASN.1 x509 certificate.
	//
	// It must contain a public key matching the private key specified by
	// PrivateKeyPEM (this will be verified).
	//
	// See https://openssl.org/docs/manmaster/apps/x509.html.
	CertificatePEM []byte

	// Fields below are lazy initialized on first use in Validate().

	init    sync.Once
	err     error
	algo    x509.SignatureAlgorithm
	certDer []byte
	pkey    crypto.Signer
}

// LoadX509Signer parses and validates private key and certificate PEM files.
//
// Returns X509Signer that is ready for work.
func LoadX509Signer(privateKeyPath, certPath string) (*X509Signer, error) {
	pkey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	cert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	signer := &X509Signer{
		PrivateKeyPEM:  pkey,
		CertificatePEM: cert,
	}
	if err = signer.Validate(); err != nil {
		return nil, err
	}
	return signer, nil
}

// Algo returns an algorithm that the signer implements.
func (s *X509Signer) Algo(ctx context.Context) (x509.SignatureAlgorithm, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	return s.algo, nil
}

// Certificate returns ASN.1 DER blob with the certificate of the signer.
func (s *X509Signer) Certificate(ctx context.Context) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s.certDer, nil
}

// Sign signs a blob using the private key.
func (s *X509Signer) Sign(ctx context.Context, blob []byte) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	var hashFunc crypto.Hash
	switch s.algo {
	case x509.SHA256WithRSA:
		hashFunc = crypto.SHA256
	default:
		panic("someone forgot to implement hashing algo for new kind of a key")
	}

	h := hashFunc.New()
	h.Write(blob)
	digest := h.Sum(nil)

	return s.pkey.Sign(cryptorand.Get(ctx), digest, hashFunc)
}

// Validate parses the private key and certificate file and verifies them.
//
// It checks that the public portion of the key matches what's in the
// certificate.
func (s *X509Signer) Validate() error {
	s.init.Do(func() {
		s.err = s.initialize()
	})
	return s.err
}

func (s *X509Signer) initialize() error {
	pkey, err := parsePrivateKey(s.PrivateKeyPEM)
	if err != nil {
		return err
	}
	cert, certDer, err := parseCertificate(s.CertificatePEM)
	if err != nil {
		return err
	}

	// Make sure the private key matches the public key in the cert. Also pick
	// the corresponding signing algorithm based on the type of the key.
	var algo x509.SignatureAlgorithm
	switch key := pkey.(type) {
	case *rsa.PrivateKey:
		var pub *rsa.PublicKey
		if pub, _ = cert.PublicKey.(*rsa.PublicKey); pub == nil {
			return fmt.Errorf("the certificate doesn't match the private key - wrong types")
		}
		if key.PublicKey.E != pub.E || key.PublicKey.N.Cmp(pub.N) != 0 {
			return fmt.Errorf("the certificate doesn't match the private key")
		}
		algo = x509.SHA256WithRSA
	default:
		panic("someone forgot to implement public key check for new kind of a key")
	}

	s.algo = algo
	s.certDer = certDer
	s.pkey = pkey
	return nil
}

func parsePrivateKey(pemData []byte) (crypto.Signer, error) {
	block, rest := pem.Decode(pemData)
	if len(rest) != 0 || block == nil {
		return nil, fmt.Errorf("not a valid private key PEM")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("not a supported private key type - %q", block.Type)
	}
}

func parseCertificate(pemData []byte) (*x509.Certificate, []byte, error) {
	block, rest := pem.Decode(pemData)
	if len(rest) != 0 || block == nil {
		return nil, nil, fmt.Errorf("not a valid certificate PEM")
	}
	if block.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("expecting \"CERTIFICATE\" PEM, got %q", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return cert, block.Bytes, err
}
