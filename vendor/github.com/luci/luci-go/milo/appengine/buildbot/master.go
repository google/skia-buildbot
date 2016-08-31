// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/settings"

	"golang.org/x/net/context"
)

func decodeMasterEntry(
	c context.Context, entry *buildbotMasterEntry, master *buildbotMaster) error {

	reader, err := gzip.NewReader(bytes.NewReader(entry.Data))
	if err != nil {
		return err
	}
	defer reader.Close()
	if err = json.NewDecoder(reader).Decode(master); err != nil {
		return err
	}
	return nil
}

// getMasterJSON fetches the latest known buildbot master data and returns
// the buildbotMaster struct (if found), whether or not it is internal,
// the last modified time, and an error if not found.
func getMasterJSON(c context.Context, name string) (
	master *buildbotMaster, internal bool, t time.Time, err error) {
	master = &buildbotMaster{}
	entry := buildbotMasterEntry{Name: name}
	ds := datastore.Get(c)
	err = ds.Get(&entry)
	if err != nil {
		logging.WithError(err).Errorf(
			c, "Encountered error while fetching entry for %s:\n%s", name, err)
		return
	}
	internal = entry.Internal
	t = entry.Modified
	err = decodeMasterEntry(c, &entry, master)
	return
}

// GetAllBuilders returns a resp.Module object containing all known masters
// and builders.
func GetAllBuilders(c context.Context) (*resp.Module, error) {
	result := &resp.Module{Name: "Buildbot"}
	// Fetch all Master entries from datastore
	ds := datastore.Get(c)
	q := datastore.NewQuery("buildbotMasterEntry")
	// TODO(hinoka): Maybe don't look past like a month or so?
	entries := []*buildbotMasterEntry{}
	err := ds.GetAll(q, &entries)
	if err != nil {
		return nil, err
	}

	// Add each builder from each master entry into the result.
	// TODO(hinoka): FanInOut this?
	for _, entry := range entries {
		if entry.Internal {
			// Bypass the master if it's an internal master and the user is not
			// part of the buildbot-private project.
			allowed, err := settings.IsAllowedInternal(c)
			if err != nil {
				logging.WithError(err).Errorf(c, "Could not process master %s", entry.Name)
				return nil, err
			}
			if !allowed {
				continue
			}
		}
		master := &buildbotMaster{}
		err = decodeMasterEntry(c, entry, master)
		if err != nil {
			logging.WithError(err).Errorf(c, "Could not decode %s", entry.Name)
			continue
		}
		ml := resp.MasterListing{Name: entry.Name}
		// Sort the builder listing.
		sb := make([]string, 0, len(master.Builders))
		for bn := range master.Builders {
			sb = append(sb, bn)
		}
		sort.Strings(sb)
		for _, bn := range sb {
			ml.Builders = append(ml.Builders, resp.Link{
				Label: bn,
				// Go templates escapes this for us, and also
				// slashes are not allowed in builder names.
				URL: fmt.Sprintf("/buildbot/%s/%s", entry.Name, bn),
			})
		}
		result.Masters = append(result.Masters, ml)
	}
	return result, nil
}
