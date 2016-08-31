// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"github.com/luci/gae/service/datastore"
	dm "github.com/luci/luci-go/dm/api/service/v1"
)

// AttemptResult holds the raw, compressed json blob returned from the
// execution.
type AttemptResult struct {
	_id     int64          `gae:"$id,1"`
	Attempt *datastore.Key `gae:"$parent"`

	// The sizes and expirations are denormalized across Attempt and
	// AttemptResult.
	Data dm.JsonResult `gae:",noindex"`
}
