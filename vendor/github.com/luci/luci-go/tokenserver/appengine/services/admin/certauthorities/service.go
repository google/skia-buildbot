// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package certauthorities implements CertificateAuthorities API.
//
// Code defined here is either invoked by an administrator or by the service
// itself (via cron jobs or task queues).
package certauthorities

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"

	"github.com/luci/luci-go/tokenserver/api/admin/v1"

	"github.com/luci/luci-go/tokenserver/appengine/certchecker"
	"github.com/luci/luci-go/tokenserver/appengine/model"
	"github.com/luci/luci-go/tokenserver/appengine/utils"
)

// Server implements admin.CertificateAuthoritiesServer RPC interface.
//
// It assumes authorization has happened already.
type Server struct {
}

// ImportConfig makes the server read its config from luci-config right now.
//
// Note that regularly configs are read in background each 5 min. ImportConfig
// can be used to force config reread immediately. It will block until configs
// are read.
func (s *Server) ImportConfig(c context.Context, _ *google.Empty) (*admin.ImportConfigResponse, error) {
	cfg, err := fetchConfigFile(c, "tokenserver.cfg")
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't read config file - %s", err)
	}
	logging.Infof(c, "Importing config at rev %s", cfg.Revision)

	// Read list of CAs.
	msg := admin.TokenServerConfig{}
	if err = proto.UnmarshalText(cfg.Content, &msg); err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't parse config file - %s", err)
	}

	seenIDs, err := model.LoadCAUniqueIDToCNMap(c)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't load unique_id map - %s", err)
	}
	if seenIDs == nil {
		seenIDs = map[int64]string{}
	}
	seenIDsDirty := false

	// There should be no duplicates.
	seenCAs := stringset.New(len(msg.GetCertificateAuthority()))
	for _, ca := range msg.GetCertificateAuthority() {
		if seenCAs.Has(ca.Cn) {
			return nil, grpc.Errorf(codes.Internal, "duplicate entries in the config")
		}
		seenCAs.Add(ca.Cn)
		// Check unique ID is not being reused.
		if existing, seen := seenIDs[ca.UniqueId]; seen {
			if existing != ca.Cn {
				return nil, grpc.Errorf(
					codes.Internal, "duplicate unique_id %d in the config: %q and %q",
					ca.UniqueId, ca.Cn, existing)
			}
		} else {
			seenIDs[ca.UniqueId] = ca.Cn
			seenIDsDirty = true
		}
	}

	// Update the mapping CA unique_id -> CA CN. Unique integer ids are used in
	// various tokens in place of a full CN name to save space. This mapping is
	// additive (all new CAs should have different IDs).
	if seenIDsDirty {
		if err := model.StoreCAUniqueIDToCNMap(c, seenIDs); err != nil {
			return nil, grpc.Errorf(codes.Internal, "can't store unique_id map - %s", err)
		}
	}

	// Add new CA datastore entries or update existing ones.
	wg := sync.WaitGroup{}
	me := errors.NewLazyMultiError(len(msg.GetCertificateAuthority()))
	for i, ca := range msg.GetCertificateAuthority() {
		wg.Add(1)
		go func(i int, ca *admin.CertificateAuthorityConfig) {
			defer wg.Done()
			certFileCfg, err := fetchConfigFile(c, ca.CertPath)
			if err != nil {
				logging.Errorf(c, "Failed to fetch %q: %s", ca.CertPath, err)
				me.Assign(i, err)
			} else if err := s.importCA(c, ca, certFileCfg.Content, cfg.Revision); err != nil {
				logging.Errorf(c, "Failed to import %q: %s", ca.Cn, err)
				me.Assign(i, err)
			}
		}(i, ca)
	}
	wg.Wait()
	if err = me.Get(); err != nil {
		return nil, grpc.Errorf(codes.Internal, "can't import CA - %s", err)
	}

	// Find CAs that were removed from the config.
	toRemove := []string{}
	q := datastore.NewQuery("CA").Eq("Removed", false).KeysOnly(true)
	err = datastore.Get(c).Run(q, func(k *datastore.Key) {
		if !seenCAs.Has(k.StringID()) {
			toRemove = append(toRemove, k.StringID())
		}
	})
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "datastore error - %s", err)
	}

	// Mark them as inactive in the datastore.
	wg = sync.WaitGroup{}
	me = errors.NewLazyMultiError(len(toRemove))
	for i, name := range toRemove {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			if err := s.removeCA(c, name, cfg.Revision); err != nil {
				logging.Errorf(c, "Failed to remove %q: %s", name, err)
				me.Assign(i, err)
			}
		}(i, name)
	}
	wg.Wait()
	if err = me.Get(); err != nil {
		return nil, grpc.Errorf(codes.Internal, "datastore error - %s", err)
	}

	return &admin.ImportConfigResponse{
		Revision: cfg.Revision,
	}, nil
}

