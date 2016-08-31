// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package certauthorities

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth"

	"github.com/luci/luci-go/tokenserver/api/admin/v1"

	"github.com/luci/luci-go/tokenserver/appengine/model"
)

// List of OAuth scopes to use for token sent to CRL endpoint.
var crlFetchScopes = []string{
	"https://www.googleapis.com/auth/userinfo.email",
}

// fetchCRL fetches a blob with der-encoded CRL from the CRL endpoint.
//
// It knows how to use ETag headers to avoid fetching already known data.
// May return transient and fatal errors.
func fetchCRL(c context.Context, cfg *admin.CertificateAuthorityConfig, knownETag string) (blob []byte, etag string, err error) {
	// Pick auth or non-auth transport.
	var transport http.RoundTripper
	if cfg.UseOauth {
		transport, err = auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(crlFetchScopes...))
	} else {
		transport, err = auth.GetRPCTransport(c, auth.NoAuth)
	}
	if err != nil {
		return nil, "", err
	}

	// Send the request with ETag related headers.
	req, err := http.NewRequest("GET", cfg.CrlUrl, nil)
	if err != nil {
		return nil, "", err
	}
	if knownETag != "" {
		req.Header.Set("If-None-Match", knownETag)
	}
	cl := http.Client{Transport: transport}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, "", errors.WrapTransient(err)
	}
	defer resp.Body.Close()

	// Nothing new?
	if resp.StatusCode == http.StatusNotModified {
		if knownETag == "" {
			return nil, "", errors.New("unexpected 304 status, no etag header was sent")
		}
		return nil, knownETag, nil
	}

	// Read the body in its entirety.
	blob, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", errors.WrapTransient(err)
	}

	// Transient error?
	if resp.StatusCode >= http.StatusInternalServerError {
		logging.Warningf(c, "GET %s - HTTP %d; %q", cfg.CrlUrl, resp.StatusCode, string(blob))
		return nil, "", errors.WrapTransient(fmt.Errorf("server replied with HTTP %d", resp.StatusCode))
	}

	// Something we don't support or expect?
	if resp.StatusCode != http.StatusOK {
		logging.Errorf(c, "GET %s - HTTP %d; %q", cfg.CrlUrl, resp.StatusCode, string(blob))
		return nil, "", fmt.Errorf("unexpected status HTTP %d", resp.StatusCode)
	}

	// Good enough.
	return blob, resp.Header.Get("ETag"), nil
}

// validateAndStoreCRL handles incoming CRL blob fetched by 'fetchCRL'.
func validateAndStoreCRL(c context.Context, crlDer []byte, etag string, ca *model.CA, prev *model.CRL) (*model.CRL, error) {
	// Make sure it is signed by the CA.
	caCert, err := x509.ParseCertificate(ca.Cert)
	if err != nil {
		return nil, fmt.Errorf("cert in the datastore is broken - %s", err)
	}
	crl, err := x509.ParseDERCRL(crlDer)
	if err != nil {
		return nil, fmt.Errorf("not a valid x509 CRL - %s", err)
	}
	if err = caCert.CheckCRLSignature(crl); err != nil {
		return nil, fmt.Errorf("CRL is not signed by the CA - %s", err)
	}

	// The CRL is peachy. Update a sharded set of all revoked certs.
	logging.Infof(c, "CRL last updated %s", crl.TBSCertList.ThisUpdate)
	logging.Infof(c, "Found %d entries in the CRL", len(crl.TBSCertList.RevokedCertificates))
	if err = model.UpdateCRLSet(c, ca.CN, model.CRLShardCount, crl); err != nil {
		return nil, err
	}
	logging.Infof(c, "All CRL entries stored")

	// Update the CRL entity. Use EntityVersion to make sure we are not
	// overwriting someone else's changes.
	var updated *model.CRL
	err = datastore.Get(c).RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		entity := *prev
		if err := ds.Get(&entity); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if entity.EntityVersion != prev.EntityVersion {
			return fmt.Errorf("CRL for %q was updated concurrently while we were working on it", ca.CN)
		}
		entity.EntityVersion++
		entity.LastUpdateTime = crl.TBSCertList.ThisUpdate.UTC()
		entity.LastFetchTime = clock.Now(c).UTC()
		entity.LastFetchETag = etag
		entity.RevokedCertsCount = len(crl.TBSCertList.RevokedCertificates)

		updated = &entity // used outside of this function
		toPut := []interface{}{updated}

		// Mark CA entity as ready for usage.
		curCA := model.CA{CN: ca.CN}
		switch err := ds.Get(&curCA); {
		case err == datastore.ErrNoSuchEntity:
			return fmt.Errorf("CA entity for %q is unexpectedly gone", ca.CN)
		case err != nil:
			return err
		}
		if !curCA.Ready {
			logging.Infof(c, "CA %q is ready now", curCA.CN)
			curCA.Ready = true
			toPut = append(toPut, &curCA)
		}
		return ds.Put(toPut)
	}, nil)
	if err != nil {
		return nil, errors.WrapTransient(err)
	}

	logging.Infof(c, "CRL for %q is updated, entity version is %d", ca.CN, updated.EntityVersion)
	return updated, nil
}
