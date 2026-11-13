// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
	agpb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/clustering2"
	culpritpb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

const sheriff = "V8 Perf Sheriff"

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// fakeRegressions mimics sqlregression2store for the read methods used by the service.
type fakeRegressions struct {
	regs        []*regression.Regression
	byIDCalls   [][]string
	bySubCalls  []regression.GetAnomalyListRequest
	revisionErr error
}

func (f *fakeRegressions) GetRegressionsBySubName(_ context.Context, req regression.GetAnomalyListRequest, limit int) ([]*regression.Regression, error) {
	f.bySubCalls = append(f.bySubCalls, req)
	// Like readBySubNameIncludeTriaged: filter by sub_name and improvements, newest first.
	matching := []*regression.Regression{}
	for _, r := range f.regs {
		if r.SubscriptionName != req.SubName || (r.IsImprovement && !req.IncludeImprovements) {
			continue
		}
		matching = append(matching, r)
	}
	sort.SliceStable(matching, func(i, j int) bool { return matching[i].CreationTime.After(matching[j].CreationTime) })
	if req.PaginationOffset >= len(matching) {
		return []*regression.Regression{}, nil
	}
	end := req.PaginationOffset + limit
	if end > len(matching) {
		end = len(matching)
	}
	return matching[req.PaginationOffset:end], nil
}

func (f *fakeRegressions) GetByIDs(_ context.Context, ids []string) ([]*regression.Regression, error) {
	f.byIDCalls = append(f.byIDCalls, ids)
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	ret := []*regression.Regression{}
	for _, r := range f.regs {
		if want[r.Id] {
			ret = append(ret, r)
		}
	}
	return ret, nil
}

func (f *fakeRegressions) GetByRevision(_ context.Context, rev string) ([]*regression.Regression, error) {
	if f.revisionErr != nil {
		return nil, f.revisionErr
	}
	n, err := strconv.Atoi(rev)
	if err != nil {
		return nil, err
	}
	ret := []*regression.Regression{}
	for _, r := range f.regs {
		if int(r.PrevCommitNumber) < n && int(r.CommitNumber) >= n {
			ret = append(ret, r)
		}
	}
	return ret, nil
}

func (f *fakeRegressions) GetBugIdsForRegressions(_ context.Context, regs []*regression.Regression) ([]*regression.Regression, error) {
	for _, r := range regs {
		r.Bugs = append(r.Bugs, types.RegressionBug{BugId: "4242", Type: types.AutoBisect})
	}
	return regs, nil
}

type fakeSubs struct {
	subs  []*subpb.Subscription
	calls int
}

func (f *fakeSubs) GetAllActiveSubscriptions(context.Context) ([]*subpb.Subscription, error) {
	f.calls++
	return f.subs, nil
}

type fakeCommits struct{}

func hashOf(n int64) string { return fmt.Sprintf("%040x", n) }

func (fakeCommits) CommitFromCommitNumber(_ context.Context, n types.CommitNumber) (provider.Commit, error) {
	return provider.Commit{CommitNumber: n, GitHash: hashOf(int64(n)), Subject: fmt.Sprintf("commit %d", n), Timestamp: int64(n) * 100}, nil
}

func (f fakeCommits) CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error) {
	ret := []provider.Commit{}
	for n := begin; n <= end; n++ {
		c, _ := f.CommitFromCommitNumber(ctx, n)
		ret = append(ret, c)
	}
	return ret, nil
}

func (fakeCommits) CommitNumberFromGitHash(_ context.Context, githash string) (types.CommitNumber, error) {
	if githash == hashOf(12) {
		return 12, nil
	}
	return types.BadCommitNumber, errors.New("not found")
}

type fakeAlerts struct{}

func (fakeAlerts) GetAlertConfig(id int64) (*alerts.Alert, error) {
	return &alerts.Alert{DisplayName: "v8 alert", Query: "benchmark=8888", Algo: types.StepFitGrouping, Step: types.AbsoluteStep, Radius: 1, Interesting: 10, DirectionAsString: alerts.BOTH}, nil
}

type fakeGroups struct{ group *agpb.AnomalyGroup }

