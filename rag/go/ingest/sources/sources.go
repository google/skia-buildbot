// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sources

import "context"

// Source defines an interface for data source provider.
type Source interface {
	// Ingest performs the ingestion of the provided data.
	Ingest(ctx context.Context) error
}