// FetchCRL makes the server fetch a CRL for some CA.
func (s *Server) FetchCRL(c context.Context, r *admin.FetchCRLRequest) (*admin.FetchCRLResponse, error) {
	ds := datastore.Get(c)

	// Grab a corresponding CA entity. It contains URL of CRL to fetch.
	ca := &model.CA{CN: r.Cn}
	switch err := ds.Get(ca); {
	case err == datastore.ErrNoSuchEntity:
		return nil, grpc.Errorf(codes.NotFound, "no such CA %q", ca.CN)
	case err != nil:
		return nil, grpc.Errorf(codes.Internal, "datastore error - %s", err)
	}

	// Grab CRL URL from the CA config.
	cfg, err := ca.ParseConfig()
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "broken CA config in the datastore - %s", err)
	}
	if cfg.CrlUrl == "" {
		return nil, grpc.Errorf(codes.NotFound, "CA %q doesn't have CRL defined", ca.CN)
	}

	// Grab info about last processed CRL, if any.
	crl := &model.CRL{Parent: ds.KeyForObj(ca)}
	if err = ds.Get(crl); err != nil && err != datastore.ErrNoSuchEntity {
		return nil, grpc.Errorf(codes.Internal, "datastore error - %s", err)
	}

	// Fetch latest CRL blob.
	logging.Infof(c, "Fetching CRL for %q from %s", ca.CN, cfg.CrlUrl)
	knownETag := crl.LastFetchETag
	if r.Force {
		knownETag = ""
	}
	fetchCtx, _ := clock.WithTimeout(c, time.Minute)
	crlDer, newEtag, err := fetchCRL(fetchCtx, cfg, knownETag)
	switch {
	case errors.IsTransient(err):
		return nil, grpc.Errorf(codes.Internal, "transient error when fetching CRL - %s", err)
	case err != nil:
		return nil, grpc.Errorf(codes.Unknown, "can't fetch CRL - %s", err)
	}

	// No changes?
	if knownETag != "" && knownETag == newEtag {
		logging.Infof(c, "No changes to CRL (etag is %s), skipping", knownETag)
	} else {
		logging.Infof(c, "Fetched CRL size is %d bytes, etag is %s", len(crlDer), newEtag)
		crl, err = validateAndStoreCRL(c, crlDer, newEtag, ca, crl)
		switch {
		case errors.IsTransient(err):
			return nil, grpc.Errorf(codes.Internal, "transient error when storing CRL - %s", err)
		case err != nil:
			return nil, grpc.Errorf(codes.Unknown, "bad CRL - %s", err)
		}
	}

	return &admin.FetchCRLResponse{CrlStatus: crl.GetStatusProto()}, nil
}

// ListCAs returns a list of Common Names of registered CAs.
func (s *Server) ListCAs(c context.Context, _ *google.Empty) (*admin.ListCAsResponse, error) {
	ds := datastore.Get(c)
	keys := []*datastore.Key{}

	q := datastore.NewQuery("CA").Eq("Removed", false).KeysOnly(true)
	if err := ds.GetAll(q, &keys); err != nil {
		return nil, grpc.Errorf(codes.Internal, "transient datastore error - %s", err)
	}

	resp := &admin.ListCAsResponse{
		Cn: make([]string, len(keys)),
	}
	for i, key := range keys {
		resp.Cn[i] = key.StringID()
	}
	return resp, nil
}

// GetCAStatus returns configuration of some CA defined in the config.
func (s *Server) GetCAStatus(c context.Context, r *admin.GetCAStatusRequest) (*admin.GetCAStatusResponse, error) {
	ds := datastore.Get(c)

	// Entities to fetch.
	ca := model.CA{CN: r.Cn}
	crl := model.CRL{Parent: ds.KeyForObj(&ca)}

	// Fetch them at the same revision. It is fine if CRL is not there yet. Don't
	// bother doing it in parallel: GetCAStatus is used only by admins, manually.
	err := ds.RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		if err := ds.Get(&ca); err != nil {
			return err // can be ErrNoSuchEntity
		}
		if err := ds.Get(&crl); err != nil && err != datastore.ErrNoSuchEntity {
			return err // only transient errors
		}
		return nil
	}, nil)
	switch {
	case err == datastore.ErrNoSuchEntity:
		return &admin.GetCAStatusResponse{}, nil
	case err != nil:
		return nil, grpc.Errorf(codes.Internal, "datastore error - %s", err)
	}

	cfgMsg, err := ca.ParseConfig()
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "broken config in the datastore - %s", err)
	}

	return &admin.GetCAStatusResponse{
		Config:     cfgMsg,
		Cert:       utils.DumpPEM(ca.Cert, "CERTIFICATE"),
		Removed:    ca.Removed,
		Ready:      ca.Ready,
		AddedRev:   ca.AddedRev,
		UpdatedRev: ca.UpdatedRev,
		RemovedRev: ca.RemovedRev,
		CrlStatus:  crl.GetStatusProto(),
	}, nil
}

