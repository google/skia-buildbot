// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"errors"
	"fmt"
	"time"

	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/logdog/common/types"
)

// LogPrefix is a datastore model for a prefix space. All log streams sharing
// a prefix will have a LogPrefix entry to group under.
//
// A LogPrefix is keyed on the hash of its Prefix property.
//
// Prefix-scoped properties are used to control creation and modification
// attributes of log streams sharing the prefix.
type LogPrefix struct {
	// ID is the LogPrefix's ID. It is an encoded hash value generated from the
	// stream's Prefix field.
	ID HashID `gae:"$id"`

	// Schema is the datastore schema version for this object. This can be used
	// to facilitate schema migrations.
	//
	// The current schema is currentSchemaVersion.
	Schema string

	// Created is the time when this stream was created.
	Created time.Time `gae:",noindex"`

	// Prefix is this log stream's prefix value. Log streams with the same prefix
	// are logically grouped.
	//
	// This value should not be changed once populated, as it will invalidate the
	// HashID.
	Prefix string `gae:",noindex"`

	// Source is the (indexed) set of source strings sent by the prefix registrar.
	Source []string

	// Expiration is the time when this log prefix expires. Stream registrations
	// for this prefix will fail after this point.
	Expiration time.Time

	// Secret is the Butler secret value for this prefix. All streams within
	// the prefix share this secret value.
	//
	// This value may only be returned to LogDog services; it is not user-visible.
	Secret []byte `gae:",noindex"`

	// extra causes datastore to ignore unrecognized fields and strip them in
	// future writes.
	extra ds.PropertyMap `gae:"-,extra"`
}

var _ interface {
	ds.PropertyLoadSaver
} = (*LogPrefix)(nil)

// LogPrefixID returns the HashID for a specific prefix.
func LogPrefixID(prefix types.StreamName) HashID {
	return makeHashID(string(prefix))
}

// Load implements ds.PropertyLoadSaver.
func (p *LogPrefix) Load(pmap ds.PropertyMap) error {
	if err := ds.GetPLS(p).Load(pmap); err != nil {
		return err
	}

	// Validate the log prefix. Don't enforce HashID correctness, since datastore
	// hasn't populated that field yet.
	if err := p.validateImpl(false); err != nil {
		return err
	}
	return nil
}

// Save implements ds.PropertyLoadSaver.
func (p *LogPrefix) Save(withMeta bool) (ds.PropertyMap, error) {
	if err := p.validateImpl(true); err != nil {
		return nil, err
	}
	p.Schema = CurrentSchemaVersion

	return ds.GetPLS(p).Save(withMeta)
}

// recalculateID calculates the hash ID from its Prefix field, which must be
// populated else this function will panic.
//
// The value is loaded into its ID field.
func (p *LogPrefix) recalculateID() {
	p.ID = p.getIDFromPrefix()
}

// getIDFromPrefix calculates the log stream's hash ID from its Prefix/Name
// fields, which must be populated else this function will panic.
func (p *LogPrefix) getIDFromPrefix() HashID {
	if p.Prefix == "" {
		panic("empty prefix")
	}
	return makeHashID(p.Prefix)
}

// Validate evaluates the state and data contents of the LogPrefix and returns
// an error if it is invalid.
func (p *LogPrefix) Validate() error {
	return p.validateImpl(true)
}

func (p *LogPrefix) validateImpl(enforceHashID bool) error {
	if enforceHashID {
		// Make sure our Prefix and Name match the Hash ID.
		if hid := p.getIDFromPrefix(); hid != p.ID {
			return fmt.Errorf("hash IDs don't match (%q != %q)", hid, p.ID)
		}
	}

	if err := types.StreamName(p.Prefix).Validate(); err != nil {
		return fmt.Errorf("invalid prefix: %s", err)
	}
	if err := types.PrefixSecret(p.Secret).Validate(); err != nil {
		return fmt.Errorf("invalid prefix secret: %s", err)
	}
	if p.Created.IsZero() {
		return errors.New("created time is not set")
	}
	return nil
}
