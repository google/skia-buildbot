// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package model defines datastore models used by the token server.
package model

import (
	"bytes"
	"crypto/x509"
	"encoding/gob"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/caching/lazyslot"
	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/common/errors"

	"github.com/luci/luci-go/tokenserver/api/admin/v1"
)

// CA defines one trusted Certificate Authority (imported from config).
//
// Entity key is CA Common Name (that must match what's is in the certificate).
// Certificate issuer (and the certificate signature) is ignored. Usually, the
// certificates here will be self-signed.
//
// Removed CAs are kept in the datastore, but not actively used.
type CA struct {
	// CN is CA's Common Name.
	CN string `gae:"$id"`

	// Config is serialized CertificateAuthorityConfig proto message.
	Config []byte `gae:",noindex"`

	// Cert is a certificate of this CA (in der encoding).
	//
	// It is read from luci-config from path specified in the config.
	Cert []byte `gae:",noindex"`

	// Removed is true if this CA has been removed from the config.
	Removed bool

	// Ready is false before this CA's CRL is fetched for the first time.
	Ready bool

	AddedRev   string `gae:",noindex"` // config rev when this CA appeared
	UpdatedRev string `gae:",noindex"` // config rev when this CA was updated
	RemovedRev string `gae:",noindex"` // config rev when it was removed

	// ParsedConfig is parsed Config.
	//
	// Populated if CA is fetched through CertChecker.
	ParsedConfig *admin.CertificateAuthorityConfig `gae:"-"`

	// ParsedCert is parsed Cert.
	//
	// Populated if CA is fetched through CertChecker.
	ParsedCert *x509.Certificate `gae:"-"`
}

// ParseConfig parses proto message stored in Config.
func (c *CA) ParseConfig() (*admin.CertificateAuthorityConfig, error) {
	msg := &admin.CertificateAuthorityConfig{}
	if err := proto.Unmarshal(c.Config, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// CAUniqueIDToCNMap is a singleton entity that stores a mapping between CA's
// unique_id (specified in config) and its Common Name.
//
// It's loaded in memory in full and kept cached there (for 1 min).
// See GetCAByUniqueID below.
type CAUniqueIDToCNMap struct {
	_id int64 `gae:"$id,1"`

	GobEncodedMap []byte `gae:",noindex"` // gob-encoded map[int64]string
}

// StoreCAUniqueIDToCNMap overwrites CAUniqueIDToCNMap with new content.
func StoreCAUniqueIDToCNMap(c context.Context, mapping map[int64]string) error {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(mapping); err != nil {
		return err
	}
	// Note that in practice 'mapping' is usually very small, so we are not
	// concerned about 1MB entity size limit.
	return errors.WrapTransient(datastore.Get(c).Put(&CAUniqueIDToCNMap{
		GobEncodedMap: buf.Bytes(),
	}))
}

// LoadCAUniqueIDToCNMap loads CAUniqueIDToCNMap from the datastore.
func LoadCAUniqueIDToCNMap(c context.Context) (map[int64]string, error) {
	ent := CAUniqueIDToCNMap{}
	switch err := datastore.Get(c).Get(&ent); {
	case err == datastore.ErrNoSuchEntity:
		return nil, nil
	case err != nil:
		return nil, errors.WrapTransient(err)
	}
	dec := gob.NewDecoder(bytes.NewReader(ent.GobEncodedMap))
	out := map[int64]string{}
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCAByUniqueID returns CN name that corresponds to given unique ID.
//
// It uses cached CAUniqueIDToCNMap for lookups. Returns empty string if there's
// no such CA.
func GetCAByUniqueID(c context.Context, id int64) (string, error) {
	mapper, err := proccache.GetOrMake(c, mapperCacheKey(0), func() (interface{}, time.Duration, error) {
		return makeIDToCNmapper(), 0, nil
	})
	if err != nil {
		return "", err
	}
	return mapper.(*idToCNmapper).getCAByUniqueID(c, id)
}

type mapperCacheKey int

// idToCNmapper is stored in proccache, it does "unique ID -> CN name" mapping.
//
// It holds cached copy of CAUniqueIDToCNMap, periodically refreshing it.
type idToCNmapper struct {
	mapping lazyslot.Slot
}

func makeIDToCNmapper() *idToCNmapper {
	return &idToCNmapper{
		mapping: lazyslot.Slot{
			Fetcher: func(c context.Context, _ lazyslot.Value) (lazyslot.Value, error) {
				val, err := LoadCAUniqueIDToCNMap(c)
				return lazyslot.Value{
					Value:      val,
					Expiration: clock.Now(c).Add(time.Minute),
				}, err
			},
		},
	}
}

func (m *idToCNmapper) getCAByUniqueID(c context.Context, id int64) (string, error) {
	val, err := m.mapping.Get(c)
	if err != nil {
		return "", err
	}
	mapping := val.Value.(map[int64]string)
	return mapping[id], nil
}