// IsRevokedCert says whether a certificate serial number is in the CRL.
func (s *Server) IsRevokedCert(c context.Context, r *admin.IsRevokedCertRequest) (*admin.IsRevokedCertResponse, error) {
	sn := big.Int{}
	if _, ok := sn.SetString(r.Sn, 0); !ok {
		return nil, grpc.Errorf(codes.InvalidArgument, "can't parse 'sn'")
	}

	checker, err := certchecker.GetCertChecker(c, r.Ca)
	if err != nil {
		if details, ok := err.(certchecker.Error); ok && details.Reason == certchecker.NoSuchCA {
			return nil, grpc.Errorf(codes.NotFound, "no such CA: %q", r.Ca)
		}
		return nil, grpc.Errorf(codes.Internal, "failed to check %q CRL - %s", r.Ca, err)
	}

	revoked, err := checker.CRL.IsRevokedSN(c, &sn)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "failed to check %q CRL - %s", r.Ca, err)
	}

	return &admin.IsRevokedCertResponse{Revoked: revoked}, nil
}

// CheckCertificate says whether a certificate is valid or not.
func (s *Server) CheckCertificate(c context.Context, r *admin.CheckCertificateRequest) (*admin.CheckCertificateResponse, error) {
	// Deserialize the cert.
	der, err := utils.ParsePEM(r.CertPem, "CERTIFICATE")
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "can't parse 'cert_pem' - %s", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "can't parse 'cert-pem' - %s", err)
	}

	// Find a checker for the CA that signed the cert, check the certificate.
	checker, err := certchecker.GetCertChecker(c, cert.Issuer.CommonName)
	if err == nil {
		_, err = checker.CheckCertificate(c, cert)
		if err == nil {
			return &admin.CheckCertificateResponse{
				IsValid: true,
			}, nil
		}
	}

	// Recognize error codes related to CA cert checking. Everything else is
	// transient errors.
	if details, ok := err.(certchecker.Error); ok {
		return &admin.CheckCertificateResponse{
			IsValid:       false,
			InvalidReason: details.Error(),
		}, nil
	}
	return nil, grpc.Errorf(codes.Internal, "failed to check the certificate - %s", err)
}

////////////////////////////////////////////////////////////////////////////////

// fetchConfigFile fetches a file from this services' config set.
func fetchConfigFile(c context.Context, path string) (*config.Config, error) {
	configSet := "services/" + info.Get(c).AppID()
	logging.Infof(c, "Reading %q from config set %q", path, configSet)
	c, _ = context.WithTimeout(c, 30*time.Second) // URL fetch deadline
	return config.GetConfig(c, configSet, path, false)
}

// importCA imports CA definition from the config (or updates an existing one).
func (s *Server) importCA(c context.Context, ca *admin.CertificateAuthorityConfig, certPem string, rev string) error {
	// Read CA certificate file, convert it to der.
	certDer, err := utils.ParsePEM(certPem, "CERTIFICATE")
	if err != nil {
		return fmt.Errorf("bad PEM - %s", err)
	}

	// Check the certificate makes sense.
	cert, err := x509.ParseCertificate(certDer)
	if err != nil {
		return fmt.Errorf("bad cert - %s", err)
	}
	if !cert.IsCA {
		return fmt.Errorf("not a CA cert")
	}
	if cert.Subject.CommonName != ca.Cn {
		return fmt.Errorf("bad CN in the certificate, expecting %q, got %q", ca.Cn, cert.Subject.CommonName)
	}

	// Serialize the config back to proto to store it in the entity.
	cfgBlob, err := proto.Marshal(ca)
	if err != nil {
		return err
	}

	// Create or update the entity.
	return datastore.Get(c).RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		existing := model.CA{CN: ca.Cn}
		err := ds.Get(&existing)
		if err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		// New one?
		if err == datastore.ErrNoSuchEntity {
			logging.Infof(c, "Adding new CA %q", ca.Cn)
			return ds.Put(&model.CA{
				CN:         ca.Cn,
				Config:     cfgBlob,
				Cert:       certDer,
				AddedRev:   rev,
				UpdatedRev: rev,
			})
		}
		// Exists already? Check whether we should update it.
		if !existing.Removed &&
			bytes.Equal(existing.Config, cfgBlob) &&
			bytes.Equal(existing.Cert, certDer) {
			return nil
		}
		logging.Infof(c, "Updating CA %q", ca.Cn)
		existing.Config = cfgBlob
		existing.Cert = certDer
		existing.Removed = false
		existing.UpdatedRev = rev
		existing.RemovedRev = ""
		return ds.Put(&existing)
	}, nil)
}

// removeCA marks the CA in the datastore as removed.
func (s *Server) removeCA(c context.Context, name string, rev string) error {
	return datastore.Get(c).RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		existing := model.CA{CN: name}
		if err := ds.Get(&existing); err != nil {
			return err
		}
		if existing.Removed {
			return nil
		}
		logging.Infof(c, "Removing CA %q", name)
		existing.Removed = true
		existing.RemovedRev = rev
		return ds.Put(&existing)
	}, nil)
}
