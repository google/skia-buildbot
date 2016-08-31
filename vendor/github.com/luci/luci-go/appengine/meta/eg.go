// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package meta

import (
	"github.com/luci/gae/service/datastore"
	"golang.org/x/net/context"
)

// EntityGroupMeta is the model corresponding to the __entity_group__ model in
// appengine. You shouldn't need to use this struct directly, but instead should
// use GetEntityGroupVersion.
type EntityGroupMeta struct {
	kind   string         `gae:"$kind,__entity_group__"`
	id     int64          `gae:"$id,1"`
	Parent *datastore.Key `gae:"$parent"`

	Version int64 `gae:"__version__"`
}

// GetEntityGroupVersion returns the entity group version for the entity group
// containing root. If the entity group doesn't exist, this function will return
// zero and a nil error.
func GetEntityGroupVersion(c context.Context, key *datastore.Key) (int64, error) {
	ds := datastore.Get(c)
	egm := &EntityGroupMeta{Parent: key.Root()}
	err := ds.Get(egm)
	ret := egm.Version
	if err == datastore.ErrNoSuchEntity {
		// this is OK for callers. The version of the entity group is effectively 0
		// in this case.
		err = nil
	}
	return ret, err
}
