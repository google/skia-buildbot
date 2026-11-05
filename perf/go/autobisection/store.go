// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autobisection

import (
	"context"

	"go.skia.org/infra/perf/go/autobisection/sqlautobisectionstore/schema"
)

// Store defines the interface for persisting and querying autobisection results.
type Store interface {
	// Save saves a autobisection result to the database.
	Save(ctx context.Context, autobisection *schema.AutobisectionSchema) error
}
