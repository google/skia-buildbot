// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package signing

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/server/auth/internal"
)

// CertsCacheExpiration defines how long to cache fetched certificates in local
// memory.
const CertsCacheExpiration = 15 * time.Minute

// Certificate is public certificate of some service. Must not be mutated once
// initialized.
type Certificate struct {
	// KeyName identifies the key used for signing.
	KeyName string `json:"key_name"`
	// X509CertificatePEM is PEM encoded certificate.
	X509CertificatePEM string `json:"x509_certificate_pem"`
}

// PublicCertificates is a bundle of recent certificates of some service. Must
// not be mutated once initialized.
type PublicCertificates struct {
	// Certificates is the list of certificates.
	Certificates []Certificate `json:"certificates"`
	// Timestamp is Unix time (microseconds) of when this list was generated.
	Timestamp JSONTime `json:"timestamp"`

	lock  sync.RWMutex
	cache map[string]*x509.Certificate
}

// JSONTime is time.Time that serializes as unix timestamp (in microseconds).
type JSONTime time.Time

// Time casts value to time.Time.
func (t JSONTime) Time() time.Time {
	return time.Time(t)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *JSONTime) UnmarshalJSON(data []byte) error {
	ts, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*t = JSONTime(time.Unix(0, ts*1000))
	return nil
}

// MarshalJSON implements json.Marshaler.
func (t JSONTime) MarshalJSON() ([]byte, error) {
	ts := t.Time().UnixNano() / 1000
	return []byte(strconv.FormatInt(ts, 10)), nil
}

type proccacheKey string

// FetchCertificates fetches certificates from the given URL.
//
// The server is expected to reply with JSON described by PublicCertificates
// struct (like LUCI services do). Uses proccache to cache them for
// CertsCacheExpiration minutes.
//
// LUCI services serve certificates at /auth/api/v1/server/certificates.
func FetchCertificates(c context.Context, url string) (*PublicCertificates, error) {
	certs, err := proccache.GetOrMake(c, proccacheKey("url:"+url), func() (interface{}, time.Duration, error) {
		certs := &PublicCertificates{}
		req := internal.Request{
			Method: "GET",
			URL:    url,
			Out:    certs,
		}
		if err := req.Do(c); err != nil {
			return nil, 0, err
		}
		return certs, CertsCacheExpiration, nil
	})
	if err != nil {
		return nil, err
	}
	return certs.(*PublicCertificates), nil
}

// FetchCertificatesFromLUCIService is shortcut for FetchCertificates
// that uses LUCI-specific endpoint.
//
// 'serviceURL' is root URL of the service (e.g. 'https://example.com').
func FetchCertificatesFromLUCIService(c context.Context, serviceURL string) (*PublicCertificates, error) {
	return FetchCertificates(c, serviceURL+"/auth/api/v1/server/certificates")
}

type robotCertURLKey int

// robotCertURL is used to mock URL of a google backend in unit tests.
func robotCertURL(c context.Context) string {
	if url, ok := c.Value(robotCertURLKey(0)).(string); ok {
		return url
	}
	return "https://www.googleapis.com/robot/v1/metadata/x509/"
}

// FetchCertificatesForServiceAccount fetches certificates of some Google
// service account.
//
// Works only with Google service accounts (@*.gserviceaccount.com). Uses
// proccache to cache them for CertsCacheExpiration minutes.
//
// Usage (roughly):
//
//   certs, err := signing.FetchCertificatesForServiceAccount(ctx, <email>)
//   if certs.CheckSignature(<key id>, <blob>, <signature>) == nil {
//     <signature is valid!>
//   }
func FetchCertificatesForServiceAccount(c context.Context, email string) (*PublicCertificates, error) {
	// Do only basic validation and offload full validation to the google backend.
	if !strings.HasSuffix(email, ".gserviceaccount.com") {
		return nil, fmt.Errorf("signature: not a google service account %q", email)
	}

	certs, err := proccache.GetOrMake(c, proccacheKey("email:"+email), func() (interface{}, time.Duration, error) {
		// Ask Google backend for a dict "key id => x509 PEM encoded cert".
		keysAndCerts := map[string]string{}
		req := internal.Request{
			Method: "GET",
			URL:    robotCertURL(c) + url.QueryEscape(email),
			Out:    &keysAndCerts,
		}
		if err := req.Do(c); err != nil {
			return nil, 0, err
		}

		// Sort by key for reproducibility of return values.
		keys := make([]string, 0, len(keysAndCerts))
		for key := range keysAndCerts {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Convert to PublicCertificates struct.
		certs := &PublicCertificates{}
		for _, key := range keys {
			certs.Certificates = append(certs.Certificates, Certificate{
				KeyName:            key,
				X509CertificatePEM: keysAndCerts[key],
			})
		}
		return certs, CertsCacheExpiration, nil
	})
	if err != nil {
		return nil, err
	}
	return certs.(*PublicCertificates), nil
}

// CertificateForKey finds the certificate for given key and deserializes it.
func (pc *PublicCertificates) CertificateForKey(key string) (*x509.Certificate, error) {
	// Use fast reader lock first.
	pc.lock.RLock()
	cert, ok := pc.cache[key]
	pc.lock.RUnlock()
	if ok {
		return cert, nil
	}

	// Grab the write lock and recheck the cache.
	pc.lock.Lock()
	defer pc.lock.Unlock()
	if cert, ok := pc.cache[key]; ok {
		return cert, nil
	}

	for _, cert := range pc.Certificates {
		if cert.KeyName == key {
			block, _ := pem.Decode([]byte(cert.X509CertificatePEM))
			if block == nil {
				return nil, fmt.Errorf("signature: the certificate %q is not PEM encoded", key)
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			if pc.cache == nil {
				pc.cache = make(map[string]*x509.Certificate)
			}
			pc.cache[key] = cert
			return cert, nil
		}
	}

	return nil, fmt.Errorf("signature: no such certificate %q", key)
}

// CheckSignature returns nil if `signed` was indeed signed by given key.
func (pc *PublicCertificates) CheckSignature(key string, signed, signature []byte) error {
	cert, err := pc.CertificateForKey(key)
	if err != nil {
		return err
	}
	return cert.CheckSignature(x509.SHA256WithRSA, signed, signature)
}
