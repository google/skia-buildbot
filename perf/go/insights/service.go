// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package insights is a read-only service that serves Skia Perf anomaly insights to LLM agents
// and scripts, over REST (perf/go/insights/rest) and MCP (perf/go/insights/mcptools).
//
// It is the core of the perfinsights proof of concept and is meant to become the implementation
// behind perf.v1 /api/v1 (P3-03) and the stateless /mcp endpoint in perf-fe (P3-06). It only
// depends on narrow read interfaces of the existing Perf stores, so it cannot write anything.
package insights

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	agpb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	culpritpb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
)

// Bounds enforced by the service.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
	DefaultDays     = 7
	MaxDays         = 90

	// MaxScanRows is the maximum number of regressions read from the store per call.
	MaxScanRows = 2000

	// scanBatchSize is the number of regressions read from the store per query.
	scanBatchSize = 200

	// MaxBatchGetIDs is the maximum number of IDs for BatchGetAnomalies with the basic view.
	MaxBatchGetIDs = 500

	// MaxFullViewIDs is the maximum number of IDs for BatchGetAnomalies with the full view.
	MaxFullViewIDs = 20

	// MaxWindowPoints is the maximum number of data points in a value window.
	MaxWindowPoints = 64

	// MaxCommitsInRange is the maximum number of commits listed for an anomaly's range.
	MaxCommitsInRange = 50

	// MaxGroupMembers is the maximum number of anomalies listed for an anomaly group.
	MaxGroupMembers = 50

	// MaxRevisionResults is the maximum number of anomalies returned for a revision.
	MaxRevisionResults = 100

	// maxOtherTraces is the maximum number of extra trace keys listed for a multi-trace anomaly.
	maxOtherTraces = 10

	// DefaultTraceCommitsBefore and DefaultTraceCommitsAfter are how many commits of an
	// anomaly's trace GetAnomalyTrace reads around its commit range by default.
	DefaultTraceCommitsBefore = 100
	DefaultTraceCommitsAfter  = 50

	// MaxTraceCommits is the maximum number of commits GetAnomalyTrace reads on each side of an
	// anomaly's commit range.
	MaxTraceCommits = 500

	// subscriptionCacheTTL is how long the list of active subscriptions is cached.
	subscriptionCacheTTL = 5 * time.Minute
)

// RegressionReader is the read-only subset of regression.Store used by the service.
type RegressionReader interface {
	GetRegressionsBySubName(ctx context.Context, req regression.GetAnomalyListRequest, limit int) ([]*regression.Regression, error)
	GetByIDs(ctx context.Context, ids []string) ([]*regression.Regression, error)
	GetByRevision(ctx context.Context, rev string) ([]*regression.Regression, error)
	GetBugIdsForRegressions(ctx context.Context, regressions []*regression.Regression) ([]*regression.Regression, error)
}

// SubscriptionReader is the read-only subset of subscription.Store used by the service.
type SubscriptionReader interface {
	GetAllActiveSubscriptions(ctx context.Context) ([]*subpb.Subscription, error)
}

// AnomalyGroupReader is the read-only subset of anomalygroup.Store used by the service.
type AnomalyGroupReader interface {
	LoadById(ctx context.Context, groupID string) (*agpb.AnomalyGroup, error)
}

// CulpritReader is the read-only subset of culprit.Store used by the service.
type CulpritReader interface {
	Get(ctx context.Context, ids []string) ([]*culpritpb.Culprit, error)
}

// CommitReader is the read-only subset of perfgit.Git used by the service.
type CommitReader interface {
	CommitFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (provider.Commit, error)
	CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error)
	CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error)
}

// AlertReader is the read-only subset of alerts.ConfigProvider used by the service.
type AlertReader interface {
	GetAlertConfig(alertID int64) (*alerts.Alert, error)
}

// TraceReader reads the measurements of one trace.
type TraceReader interface {
	// ReadTrace returns the commit numbers and values of the trace with the given key between
	// begin and end, inclusive, in commit order. Commits without a value are left out.
	ReadTrace(ctx context.Context, traceKey string, begin, end types.CommitNumber) ([]types.CommitNumber, []float32, error)
}