func (f fakeGroups) LoadById(_ context.Context, id string) (*agpb.AnomalyGroup, error) {
	if f.group == nil || f.group.GroupId != id {
		return nil, errors.New("failed to load the anomaly group: no rows in result set")
	}
	return f.group, nil
}

type fakeCulprits struct{}

func (fakeCulprits) Get(_ context.Context, ids []string) ([]*culpritpb.Culprit, error) {
	ret := []*culpritpb.Culprit{}
	for _, id := range ids {
		ret = append(ret, &culpritpb.Culprit{Id: id, Commit: &culpritpb.Commit{Host: "chromium.googlesource.com", Project: "v8/v8", Ref: "refs/heads/main", Revision: "abc"}, IssueIds: []string{"777"}})
	}
	return ret, nil
}

func id(n int) string { return fmt.Sprintf("abcdef00-0000-4000-8000-%012d", n) }

const traceKey = ",benchmark=8888,bot=x86,master=demo,test=encode,units=ms,"

type regOpts struct {
	n           int
	age         time.Duration
	prev, curr  int
	before      float32
	after       float32
	improvement bool
	status      regression.Status
	key         string
}

func newReg(o regOpts) *regression.Regression {
	if o.key == "" {
		o.key = traceKey
	}
	if o.status == "" {
		o.status = regression.Untriaged
	}
	r := regression.NewRegression()
	r.Id = id(o.n)
	r.SubscriptionName = sheriff
	r.CreationTime = now.Add(-o.age)
	r.PrevCommitNumber = types.CommitNumber(o.prev)
	r.CommitNumber = types.CommitNumber(o.curr)
	r.DisplayCommitNumber = types.CommitNumber(o.curr)
	r.MedianBefore = o.before
	r.MedianAfter = o.after
	r.IsImprovement = o.improvement
	r.AlertId = 7
	r.Frame = &frame.FrameResponse{DataFrame: &dataframe.DataFrame{
		TraceSet: types.TraceSet{o.key: nil},
		Header: []*dataframe.ColumnHeader{
			{Offset: types.CommitNumber(o.prev), Timestamp: 1000},
			{Offset: types.CommitNumber(o.curr), Timestamp: 2000},
		},
	}}
	r.High = &clustering2.ClusterSummary{
		Centroid:  []float32{o.before, o.after},
		StepPoint: &dataframe.ColumnHeader{Offset: types.CommitNumber(o.curr)},
	}
	r.HighStatus = regression.TriageStatus{Status: o.status}
	return r
}

func newService(t *testing.T, regs ...*regression.Regression) (*Service, *fakeRegressions) {
	fr := &fakeRegressions{regs: regs}
	svc, err := New(Stores{
		Regressions:   fr,
		Subscriptions: &fakeSubs{subs: []*subpb.Subscription{{Name: sheriff, Revision: "r1", BugComponent: "123"}, {Name: "Other Sheriff"}}},
		AnomalyGroups: fakeGroups{group: &agpb.AnomalyGroup{GroupId: id(900), GroupAction: agpb.GroupActionType_BISECT, AnomalyIds: []string{id(1), id(2), "not-a-uuid"}, CulpritIds: []string{id(901)}, SubsciptionName: sheriff, BenchmarkName: "8888"}},
		Culprits:      fakeCulprits{},
		Commits:       fakeCommits{},
		Alerts:        fakeAlerts{},
	}, Options{InstanceName: "demo", InstanceURL: "https://perf.example.com", Now: func() time.Time { return now }})
	require.NoError(t, err)
	return svc, fr
}

func publicCode(t *testing.T, err error) Code {
	require.Error(t, err)
	return ToPublicError(err).Code
}

