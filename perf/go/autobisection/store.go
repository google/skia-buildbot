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