// Stores are the data sources of the service. Regressions and Subscriptions are required; the
// others are optional and the matching fields are left out of responses when they are nil.
type Stores struct {
	Regressions   RegressionReader
	Subscriptions SubscriptionReader
	AnomalyGroups AnomalyGroupReader
	Culprits      CulpritReader
	Commits       CommitReader
	Alerts        AlertReader
	Traces        TraceReader
}

// ExploreURLFunc returns a link to graph the given trace params over a commit range.
type ExploreURLFunc func(ctx context.Context, startCommit, endCommit int, params map[string][]string) string

// Options configure the service.
type Options struct {
	// InstanceName is the name of the Perf instance, e.g. "v8-internal".
	InstanceName string

	// InstanceURL is the base URL of the Perf instance UI, used to build links. Optional.
	InstanceURL string

	// Visibility is "internal" or "public".
	Visibility string

	// ExploreURL builds graph links. Optional.
	ExploreURL ExploreURLFunc

	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
}

// Service serves anomaly insights. It is safe for concurrent use.
type Service struct {
	stores Stores
	opts   Options

	mu     sync.Mutex
	subs   []*subpb.Subscription
	subsAt time.Time
}

// New returns a new Service.
func New(stores Stores, opts Options) (*Service, error) {
	if stores.Regressions == nil || stores.Subscriptions == nil {
		return nil, skerr.Fmt("the Regressions and Subscriptions stores are required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Visibility == "" {
		opts.Visibility = "internal"
	}
	if opts.InstanceName == "" {
		opts.InstanceName = "unknown"
	}
	return &Service{stores: stores, opts: opts}, nil
}

// Instance returns information about the Perf instance and the limits of the service.
func (s *Service) Instance() InstanceInfo {
	return InstanceInfo{
		Name:       s.opts.InstanceName,
		URL:        s.opts.InstanceURL,
		Visibility: s.opts.Visibility,
		Limits: Limits{
			DefaultPageSize: DefaultPageSize,
			MaxPageSize:     MaxPageSize,
			DefaultDays:     DefaultDays,
			MaxDays:         MaxDays,
			MaxScanRows:     MaxScanRows,
			MaxBatchGetIDs:  MaxBatchGetIDs,
			MaxFullViewIDs:  MaxFullViewIDs,
		},
	}
}

// Ping checks that the database is reachable.
func (s *Service) Ping(ctx context.Context) error {
	_, err := s.stores.Subscriptions.GetAllActiveSubscriptions(ctx)
	return err
}

// activeSubscriptions returns the active subscriptions, cached for subscriptionCacheTTL.
func (s *Service) activeSubscriptions(ctx context.Context) ([]*subpb.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.opts.Now()
	if s.subs != nil && now.Sub(s.subsAt) < subscriptionCacheTTL {
		return s.subs, nil
	}
	subs, err := s.stores.Subscriptions.GetAllActiveSubscriptions(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading active subscriptions")
	}
	if subs == nil {
		subs = []*subpb.Subscription{}
	}
	s.subs = subs
	s.subsAt = now
	return subs, nil
}

// checkSheriff validates that name is an active sheriff. On failure, the hint lists close
// matches so an agent can retry.
func (s *Service) checkSheriff(ctx context.Context, name string) error {
	if name == "" {
		return InvalidArgument("call list_sheriffs to get the exact names", "sheriff is required")
	}
	if err := validateName("sheriff", name); err != nil {
		return err
	}
	subs, err := s.activeSubscriptions(ctx)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub.Name == name {
			return nil
		}
		names = append(names, sub.Name)
	}
	sort.Strings(names)
	lower := strings.ToLower(name)
	suggestions := []string{}
	for _, n := range names {
		ln := strings.ToLower(n)
		if strings.Contains(ln, lower) || strings.Contains(lower, ln) {
			suggestions = append(suggestions, n)
		}
	}
	if len(suggestions) == 0 {
		suggestions = names
	}
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	hint := "call list_sheriffs to get the exact names"
	if len(suggestions) > 0 {
		hint = fmt.Sprintf("did you mean: %s? (call list_sheriffs to get all exact names)", strings.Join(suggestions, ", "))
	}
	return InvalidArgument(hint, "unknown sheriff")
}