func TestListAnomalies_Defaults_ReturnsRecentUntriagedRegressionsNewestFirst(t *testing.T) {
	svc, fr := newService(t,
		newReg(regOpts{n: 1, age: time.Hour, prev: 4, curr: 5, before: 100, after: 110}),
		newReg(regOpts{n: 2, age: 2 * time.Hour, prev: 6, curr: 7, before: 10, after: 12, status: regression.Triaged}),
		newReg(regOpts{n: 3, age: 3 * time.Hour, prev: 8, curr: 9, before: 10, after: 8, improvement: true}),
		newReg(regOpts{n: 4, age: 4 * time.Hour, prev: 1, curr: 3, before: 50, after: 60}),
		newReg(regOpts{n: 5, age: 10 * 24 * time.Hour, prev: 1, curr: 2, before: 1, after: 2}),
	)
	resp, err := svc.ListAnomalies(context.Background(), ListAnomaliesRequest{Sheriff: sheriff})
	require.NoError(t, err)

	require.Len(t, resp.Anomalies, 2)
	assert.Equal(t, id(1), resp.Anomalies[0].ID)
	assert.Equal(t, id(4), resp.Anomalies[1].ID)
	assert.Empty(t, resp.NextPageToken, "the scan reached anomalies older than 7 days")
	assert.False(t, resp.Truncated)
	// Rows 1, 2 and 4 were read; row 5 is older than the cutoff, and the improvement was never
	// requested from the store.
	assert.Equal(t, 3, resp.TotalScanned)
	require.NotEmpty(t, fr.bySubCalls)
	assert.True(t, fr.bySubCalls[0].IncludeTriaged, "triaged rows must be included to get newest-first order")
	assert.False(t, fr.bySubCalls[0].IncludeImprovements)

	a := resp.Anomalies[0]
	assert.Equal(t, "demo/x86/8888/encode", a.Test.TestPath)
	assert.Equal(t, "ms", a.Test.Unit)
	assert.Equal(t, DirectionRegression, a.Direction)
	require.NotNil(t, a.PctChange)
	assert.Equal(t, 10.0, *a.PctChange)
	assert.Equal(t, CommitRange{Start: 5, End: 5, Display: 5}, a.CommitRange)
	assert.Equal(t, "demo/x86/8888/encode regressed +10.0% (100 -> 110 ms) in commits 5-5; untriaged.", a.Headline)
	assert.Equal(t, "https://perf.example.com/u/?anomalyIDs="+id(1), a.Links.Report)
}

func TestListAnomalies_PageTokens_WalkAllPages(t *testing.T) {
	regs := []*regression.Regression{}
	for i := 1; i <= 5; i++ {
		regs = append(regs, newReg(regOpts{n: i, age: time.Duration(i) * time.Hour, prev: i, curr: i + 1, before: 10, after: 11}))
	}
	svc, _ := newService(t, regs...)
	ctx := context.Background()

	got := []string{}
	token := ""
	for page := 0; page < 5; page++ {
		resp, err := svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, PageSize: 2, PageToken: token})
		require.NoError(t, err)
		for _, a := range resp.Anomalies {
			got = append(got, a.ID)
		}
		token = resp.NextPageToken
		if token == "" {
			break
		}
	}
	assert.Equal(t, []string{id(1), id(2), id(3), id(4), id(5)}, got)
	assert.Empty(t, token)
}

func TestListAnomalies_Filters_BenchmarkBotStatusDirectionMinPct(t *testing.T) {
	svc, _ := newService(t,
		newReg(regOpts{n: 1, age: time.Hour, prev: 1, curr: 2, before: 100, after: 101}),
		newReg(regOpts{n: 2, age: 2 * time.Hour, prev: 1, curr: 2, before: 100, after: 120, key: ",benchmark=other,bot=arm,master=demo,test=decode,"}),
		newReg(regOpts{n: 3, age: 3 * time.Hour, prev: 1, curr: 2, before: 100, after: 80, improvement: true}),
		newReg(regOpts{n: 4, age: 4 * time.Hour, prev: 1, curr: 2, before: 100, after: 130, status: regression.Ignored}),
	)
	ctx := context.Background()

	resp, err := svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, MinAbsPct: 5})
	require.NoError(t, err)
	require.Len(t, resp.Anomalies, 1)
	assert.Equal(t, id(2), resp.Anomalies[0].ID)

	resp, err = svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, Benchmark: "8888", Bot: "x86", Status: "any", Direction: "any"})
	require.NoError(t, err)
	ids := []string{}
	for _, a := range resp.Anomalies {
		ids = append(ids, a.ID)
	}
	assert.Equal(t, []string{id(1), id(3), id(4)}, ids)

	resp, err = svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, Status: "ignored"})
	require.NoError(t, err)
	require.Len(t, resp.Anomalies, 1)
	assert.Equal(t, id(4), resp.Anomalies[0].ID)
}

