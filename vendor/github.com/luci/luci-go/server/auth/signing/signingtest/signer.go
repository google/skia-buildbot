// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package signingtest

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"math/rand"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/signing"
)

// Signer holds private key and corresponding cert and can sign blobs with
// PKCS1v15.
type Signer struct {
	priv        *rsa.PrivateKey
	certs       signing.PublicCertificates
	serviceInfo signing.ServiceInfo
}

var _ signing.Signer = (*Signer)(nil)

// NewSigner returns Signer instance deterministically deriving the key from
// the given seed. Panics on errors.
func NewSigner(seed int64, serviceInfo *signing.ServiceInfo) *Signer {
	src := notRandom{rand.New(rand.NewSource(seed))}

	// Generate deterministic key from the seed.
	priv, err := rsa.GenerateKey(src, 512)
	if err != nil {
		panic(err)
	}

	// Generate fake self signed certificate.
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Unix(1000000000, 0),
		NotAfter:              time.Unix(10000000000, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(src, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}

	// PEM-encode it, derive the name.
	pemOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyName := sha1.Sum(der)

	// Put in default service info if necessary.
	if serviceInfo == nil {
		serviceInfo = &signing.ServiceInfo{}
	}

	return &Signer{
		priv: priv,
		certs: signing.PublicCertificates{
			Certificates: []signing.Certificate{
				{
					KeyName:            hex.EncodeToString(keyName[:]),
					X509CertificatePEM: string(pemOut),
				},
			},
			Timestamp: signing.JSONTime(time.Unix(1000000000, 0)),
		},
		serviceInfo: *serviceInfo,
	}
}

// SignBytes signs the blob with some active private key.
// Returns the signature and name of the key used.
func (s *Signer) SignBytes(c context.Context, blob []byte) (keyName string, signature []byte, err error) {
	// Passing nil as 'rand' to SignPKCS1v15 disables RSA blinding, making
	// the signature deterministic, but reduces overall security. Do not do it
	// in non testing code.
	digest := sha256.Sum256(blob)
	sig, err := rsa.SignPKCS1v15(nil, s.priv, crypto.SHA256, digest[:])
	return s.certs.Certificates[0].KeyName, sig, err
}

// Certificates returns a bundle with public certificates for all active keys.
func (s *Signer) Certificates(c context.Context) (*signing.PublicCertificates, error) {
	return &s.certs, nil
}

// ServiceInfo returns information about the current service.
//
// It includes app ID and the service account name (that ultimately owns the
// signing private key).
func (s *Signer) ServiceInfo(c context.Context) (*signing.ServiceInfo, error) {
	return &s.serviceInfo, nil
}

////

type notRandom struct {
	*rand.Rand
}

func (r notRandom) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(r.Intn(256))
	}
	return len(p), nil
}