// ListSheriffs returns the active sheriffs (subscriptions), sorted by name.
func (s *Service) ListSheriffs(ctx context.Context) (*ListSheriffsResponse, error) {
	subs, err := s.activeSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	ret := &ListSheriffsResponse{Sheriffs: make([]Sheriff, 0, len(subs))}
	for _, sub := range subs {
		ret.Sheriffs = append(ret.Sheriffs, Sheriff{
			Name:         sub.Name,
			Revision:     sub.Revision,
			BugComponent: sub.BugComponent,
			Contact:      sub.ContactEmail,
		})
	}
	sort.Slice(ret.Sheriffs, func(i, j int) bool { return ret.Sheriffs[i].Name < ret.Sheriffs[j].Name })
	return ret, nil
}

// scan reads the regressions of a sheriff newest first (creation_time DESC), starting at offset,
// and calls visit for each one. It stops when visit returns false, when a regression older than
// since is reached, when the store has no more rows, or after maxRows rows. It returns the offset
// of the next unread row, the number of rows read, whether there are no more matching rows, and
// whether the scan stopped because of maxRows.
//
// Including triaged rows is what makes the store sort newest first; status is filtered in memory.
func (s *Service) scan(ctx context.Context, sheriff string, includeImprovements bool, since time.Time, offset, maxRows int, visit func(*regression.Regression) bool) (int, int, bool, bool, error) {
	scanned := 0
	for scanned < maxRows {
		batch := scanBatchSize
		if maxRows-scanned < batch {
			batch = maxRows - scanned
		}
		regs, err := s.stores.Regressions.GetRegressionsBySubName(ctx, regression.GetAnomalyListRequest{
			SubName:             sheriff,
			IncludeTriaged:      true,
			IncludeImprovements: includeImprovements,
			PaginationOffset:    offset,
		}, batch)
		if err != nil {
			return offset, scanned, false, false, skerr.Wrapf(err, "reading regressions")
		}
		for _, r := range regs {
			if r == nil {
				offset++
				scanned++
				continue
			}
			if !since.IsZero() && r.CreationTime.Before(since) {
				return offset, scanned, true, false, nil
			}
			offset++
			scanned++
			if !visit(r) {
				return offset, scanned, false, false, nil
			}
		}
		if len(regs) < batch {
			return offset, scanned, true, false, nil
		}
	}
	return offset, scanned, false, true, nil
}

// matchesFilters returns true if the anomaly passes the list filters.
func matchesFilters(sum AnomalySummary, status, direction, benchmark, bot string, minAbsPct float64) bool {
	if status != StatusAny && sum.Triage.Status != status {
		return false
	}
	if direction != DirectionAny && sum.Direction != direction {
		return false
	}
	if benchmark != "" && sum.Test.Benchmark != benchmark {
		return false
	}
	if bot != "" && sum.Test.Bot != bot {
		return false
	}
	if minAbsPct > 0 && (sum.PctChange == nil || absPct(sum) < minAbsPct) {
		return false
	}
	return true
}