func TestListAnomalies_InvalidInput_ReturnsInvalidArgumentWithHints(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	_, err := svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: "v8 perf"})
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
	assert.Contains(t, ToPublicError(err).Hint, sheriff)

	for name, req := range map[string]ListAnomaliesRequest{
		"empty sheriff":   {},
		"bad status":      {Sheriff: sheriff, Status: "fixed"},
		"bad direction":   {Sheriff: sheriff, Direction: "sideways"},
		"too many days":   {Sheriff: sheriff, Days: 91},
		"negative days":   {Sheriff: sheriff, Days: -1},
		"page too large":  {Sheriff: sheriff, PageSize: 101},
		"negative pct":    {Sheriff: sheriff, MinAbsPct: -1},
		"garbage token":   {Sheriff: sheriff, PageToken: "!!!"},
		"control chars":   {Sheriff: sheriff, Benchmark: "a\x00b"},
		"long bot filter": {Sheriff: sheriff, Bot: string(make([]byte, 300))},
	} {
		_, err := svc.ListAnomalies(ctx, req)
		assert.Equal(t, CodeInvalidArgument, publicCode(t, err), name)
	}
}

func TestListAnomalies_PageTokenFromOtherFilters_Rejected(t *testing.T) {
	regs := []*regression.Regression{}
	for i := 1; i <= 3; i++ {
		regs = append(regs, newReg(regOpts{n: i, age: time.Duration(i) * time.Hour, prev: i, curr: i + 1, before: 10, after: 11}))
	}
	svc, _ := newService(t, regs...)
	ctx := context.Background()
	resp, err := svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, PageSize: 1})
	require.NoError(t, err)
	require.NotEmpty(t, resp.NextPageToken)

	_, err = svc.ListAnomalies(ctx, ListAnomaliesRequest{Sheriff: sheriff, PageSize: 1, Days: 30, PageToken: resp.NextPageToken})
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
}

func TestBatchGetAnomalies_NonUUIDIDs_RejectedBeforeReachingTheStore(t *testing.T) {
	svc, fr := newService(t, newReg(regOpts{n: 1, prev: 1, curr: 2, before: 1, after: 2}))
	ctx := context.Background()
	for _, ids := range [][]string{
		{"' OR 1=1 --"},
		{id(1), "00000000-0000-4000-8000-000000000001') OR ('1'='1"},
		{"00000000000040008000000000000001"},
		{},
	} {
		_, err := svc.BatchGetAnomalies(ctx, ids, ViewBasic)
		assert.Equal(t, CodeInvalidArgument, publicCode(t, err), "%q", ids)
		assert.NotContains(t, err.Error(), "OR 1=1", "untrusted input must not be echoed back")
	}
	assert.Empty(t, fr.byIDCalls, "GetByIDs interpolates ids into SQL (FIX-01), so it must never see invalid ids")
}

