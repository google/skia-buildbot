// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tracereader reads single traces from the TraceValues table of a Perf database.
//
// The trace store (perf/go/tracestore/sqltracestore) loads an in-memory index of every trace in
// the instance when it is created. That is right for perf-fe, but too heavy for perfinsights, which
// only reads the trace of one anomaly at a time. This reader runs one parameterized, read-only
// query instead: the same lookup by trace id and commit range the trace store does.
package tracereader

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/types"
)

// readTrace reads one trace over a commit range. Every value is a query parameter.
const readTrace = `
	SELECT commit_number, val
	FROM TraceValues
	WHERE trace_id = $1 AND commit_number >= $2 AND commit_number <= $3
	ORDER BY commit_number
	LIMIT $4`

// MaxRows bounds the rows read by one call.
const MaxRows = 5000

// Querier is the subset of pool.Pool used by the reader.
type Querier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// Reader reads traces. It is safe for concurrent use.
type Reader struct {
	db Querier
}

// New returns a new Reader.
func New(db Querier) *Reader {
	return &Reader{db: db}
}

// ReadTrace returns the commit numbers and values of the trace with the given key between begin
// and end, inclusive, in commit order, up to MaxRows of them. Commits without a value are left out.
func (r *Reader) ReadTrace(ctx context.Context, traceKey string, begin, end types.CommitNumber) ([]types.CommitNumber, []float32, error) {
	if !query.IsValid(traceKey) {
		return nil, nil, skerr.Fmt("invalid trace key")
	}
	if begin < 0 || end < begin {
		return nil, nil, skerr.Fmt("invalid commit range [%d, %d]", begin, end)
	}
	traceID := types.TraceIDForSQLInBytesFromTraceName(traceKey)
	rows, err := r.db.Query(ctx, readTrace, traceID[:], int64(begin), int64(end), MaxRows)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "reading trace values")
	}
	defer rows.Close()
	commits := []types.CommitNumber{}
	values := []float32{}
	for rows.Next() {
		var commit int64
		var value float32
		if err := rows.Scan(&commit, &value); err != nil {
			return nil, nil, skerr.Wrapf(err, "scanning trace values")
		}
		commits = append(commits, types.CommitNumber(commit))
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, skerr.Wrapf(err, "reading trace values")
	}
	return commits, values, nil
}