// ListAnomalies returns a sheriff's anomalies, newest first, filtered and paginated.
func (s *Service) ListAnomalies(ctx context.Context, req ListAnomaliesRequest) (*ListAnomaliesResponse, error) {
	req.Sheriff = strings.TrimSpace(req.Sheriff)
	if err := s.checkSheriff(ctx, req.Sheriff); err != nil {
		return nil, err
	}
	status, err := normalizeEnum("status", req.Status, StatusUntriaged, StatusUntriaged, StatusTriaged, StatusIgnored, StatusPending, StatusAny)
	if err != nil {
		return nil, err
	}
	direction, err := normalizeEnum("direction", req.Direction, DirectionRegression, DirectionRegression, DirectionImprovement, DirectionAny)
	if err != nil {
		return nil, err
	}
	if err := validateOptionalName("benchmark", req.Benchmark); err != nil {
		return nil, err
	}
	if err := validateOptionalName("bot", req.Bot); err != nil {
		return nil, err
	}
	days, err := normalizeDays(req.Days)
	if err != nil {
		return nil, err
	}
	pageSize, err := normalizePageSize(req.PageSize)
	if err != nil {
		return nil, err
	}
	if req.MinAbsPct < 0 || req.MinAbsPct > 1e6 {
		return nil, InvalidArgument("use a non-negative percentage, e.g. 2.5", "invalid min_abs_pct")
	}
	fh := filterHash(req.Sheriff, status, direction, req.Benchmark, req.Bot, strconv.Itoa(days), strconv.FormatFloat(req.MinAbsPct, 'g', -1, 64))
	offset, err := decodePageToken(req.PageToken, fh)
	if err != nil {
		return nil, err
	}

	since := s.opts.Now().Add(-time.Duration(days) * 24 * time.Hour)
	anomalies := []AnomalySummary{}
	next, scanned, done, hitLimit, err := s.scan(ctx, req.Sheriff, direction != DirectionRegression, since, offset, MaxScanRows, func(r *regression.Regression) bool {
		sum := s.toSummary(r)
		if !matchesFilters(sum, status, direction, req.Benchmark, req.Bot, req.MinAbsPct) {
			return true
		}
		anomalies = append(anomalies, sum)
		return len(anomalies) < pageSize
	})
	if err != nil {
		return nil, err
	}
	resp := &ListAnomaliesResponse{Anomalies: anomalies, TotalScanned: scanned, Truncated: hitLimit}
	if !done {
		resp.NextPageToken = encodePageToken(next, fh)
	}
	return resp, nil
}

// BatchGetAnomalies returns the anomalies with the given IDs, in the order requested.
func (s *Service) BatchGetAnomalies(ctx context.Context, ids []string, view View) (*BatchGetAnomaliesResponse, error) {
	if view == "" {
		view = ViewBasic
	}
	if view != ViewBasic && view != ViewFull {
		return nil, InvalidArgument("use view=basic or view=full", "invalid view")
	}
	max := MaxBatchGetIDs
	if view == ViewFull {
		max = MaxFullViewIDs
	}
	ids, err := normalizeIDs("ids", ids, max)
	if err != nil {
		return nil, err
	}
	regs, err := s.stores.Regressions.GetByIDs(ctx, ids)
	if err != nil {
		return nil, skerr.Wrapf(err, "reading regressions by id")
	}
	if view == ViewFull && len(regs) > 0 {
		// Adds the bugs filed by auto-triage and auto-bisection to the manually assigned ones.
		if withBugs, err := s.stores.Regressions.GetBugIdsForRegressions(ctx, regs); err != nil {
			sklog.Warningf("Failed to read bugs for regressions: %s", err)
		} else if withBugs != nil {
			regs = withBugs
		}
	}
	byID := make(map[string]*regression.Regression, len(regs))
	for _, r := range regs {
		if r != nil {
			byID[r.Id] = r
		}
	}
	resp := &BatchGetAnomaliesResponse{Anomalies: []AnomalyDetail{}}
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			resp.NotFound = append(resp.NotFound, id)
			continue
		}
		resp.Anomalies = append(resp.Anomalies, s.toDetail(ctx, r, view))
	}
	return resp, nil
}