func TestBatchGetAnomalies_FullView_ReturnsDetailsInRequestOrder(t *testing.T) {
	svc, fr := newService(t,
		newReg(regOpts{n: 1, prev: 4, curr: 6, before: 100, after: 110}),
		newReg(regOpts{n: 2, prev: 8, curr: 9, before: 50, after: 40}),
	)
	resp, err := svc.BatchGetAnomalies(context.Background(), []string{strings2Upper(id(2)), id(1), id(3), id(1)}, ViewFull)
	require.NoError(t, err)

	require.Len(t, fr.byIDCalls, 1)
	assert.Equal(t, []string{id(2), id(1), id(3)}, fr.byIDCalls[0], "ids are lower-cased and de-duplicated")
	require.Len(t, resp.Anomalies, 2)
	assert.Equal(t, id(2), resp.Anomalies[0].ID)
	assert.Equal(t, id(1), resp.Anomalies[1].ID)
	assert.Equal(t, []string{id(3)}, resp.NotFound)

	d := resp.Anomalies[1]
	require.NotNil(t, d.Alert)
	assert.Equal(t, "stepfit", d.Alert.Algo)
	assert.Equal(t, "absolute", d.Alert.Step)
	require.NotNil(t, d.Window)
	assert.False(t, d.Window.Normalized)
	assert.Equal(t, int64(6), d.Window.StepAt)
	assert.Equal(t, []WindowPoint{{Commit: 4, Timestamp: 1000, Value: 100}, {Commit: 6, Timestamp: 2000, Value: 110}}, d.Window.Points)
	require.NotNil(t, d.Commits)
	assert.Equal(t, 2, d.Commits.Total)
	require.Len(t, d.Commits.InRange, 2)
	assert.Equal(t, hashOf(5), d.Commits.InRange[0].Hash)
	assert.Equal(t, "commit 6", d.Commits.End.Subject)
	assert.Equal(t, []string{"4242"}, d.Triage.BugIDs, "full view adds auto-triage and auto-bisect bugs")
}

func strings2Upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'f' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

func TestBatchGetAnomalies_FullViewWithTooManyIDs_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := newService(t)
	ids := []string{}
	for i := 0; i < MaxFullViewIDs+1; i++ {
		ids = append(ids, id(i))
	}
	_, err := svc.BatchGetAnomalies(context.Background(), ids, ViewFull)
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))

	_, err = svc.BatchGetAnomalies(context.Background(), ids, ViewBasic)
	require.NoError(t, err)
}

func TestFindAnomaliesForRevision_CommitNumberHashAndInvalid(t *testing.T) {
	svc, _ := newService(t,
		newReg(regOpts{n: 1, prev: 10, curr: 15, before: 100, after: 101}),
		newReg(regOpts{n: 2, prev: 11, curr: 13, before: 100, after: 150}),
		newReg(regOpts{n: 3, prev: 20, curr: 21, before: 100, after: 150}),
	)
	ctx := context.Background()

	resp, err := svc.FindAnomaliesForRevision(ctx, "12")
	require.NoError(t, err)
	assert.Equal(t, int64(12), resp.Revision)
	require.Len(t, resp.Anomalies, 2)
	assert.Equal(t, id(2), resp.Anomalies[0].ID, "sorted by |pct_change|")

	resp, err = svc.FindAnomaliesForRevision(ctx, hashOf(12))
	require.NoError(t, err)
	assert.Len(t, resp.Anomalies, 2)

	_, err = svc.FindAnomaliesForRevision(ctx, hashOf(99))
	assert.Equal(t, CodeNotFound, publicCode(t, err))

	for _, rev := range []string{"", "abc", "1; DROP TABLE Regressions2", "-5", "12345678901"} {
		_, err = svc.FindAnomaliesForRevision(ctx, rev)
		assert.Equal(t, CodeInvalidArgument, publicCode(t, err), rev)
	}
}

func TestGetAnomalyGroup_ReturnsMembersAndCulprits(t *testing.T) {
	svc, fr := newService(t,
		newReg(regOpts{n: 1, prev: 1, curr: 2, before: 100, after: 101}),
		newReg(regOpts{n: 2, prev: 1, curr: 2, before: 100, after: 150}),
	)
	ctx := context.Background()
	resp, err := svc.GetAnomalyGroup(ctx, id(900))
	require.NoError(t, err)
	assert.Equal(t, "BISECT", resp.Action)
	assert.Equal(t, 3, resp.NumAnomalies)
	require.Len(t, resp.Anomalies, 2)
	assert.Equal(t, id(2), resp.Anomalies[0].ID)
	require.Len(t, resp.Culprits, 1)
	assert.Equal(t, "v8/v8", resp.Culprits[0].Project)
	assert.Equal(t, []string{id(1), id(2)}, fr.byIDCalls[0], "invalid member ids from the database are dropped")

	_, err = svc.GetAnomalyGroup(ctx, id(901))
	assert.Equal(t, CodeNotFound, publicCode(t, err))
	_, err = svc.GetAnomalyGroup(ctx, "x")
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
}

