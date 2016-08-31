// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gaesettings implements settings.Storage interface on top of GAE
// datastore.
//
// See github.com/luci/luci-go/server/settings for more details.
package gaesettings

import (
	"encoding/json"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/filter/txnBuf"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/settings"
)

// Storage knows how to store JSON blobs with settings in the datastore.
//
// It implements server/settings.EventualConsistentStorage interface.
type Storage struct{}

// settingsEntity is used to store all settings as JSON blob. Latest settings
// are stored under key (gaesettings.Settings, latest). The historical log is
// stored using exact same entity under keys (gaesettings.SettingsLog, version),
// with parent being (gaesettings.Settings, latest). Version is monotonically
// increasing integer starting from 1.
type settingsEntity struct {
	Kind    string         `gae:"$kind"`
	ID      string         `gae:"$id"`
	Parent  *datastore.Key `gae:"$parent"`
	Version int            `gae:",noindex"`
	Value   string         `gae:",noindex"`
	Who     string         `gae:",noindex"`
	Why     string         `gae:",noindex"`
	When    time.Time      `gae:",noindex"`

	// Disable dscache, since settings must remain functional in case memcache is
	// malfunctioning.
	_ datastore.Toggle `gae:"$dscache.enable,false"`
}

// defaultDS returns datastore interface configured to use default namespace and
// escape any current transaction.
func defaultDS(c context.Context) datastore.Interface {
	return txnBuf.GetNoTxn(info.Get(c).MustNamespace(""))
}

// latestSettings returns settingsEntity with prefilled key pointing to latest
// settings.
func latestSettings() settingsEntity {
	return settingsEntity{Kind: "gaesettings.Settings", ID: "latest"}
}

// expirationDuration returns how long to hold settings in memory cache.
//
// One minute in prod, one second on dev server (since long expiration time on
// dev server is very annoying).
func (s Storage) expirationDuration(c context.Context) time.Duration {
	if info.Get(c).IsDevAppServer() {
		return time.Second
	}
	return time.Minute
}

// FetchAllSettings fetches all latest settings at once.
func (s Storage) FetchAllSettings(c context.Context) (*settings.Bundle, error) {
	logging.Debugf(c, "Fetching app settings from the datastore")

	latest := latestSettings()
	switch err := defaultDS(c).Get(&latest); {
	case err == datastore.ErrNoSuchEntity:
		break
	case err != nil:
		return nil, errors.WrapTransient(err)
	}

	pairs := map[string]*json.RawMessage{}
	if latest.Value != "" {
		if err := json.Unmarshal([]byte(latest.Value), &pairs); err != nil {
			return nil, err
		}
	}
	return &settings.Bundle{
		Values: pairs,
		Exp:    clock.Now(c).Add(s.expirationDuration(c)),
	}, nil
}

// UpdateSetting updates a setting at the given key.
func (s Storage) UpdateSetting(c context.Context, key string, value json.RawMessage, who, why string) error {
	var fatalFail error // set in transaction on fatal errors
	err := defaultDS(c).RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)

		// Fetch the most recent values.
		latest := latestSettings()
		if err := ds.Get(&latest); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}

		// Update the value.
		pairs := map[string]*json.RawMessage{}
		if len(latest.Value) != 0 {
			if err := json.Unmarshal([]byte(latest.Value), &pairs); err != nil {
				fatalFail = err
				return err
			}
		}
		pairs[key] = &value

		// Store the previous one in the log.
		auditCopy := latest
		auditCopy.Kind = "gaesettings.SettingsLog"
		auditCopy.ID = strconv.Itoa(latest.Version)
		auditCopy.Parent = ds.KeyForObj(&latest)

		// Prepare a new version.
		buf, err := json.MarshalIndent(pairs, "", "  ")
		if err != nil {
			fatalFail = err
			return err
		}
		latest.Version++
		latest.Value = string(buf)
		latest.Who = who
		latest.Why = why
		latest.When = clock.Now(c).UTC()

		// Skip update if no changes at all.
		if latest.Value == auditCopy.Value {
			return nil
		}

		// Don't store copy of "no settings at all", it's useless.
		if latest.Version == 1 {
			return ds.Put(&latest)
		}
		return ds.Put(&latest, &auditCopy)
	}, nil)

	if fatalFail != nil {
		return fatalFail
	}
	return errors.WrapTransient(err)
}

// GetConsistencyTime returns "last modification time" + "expiration period".
//
// It indicates moment in time when last setting change is fully propagated to
// all instances.
//
// Returns zero time if there are no settings stored.
func (s Storage) GetConsistencyTime(c context.Context) (time.Time, error) {
	latest := latestSettings()
	switch err := defaultDS(c).Get(&latest); err {
	case nil:
		return latest.When.Add(s.expirationDuration(c)), nil
	case datastore.ErrNoSuchEntity:
		return time.Time{}, nil
	default:
		return time.Time{}, errors.WrapTransient(err)
	}
}