// toDetail converts a regression into an AnomalyDetail.
func (s *Service) toDetail(ctx context.Context, reg *regression.Regression, view View) AnomalyDetail {
	d := AnomalyDetail{AnomalySummary: s.toSummary(reg)}
	normalized := d.NumTraces > 1
	if s.stores.Alerts != nil && reg.AlertId != 0 {
		if a, err := s.stores.Alerts.GetAlertConfig(reg.AlertId); err != nil {
			sklog.Warningf("Failed to read alert %d: %s", reg.AlertId, err)
		} else if a != nil {
			d.Alert = alertInfoOf(reg.AlertId, a)
			normalized = d.Alert.Algo == string(types.KMeansGrouping)
		}
	}
	d.Window = windowOf(reg, normalized)
	if keys := traceKeys(reg); len(keys) > 1 {
		others := keys[1:]
		if len(others) > maxOtherTraces {
			others = others[:maxOtherTraces]
		}
		d.OtherTraces = others
	}
	if view == ViewFull {
		d.Commits = s.commitsFor(ctx, d.CommitRange)
		if s.opts.ExploreURL != nil && d.Test.TraceKey != "" {
			if params, err := parseParams(d.Test.TraceKey); err == nil {
				d.Links.Graph = s.opts.ExploreURL(ctx, int(d.CommitRange.Start), int(d.CommitRange.End), params)
			}
		}
	}
	return d
}

// commitsFor resolves the commits in an anomaly's commit range. Errors are logged and the
// commits are left out, since they are best-effort context.
func (s *Service) commitsFor(ctx context.Context, cr CommitRange) *CommitsInfo {
	if s.stores.Commits == nil || cr.End < 0 || cr.Start > cr.End {
		return nil
	}
	ret := &CommitsInfo{InRange: []CommitInfo{}, Total: int(cr.End - cr.Start + 1)}
	if c, err := s.stores.Commits.CommitFromCommitNumber(ctx, types.CommitNumber(cr.Start)); err == nil {
		ci := commitInfoOf(c)
		ret.Start = &ci
	}
	if c, err := s.stores.Commits.CommitFromCommitNumber(ctx, types.CommitNumber(cr.End)); err == nil {
		ci := commitInfoOf(c)
		ret.End = &ci
	}
	begin := cr.Start
	if ret.Total > MaxCommitsInRange {
		begin = cr.End - MaxCommitsInRange + 1
		ret.Truncated = true
	}
	commits, err := s.stores.Commits.CommitSliceFromCommitNumberRange(ctx, types.CommitNumber(begin), types.CommitNumber(cr.End))
	if err != nil {
		sklog.Warningf("Failed to read commits [%d, %d]: %s", begin, cr.End, err)
		return ret
	}
	for _, c := range commits {
		ret.InRange = append(ret.InRange, commitInfoOf(c))
	}
	return ret
}

// parseParams turns a trace key into explore-page query params.
func parseParams(key string) (map[string][]string, error) {
	params, err := parseKey(key)
	if err != nil {
		return nil, err
	}
	ret := make(map[string][]string, len(params))
	for k, v := range params {
		ret[k] = []string{v}
	}
	return ret, nil
}

// GetAnomalyGroup returns an anomaly group with summaries of its anomalies and its culprits.
func (s *Service) GetAnomalyGroup(ctx context.Context, id string) (*AnomalyGroupResponse, error) {
	if s.stores.AnomalyGroups == nil {
		return nil, NotFound("", "anomaly groups are not available on this instance")
	}
	ids, err := normalizeIDs("id", []string{id}, 1)
	if err != nil {
		return nil, err
	}
	group, err := s.stores.AnomalyGroups.LoadById(ctx, ids[0])
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, NotFound("anomaly group ids come from get_anomalies or the Perf UI", "anomaly group not found")
		}
		return nil, skerr.Wrapf(err, "loading anomaly group")
	}
	resp := &AnomalyGroupResponse{
		ID:              group.GroupId,
		Action:          group.GroupAction.String(),
		Sheriff:         group.SubsciptionName,
		SheriffRevision: group.SubscriptionRevision,
		Benchmark:       group.BenchmarkName,
		ReportedIssueID: group.ReportedIssueId,
		NumAnomalies:    len(group.AnomalyIds),
		Anomalies:       []AnomalySummary{},
		Culprits:        []Culprit{},
	}
	memberIDs := validIDs(group.AnomalyIds)
	if len(memberIDs) > MaxGroupMembers {
		memberIDs = memberIDs[:MaxGroupMembers]
		resp.Truncated = true
	}
	if len(memberIDs) > 0 {
		regs, err := s.stores.Regressions.GetByIDs(ctx, memberIDs)
		if err != nil {
			return nil, skerr.Wrapf(err, "reading the group's regressions")
		}
		for _, r := range regs {
			if r != nil {
				resp.Anomalies = append(resp.Anomalies, s.toSummary(r))
			}
		}
		sortBySeverity(resp.Anomalies)
	}
	culpritIDs := validIDs(group.CulpritIds)
	if s.stores.Culprits != nil && len(culpritIDs) > 0 {
		culprits, err := s.stores.Culprits.Get(ctx, culpritIDs)
		if err != nil {
			sklog.Warningf("Failed to read culprits for group %s: %s", group.GroupId, err)
		}
		for _, c := range culprits {
			if c == nil {
				continue
			}
			cu := Culprit{ID: c.Id, IssueIDs: c.IssueIds}
			if c.Commit != nil {
				cu.Host, cu.Project, cu.Ref, cu.Revision = c.Commit.Host, c.Commit.Project, c.Commit.Ref, c.Commit.Revision
			}
			resp.Culprits = append(resp.Culprits, cu)
		}
	}
	return resp, nil
}

