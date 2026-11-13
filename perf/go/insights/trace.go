// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"fmt"
	"math"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// normalizeTraceCommits applies the default and bounds for a number of commits around an anomaly.
func normalizeTraceCommits(field string, n, def int) (int, error) {
	if n == 0 {
		return def, nil
	}
	if n < 1 || n > MaxTraceCommits {
		return 0, InvalidArgument(fmt.Sprintf("use a value between 1 and %d", MaxTraceCommits), "invalid %s: %d", field, n)
	}
	return n, nil
}

// GetAnomalyTrace returns the measurements of an anomaly's trace from CommitsBefore commits before
// its commit range to CommitsAfter commits after it. It shows whether the change is a clean step or
// noise, whether it persisted or recovered, and whether similar changes happened before. For an
// anomaly on a cluster of traces, only the first trace is returned.
//
// The trace key comes from the stored anomaly, never from the caller.
func (s *Service) GetAnomalyTrace(ctx context.Context, req AnomalyTraceRequest) (*AnomalyTraceResponse, error) {
	if s.stores.Traces == nil {
		return nil, NotFound("", "trace data is not available on this instance")
	}
	ids, err := normalizeIDs("id", []string{req.ID}, 1)
	if err != nil {
		return nil, err
	}
	before, err := normalizeTraceCommits("commits_before", req.CommitsBefore, DefaultTraceCommitsBefore)
	if err != nil {
		return nil, err
	}
	after, err := normalizeTraceCommits("commits_after", req.CommitsAfter, DefaultTraceCommitsAfter)
	if err != nil {
		return nil, err
	}

	regs, err := s.stores.Regressions.GetByIDs(ctx, ids)
	if err != nil {
		return nil, skerr.Wrapf(err, "reading the regression")
	}
	var reg *regression.Regression
	for _, r := range regs {
		if r != nil && r.Id == ids[0] {
			reg = r
		}
	}
	if reg == nil {
		return nil, NotFound("anomaly ids come from list_anomalies, summarize_anomalies or the Perf UI", "anomaly not found")
	}
	keys := traceKeys(reg)
	if len(keys) == 0 || !query.IsValid(keys[0]) {
		return nil, NotFound("", "the anomaly has no trace")
	}

	sum := s.toSummary(reg)
	begin := sum.CommitRange.Start - int64(before)
	if begin < 0 {
		begin = 0
	}
	end := sum.CommitRange.End + int64(after)
	if end < begin {
		end = begin
	}
	commits, values, err := s.stores.Traces.ReadTrace(ctx, keys[0], types.CommitNumber(begin), types.CommitNumber(end))
	if err != nil {
		return nil, skerr.Wrapf(err, "reading the trace")
	}
	if len(commits) != len(values) {
		return nil, skerr.Fmt("the trace reader returned %d commits and %d values", len(commits), len(values))
	}

	resp := &AnomalyTraceResponse{
		Anomaly:     sum,
		TraceKey:    keys[0],
		Begin:       begin,
		End:         end,
		Points:      make([]TracePoint, 0, len(values)),
		OtherTraces: len(keys) - 1,
	}
	for i, v := range values {
		f := float64(v)
		if v == vec32.MissingDataSentinel || math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		resp.Points = append(resp.Points, TracePoint{Commit: int64(commits[i]), Value: roundSig(f, 6)})
	}
	return resp, nil
}