func TestSummarize_ComputesTotalsTopRegressionsAndOverlaps(t *testing.T) {
	svc, _ := newService(t,
		newReg(regOpts{n: 1, age: time.Hour, prev: 10, curr: 20, before: 100, after: 130}),
		newReg(regOpts{n: 2, age: 2 * time.Hour, prev: 15, curr: 25, before: 100, after: 110, key: ",benchmark=jetstream,bot=mac,master=m,test=score,"}),
		newReg(regOpts{n: 3, age: 3 * time.Hour, prev: 40, curr: 41, before: 100, after: 105, status: regression.Triaged}),
		newReg(regOpts{n: 4, age: 4 * time.Hour, prev: 5, curr: 6, before: 100, after: 90, improvement: true}),
		newReg(regOpts{n: 5, age: 30 * 24 * time.Hour, prev: 1, curr: 2, before: 1, after: 9}),
	)
	sum, err := svc.Summarize(context.Background(), SummarizeRequest{Sheriff: sheriff})
	require.NoError(t, err)

	assert.Equal(t, 7, sum.Scope.Days)
	assert.Equal(t, 4, sum.Scope.TotalScanned)
	assert.Equal(t, SummaryTotals{Regressions: 3, Improvements: 1, Untriaged: 3, Triaged: 1}, sum.Totals)
	require.Len(t, sum.TopRegressions, 3)
	assert.Equal(t, id(1), sum.TopRegressions[0].ID)
	require.Len(t, sum.ByBenchmark, 2)
	assert.Equal(t, BenchmarkCount{Benchmark: "8888", Regressions: 2, MedianPctChange: floatPtr(17.5)}, sum.ByBenchmark[0])
	require.Len(t, sum.OverlappingRanges, 1)
	assert.Equal(t, OverlapCluster{CommitRange: [2]int64{16, 20}, Count: 2, Benchmarks: []string{"8888", "jetstream"}, AnomalyIDs: []string{id(1), id(2)}}, sum.OverlappingRanges[0])
	require.Len(t, sum.RepeatOffenders, 1)
	assert.Equal(t, RepeatOffender{TestPath: "demo/x86/8888/encode", Anomalies: 2}, sum.RepeatOffenders[0])

	_, err = svc.Summarize(context.Background(), SummarizeRequest{Sheriff: "nope"})
	assert.Equal(t, CodeInvalidArgument, publicCode(t, err))
}

func floatPtr(f float64) *float64 { return &f }

func TestListSheriffs_SortedAndCached(t *testing.T) {
	subs := &fakeSubs{subs: []*subpb.Subscription{{Name: "b"}, {Name: "a", ContactEmail: "a@example.com"}}}
	svc, err := New(Stores{Regressions: &fakeRegressions{}, Subscriptions: subs}, Options{Now: func() time.Time { return now }})
	require.NoError(t, err)
	resp, err := svc.ListSheriffs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []Sheriff{{Name: "a", Contact: "a@example.com"}, {Name: "b"}}, resp.Sheriffs)
	_, err = svc.ListSheriffs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, subs.calls)
}

func TestToPublicError_HidesInternalDetails(t *testing.T) {
	pe := ToPublicError(fmt.Errorf("failed to get regressions by id list. Query: SELECT * FROM Regressions2"))
	assert.Equal(t, CodeInternal, pe.Code)
	assert.NotContains(t, pe.Message, "SELECT")

	pe = ToPublicError(fmt.Errorf("wrapped: %w", context.DeadlineExceeded))
	assert.Equal(t, CodeDeadlineExceeded, pe.Code)

	inv := InvalidArgument("hint", "bad %s", "thing")
	assert.Equal(t, inv, ToPublicError(fmt.Errorf("wrapped: %w", inv)))
	b, err := json.Marshal(ErrorEnvelope{Error: inv})
	require.NoError(t, err)
	assert.JSONEq(t, `{"error":{"code":"INVALID_ARGUMENT","message":"bad thing","hint":"hint"}}`, string(b))
}