// FindAnomaliesForRevision returns the anomalies whose commit range contains the revision, which
// is a commit number or a full git hash.
func (s *Service) FindAnomaliesForRevision(ctx context.Context, rev string) (*RevisionAnomaliesResponse, error) {
	rev = strings.ToLower(strings.TrimSpace(rev))
	revHint := "pass a commit number (e.g. 1234567) or a full 40-character git hash"
	var commitNumber int64
	switch {
	case commitNumberRE.MatchString(rev):
		n, err := strconv.ParseInt(rev, 10, 64)
		if err != nil {
			return nil, InvalidArgument(revHint, "invalid revision")
		}
		commitNumber = n
	case gitHashRE.MatchString(rev):
		if s.stores.Commits == nil {
			return nil, InvalidArgument("pass a commit number instead", "git hashes are not supported on this instance")
		}
		n, err := s.stores.Commits.CommitNumberFromGitHash(ctx, rev)
		if err != nil {
			return nil, NotFound("check that the hash is on the tracked branch, or pass a commit number", "git hash not found")
		}
		commitNumber = int64(n)
	default:
		return nil, InvalidArgument(revHint, "invalid revision")
	}
	regs, err := s.stores.Regressions.GetByRevision(ctx, strconv.FormatInt(commitNumber, 10))
	if err != nil {
		return nil, skerr.Wrapf(err, "reading regressions by revision")
	}
	resp := &RevisionAnomaliesResponse{Revision: commitNumber, Anomalies: []AnomalySummary{}}
	for _, r := range regs {
		if r != nil {
			resp.Anomalies = append(resp.Anomalies, s.toSummary(r))
		}
	}
	sortBySeverity(resp.Anomalies)
	if len(resp.Anomalies) > MaxRevisionResults {
		resp.Anomalies = resp.Anomalies[:MaxRevisionResults]
		resp.Truncated = true
	}
	return resp, nil
}

// Summarize returns aggregated insights about a sheriff's anomalies over the last days.
func (s *Service) Summarize(ctx context.Context, req SummarizeRequest) (*InsightSummary, error) {
	req.Sheriff = strings.TrimSpace(req.Sheriff)
	if err := s.checkSheriff(ctx, req.Sheriff); err != nil {
		return nil, err
	}
	days, err := normalizeDays(req.Days)
	if err != nil {
		return nil, err
	}
	since := s.opts.Now().Add(-time.Duration(days) * 24 * time.Hour)
	sums := []AnomalySummary{}
	_, scanned, _, hitLimit, err := s.scan(ctx, req.Sheriff, true, since, 0, MaxScanRows, func(r *regression.Regression) bool {
		sums = append(sums, s.toSummary(r))
		return true
	})
	if err != nil {
		return nil, err
	}
	ret := summarize(sums)
	ret.Scope = SummaryScope{
		Sheriff:      req.Sheriff,
		Days:         days,
		Since:        since.UTC(),
		TotalScanned: scanned,
		Truncated:    hitLimit,
	}
	return ret, nil
}
