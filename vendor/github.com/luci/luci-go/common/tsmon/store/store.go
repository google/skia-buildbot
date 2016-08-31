// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package store contains code for storing and retreiving metrics.
package store

import (
	"time"

	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

// A Store is responsible for handling all metric data.
type Store interface {
	Register(m types.Metric)
	Unregister(m types.Metric)

	DefaultTarget() types.Target
	SetDefaultTarget(t types.Target)

	Get(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}) (value interface{}, err error)
	Set(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}, value interface{}) error
	Incr(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}, delta interface{}) error

	GetAll(c context.Context) []types.Cell

	Reset(c context.Context, m types.Metric)
}