func TestPageToken_RoundTripAndRejectsTampering(t *testing.T) {
	fh := filterHash("a", "b")
	tok := encodePageToken(42, fh)
	off, err := decodePageToken(tok, fh)
	require.NoError(t, err)
	assert.Equal(t, 42, off)

	_, err = decodePageToken(tok, filterHash("a", "c"))
	assert.Error(t, err)
	_, err = decodePageToken(encodePageToken(-1, fh), fh)
	assert.Error(t, err)
	_, err = decodePageToken(string(make([]byte, 1000)), fh)
	assert.Error(t, err)
}

func TestParseTestInfo(t *testing.T) {
	info := parseTestInfo(",benchmark=jetstream2,bot=mac-m1,master=ChromiumPerf,subtest_1=a,subtest_2=b,test=Score,unit=score,")
	assert.Equal(t, "ChromiumPerf/mac-m1/jetstream2/Score/a/b", info.TestPath)
	assert.Equal(t, "score", info.Unit)
	assert.Equal(t, []string{"a", "b"}, info.Subtests)

	assert.Equal(t, ",arch=x86,config=8888,", parseTestInfo(",arch=x86,config=8888,").TestPath)
	assert.Equal(t, "not a key", parseTestInfo("not a key").TestPath)
	assert.Equal(t, "unknown", parseTestInfo("").TestPath)
}

func TestWindowOf_SkipsMissingDataAndCentersOnStep(t *testing.T) {
	r := newReg(regOpts{n: 1, prev: 0, curr: 100, before: 1, after: 2})
	r.Frame.DataFrame.Header = nil
	r.High.Centroid = nil
	for i := 0; i < 100; i++ {
		r.Frame.DataFrame.Header = append(r.Frame.DataFrame.Header, &dataframe.ColumnHeader{Offset: types.CommitNumber(i)})
		v := float32(i)
		if i == 3 {
			v = 1e32
		}
		r.High.Centroid = append(r.High.Centroid, v)
	}
	r.High.StepPoint = &dataframe.ColumnHeader{Offset: 80}
	w := windowOf(r, false)
	require.NotNil(t, w)
	assert.True(t, w.Truncated)
	require.Len(t, w.Points, MaxWindowPoints)
	assert.Equal(t, int64(80), w.StepAt)
	assert.Equal(t, int64(36), w.Points[0].Commit)
	assert.Equal(t, int64(99), w.Points[len(w.Points)-1].Commit)
	assert.Nil(t, windowOf(newRegNoCluster(), false))
}

func newRegNoCluster() *regression.Regression {
	r := regression.NewRegression()
	r.Id = id(1)
	return r
}

func TestMediansOf_FallsBackToTheCentroidWhenPerfStoredZeros(t *testing.T) {
	// Perf stores 0 for both medians when the alert's radius gives fewer than 4 points per side,
	// as with the demo alert (radius 1).
	r := newReg(regOpts{n: 1, prev: 4, curr: 5})
	r.High.Centroid = []float32{50.8, 91.66}
	r.High.StepFit = &stepfit.StepFit{TurningPoint: 1}
	before, after := mediansOf(r)
	assert.InDelta(t, 50.8, before, 1e-4)
	assert.InDelta(t, 91.66, after, 1e-4)

	svc, _ := newService(t, r)
	sum := svc.toSummary(r)
	assert.Equal(t, 50.8, sum.MedianBefore)
	assert.Equal(t, 91.66, sum.MedianAfter)
	require.NotNil(t, sum.PctChange)
	assert.InDelta(t, 80.43, *sum.PctChange, 0.01)

	// Stored medians win.
	r.MedianBefore, r.MedianAfter = 10, 12
	before, after = mediansOf(r)
	assert.Equal(t, 10.0, before)
	assert.Equal(t, 12.0, after)

	// Without a turning point there is nothing to fall back to.
	r2 := newReg(regOpts{n: 2, prev: 4, curr: 5})
	r2.High.StepFit = nil
	before, after = mediansOf(r2)
	assert.Zero(t, before)
	assert.Zero(t, after)
}
