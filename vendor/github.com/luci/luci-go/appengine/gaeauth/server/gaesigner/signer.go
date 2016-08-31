// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gaesigner implements signing.Signer interface using GAE App Identity
// API.
package gaesigner

import (
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/server/auth/signing"
)

// Signer implements signing.Signer using GAE App Identity API.
type Signer struct{}

// SignBytes signs the blob with some active private key.
//
// Returns the signature and name of the key used.
func (Signer) SignBytes(c context.Context, blob []byte) (keyName string, signature []byte, err error) {
	return info.Get(c).SignBytes(blob)
}

// Certificates returns a bundle with public certificates for all active keys.
func (Signer) Certificates(c context.Context) (*signing.PublicCertificates, error) {
	certs, err := cachedCerts(c)
	if err != nil {
		return nil, err
	}
	return certs.(*signing.PublicCertificates), nil
}

// ServiceInfo returns information about the current service.
//
// It includes app ID and the service account name (that ultimately owns the
// signing private key).
func (Signer) ServiceInfo(c context.Context) (*signing.ServiceInfo, error) {
	inf, err := cachedInfo(c)
	if err != nil {
		return nil, err
	}
	return inf.(*signing.ServiceInfo), nil
}

////

type cacheKey int

// cachedCerts caches this app certs in local memory for 1 hour.
var cachedCerts = proccache.Cached(cacheKey(0), func(c context.Context, key interface{}) (interface{}, time.Duration, error) {
	aeCerts, err := info.Get(c).PublicCertificates()
	if err != nil {
		return nil, 0, err
	}
	certs := make([]signing.Certificate, len(aeCerts))
	for i, ac := range aeCerts {
		certs[i] = signing.Certificate{
			KeyName:            ac.KeyName,
			X509CertificatePEM: string(ac.Data),
		}
	}
	return &signing.PublicCertificates{
		Certificates: certs,
		Timestamp:    signing.JSONTime(clock.Now(c)),
	}, time.Hour, nil
})

// cachedInfo caches this app service info in local memory forever.
//
// This info is static during lifetime of the process.
var cachedInfo = proccache.Cached(cacheKey(1), func(c context.Context, key interface{}) (interface{}, time.Duration, error) {
	i := info.Get(c)
	account, err := i.ServiceAccount()
	if err != nil {
		return nil, 0, err
	}
	return &signing.ServiceInfo{
		AppID:              i.AppID(),
		AppRuntime:         "go",
		AppRuntimeVersion:  runtime.Version(),
		AppVersion:         strings.Split(i.VersionID(), ".")[0],
		ServiceAccountName: account,
	}, 0, nil
})
