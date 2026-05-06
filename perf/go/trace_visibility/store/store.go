package store

import (
	"context"

	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore/schema"
)

// Store is an interface for the configuration store.
type Store interface {
	GetAll(ctx context.Context) ([]schema.PublicTraceRulesSchema, error)
}
