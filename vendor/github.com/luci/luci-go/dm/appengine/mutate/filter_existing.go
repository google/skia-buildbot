// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/dm/appengine/model"
	"golang.org/x/net/context"
)

// filterExisting removes the FwdDep objects which already exist.
//
// returns gRPC code error.
func filterExisting(c context.Context, fwdDeps []*model.FwdDep) ([]*model.FwdDep, error) {
	ret := make([]*model.FwdDep, 0, len(fwdDeps))

	err := datastore.Get(c).Get(fwdDeps)
	if err == nil {
		return nil, nil
	}

	merr, ok := err.(errors.MultiError)
	if !ok {
		return nil, err
	}

	for i, err := range merr {
		if err == nil {
			continue
		}
		ret = append(ret, fwdDeps[i])
	}

	return ret, nil
}
