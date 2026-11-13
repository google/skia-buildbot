// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/regression"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
)

type traceCall struct {
	key        string
	begin, end types.CommitNumber
}

type fakeTraces struct {
	calls   []traceCall
	commits []types.CommitNumber
	values  []float32
	err     error
}

func (f *fakeTraces) ReadTrace(_ context.Context, key string, begin, end types.CommitNumber) ([]types.CommitNumber, []float32, error) {
	f.calls = append(f.calls, traceCall{key, begin, end})
	return f.commits, f.values, f.err
}

func newTraceService(t *testing.T, traces TraceReader, regs ...*regression.Regression) (*Service, *fakeRegressions) {
	fr := &fakeRegressions{regs: regs}
	svc, err := New(Stores{
		Regressions:   fr,
		Subscriptions: &fakeSubs{subs: []*subpb.Subscription{{Name: sheriff}}},
		Traces:        traces,
	}, Options{InstanceName: "demo", InstanceURL: "https://perf.example.com", Now: func() time.Time { return now }})
	require.NoError(t, err)
	return svc, fr
}

func TestGetAnomalyTrace_ReadsTheAnomalysOwnTraceAroundItsRange(t *testing.T) {
	ft := &fakeTraces{
		commits: []types.CommitNumber{95, 96, 100, 101, 102},
		values:  []float32{10, 10.5, vec32.MissingDataSentinel, 20, 20.25},
	}
	// The commit range is [100, 101].
	svc, _ := newTraceService(t, ft, newReg(regOpts{n: 1, prev: 99, curr: 101, before: 10, after: 20}))

	resp, err := svc.GetAnomalyTrace(context.Background(), AnomalyTraceRequest{ID: strings.ToUpper(id(1)), CommitsBefore: 5, CommitsAfter: 2})
	require.NoError(t, err)
	require.Len(t, ft.calls, 1)
	assert.Equal(t, traceCall{traceKey, 95, 103}, ft.calls[0])
	assert.Equal(t, int64(95), resp.Begin)
	assert.Equal(t, int64(103), resp.End)
	assert.Equal(t, traceKey, resp.TraceKey)
	assert.Equal(t, id(1), resp.Anomaly.ID)
	assert.Equal(t, 10.0, resp.Anomaly.MedianBefore)
	assert.Equal(t, []TracePoint{{95, 10}, {96, 10.5}, {101, 20}, {102, 20.25}}, resp.Points, "missing data is left out")
	assert.Zero(t, resp.OtherTraces)
}

func TestGetAnomalyTrace_DefaultsAndClampsTheRange(t *testing.T) {
	ft := &fakeTraces{}
	svc, _ := newTraceService(t, ft, newReg(regOpts{n: 1, prev: 39, curr: 41, before: 10, after: 20}))
	resp, err := svc.GetAnomalyTrace(context.Background(), AnomalyTraceRequest{ID: id(1)})
	require.NoError(t, err)
	assert.Equal(t, traceCall{traceKey, 0, 41 + DefaultTraceCommitsAfter}, ft.calls[0], "begin can't go below commit 0")
	assert.Empty(t, resp.Points)
}

func TestGetAnomalyTrace_InvalidInputIsRejectedBeforeReading(t *testing.T) {
	ft := &fakeTraces{}
	svc, fr := newTraceService(t, ft, newReg(regOpts{n: 1, prev: 99, curr: 101}))
	for _, req := range []AnomalyTraceRequest{
		{ID: "' OR 1=1 --"},
		{ID: id(1), CommitsBefore: MaxTraceCommits + 1},
		{ID: id(1), CommitsAfter: -1},
	} {
		_, err := svc.GetAnomalyTrace(context.Background(), req)
		assert.Equal(t, CodeInvalidArgument, publicCode(t, err), "%+v", req)
	}
	assert.Empty(t, fr.byIDCalls)
	assert.Empty(t, ft.calls)
}

func TestGetAnomalyTrace_NotFoundAndUnavailable(t *testing.T) {
	ft := &fakeTraces{}
	svc, _ := newTraceService(t, ft)
	_, err := svc.GetAnomalyTrace(context.Background(), AnomalyTraceRequest{ID: id(1)})
	assert.Equal(t, CodeNotFound, publicCode(t, err))
	assert.Empty(t, ft.calls)

	noTraces, _ := newTraceService(t, nil, newReg(regOpts{n: 1, prev: 99, curr: 101}))
	_, err = noTraces.GetAnomalyTrace(context.Background(), AnomalyTraceRequest{ID: id(1)})
	assert.Equal(t, CodeNotFound, publicCode(t, err))
}

func TestGetAnomalyTrace_ReaderErrorsAreInternal(t *testing.T) {
	ft := &fakeTraces{err: errors.New("failed: SELECT * FROM TraceValues")}
	svc, _ := newTraceService(t, ft, newReg(regOpts{n: 1, prev: 99, curr: 101}))
	_, err := svc.GetAnomalyTrace(context.Background(), AnomalyTraceRequest{ID: id(1)})
	pe := ToPublicError(err)
	assert.Equal(t, CodeInternal, pe.Code)
	assert.NotContains(t, pe.Message, "SELECT")
}
