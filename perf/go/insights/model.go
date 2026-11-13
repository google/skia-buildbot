// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import "time"

// Triage statuses, as stored in Regressions2.triage_status, plus StatusAny for filtering.
const (
	StatusUntriaged = "untriaged"
	StatusTriaged   = "triaged"
	StatusIgnored   = "ignored"
	StatusPending   = "pending"
	StatusAny       = "any"
)

// Directions of an anomaly, plus DirectionAny for filtering.
const (
	DirectionRegression  = "regression"
	DirectionImprovement = "improvement"
	DirectionAny         = "any"
)

// View controls how much detail BatchGetAnomalies returns.
type View string

const (
	// ViewBasic returns the summary, the stored value window and the alert config.
	ViewBasic View = "basic"

	// ViewFull also resolves the commits in the anomaly's range and a graph link, which costs
	// extra database reads per anomaly.
	ViewFull View = "full"
)

// InstanceInfo describes the Perf instance whose data the service serves.
type InstanceInfo struct {
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
	Visibility string `json:"visibility"`
	Limits     Limits `json:"limits"`
}

// Limits are the bounds enforced by the service.
type Limits struct {
	DefaultPageSize int `json:"default_page_size"`
	MaxPageSize     int `json:"max_page_size"`
	DefaultDays     int `json:"default_days"`
	MaxDays         int `json:"max_days"`
	MaxScanRows     int `json:"max_scan_rows"`
	MaxBatchGetIDs  int `json:"max_batch_get_ids"`
	MaxFullViewIDs  int `json:"max_full_view_ids"`
}

// Sheriff is an active subscription (sheriff config) that anomalies are reported to.
type Sheriff struct {
	Name         string `json:"name"`
	Revision     string `json:"revision,omitempty"`
	BugComponent string `json:"bug_component,omitempty"`
	Contact      string `json:"contact,omitempty"`
}

// ListSheriffsResponse is the response of ListSheriffs.
type ListSheriffsResponse struct {
	Sheriffs []Sheriff `json:"sheriffs"`
}

// TestInfo describes the trace an anomaly was detected on.
type TestInfo struct {
	// TestPath is master/bot/benchmark/test/subtests, or the raw trace key if it has none.
	TestPath  string   `json:"test_path"`
	Master    string   `json:"master,omitempty"`
	Bot       string   `json:"bot,omitempty"`
	Benchmark string   `json:"benchmark,omitempty"`
	Test      string   `json:"test,omitempty"`
	Subtests  []string `json:"subtests,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	TraceKey  string   `json:"trace_key,omitempty"`
}

// CommitRange is the range of commits that may contain the change.
type CommitRange struct {
	// Start is the first commit that may contain the change (the previous data point + 1).
	Start int64 `json:"start"`
	// End is the commit of the first data point after the change.
	End int64 `json:"end"`
	// Display is the commit the UI highlights.
	Display int64 `json:"display"`
}

// Triage is the triage state of an anomaly.
type Triage struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	BugIDs  []string `json:"bug_ids"`
}

// Links are URLs into the Perf UI.
type Links struct {
	Report string `json:"report,omitempty"`
	Graph  string `json:"graph,omitempty"`
}

// AnomalySummary is the compact, LLM-friendly view of an anomaly.
type AnomalySummary struct {
	ID string `json:"id"`
	// Headline is a deterministic one-line description of the anomaly.
	Headline  string   `json:"headline"`
	Sheriff   string   `json:"sheriff,omitempty"`
	Test      TestInfo `json:"test"`
	NumTraces int      `json:"num_traces"`
	// Direction is "regression" or "improvement".
	Direction    string  `json:"direction"`
	MedianBefore float64 `json:"median_before"`
	MedianAfter  float64 `json:"median_after"`
	// PctChange is the change of the median in percent, or null if median_before is 0.
	PctChange   *float64    `json:"pct_change"`
	AbsDelta    float64     `json:"abs_delta"`
	CommitRange CommitRange `json:"commit_range"`
	DetectedAt  time.Time   `json:"detected_at"`
	Triage      Triage      `json:"triage"`
	AlertID     int64       `json:"alert_id"`
	Links       Links       `json:"links"`
}

// WindowPoint is one data point around the anomaly.
type WindowPoint struct {
	Commit    int64   `json:"commit"`
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"value"`
}

// ValueWindow is the data around the anomaly, as stored with the regression when it was
// detected.
type ValueWindow struct {
	// Normalized is true when the values are a normalized k-means centroid rather than raw
	// measurements.
	Normalized bool          `json:"normalized"`
	StepAt     int64         `json:"step_at,omitempty"`
	Points     []WindowPoint `json:"points"`
	Truncated  bool          `json:"truncated,omitempty"`
}

// AlertInfo is the alert config that detected the anomaly; it explains why it was flagged.
type AlertInfo struct {
	ID               int64   `json:"id"`
	DisplayName      string  `json:"display_name,omitempty"`
	Query            string  `json:"query"`
	Algo             string  `json:"algo"`
	Step             string  `json:"step"`
	Radius           int     `json:"radius"`
	Threshold        float32 `json:"threshold"`
	Direction        string  `json:"direction"`
	GroupBy          string  `json:"group_by,omitempty"`
	MinimumNum       int     `json:"minimum_num,omitempty"`
	Action           string  `json:"action,omitempty"`
	HasDetectionRule bool    `json:"has_detection_rule,omitempty"`
}

// CommitInfo describes a commit.
type CommitInfo struct {
	Number    int64  `json:"number"`
	Hash      string `json:"hash"`
	Author    string `json:"author,omitempty"`
	Subject   string `json:"subject"`
	URL       string `json:"url,omitempty"`
	Timestamp int64  `json:"ts"`
}

// CommitsInfo lists the commits in the anomaly's commit range: the likely suspects.
type CommitsInfo struct {
	Start     *CommitInfo  `json:"start,omitempty"`
	End       *CommitInfo  `json:"end,omitempty"`
	InRange   []CommitInfo `json:"in_range"`
	Total     int          `json:"total"`
	Truncated bool         `json:"truncated"`
}

// AnomalyDetail is the full view of an anomaly.
type AnomalyDetail struct {
	AnomalySummary
	Window      *ValueWindow `json:"window,omitempty"`
	Alert       *AlertInfo   `json:"alert,omitempty"`
	Commits     *CommitsInfo `json:"commits,omitempty"`
	OtherTraces []string     `json:"other_traces,omitempty"`
}

// ListAnomaliesRequest are the parameters of ListAnomalies.
type ListAnomaliesRequest struct {
	Sheriff   string
	Status    string
	Direction string
	Benchmark string
	Bot       string
	Days      int
	MinAbsPct float64
	PageSize  int
	PageToken string
}

// ListAnomaliesResponse is the response of ListAnomalies.
type ListAnomaliesResponse struct {
	Anomalies     []AnomalySummary `json:"anomalies"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	TotalScanned  int              `json:"total_scanned"`
	// Truncated is true when the scan limit was reached before the page was full.
	Truncated bool `json:"truncated"`
}

// BatchGetAnomaliesResponse is the response of BatchGetAnomalies.
type BatchGetAnomaliesResponse struct {
	Anomalies []AnomalyDetail `json:"anomalies"`
	NotFound  []string        `json:"not_found,omitempty"`
}

// Culprit is a commit found by bisection to cause an anomaly group's regressions.
type Culprit struct {
	ID       string   `json:"id"`
	Host     string   `json:"host,omitempty"`
	Project  string   `json:"project,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Revision string   `json:"revision"`
	IssueIDs []string `json:"issue_ids,omitempty"`
}

// AnomalyGroupResponse is the response of GetAnomalyGroup.
type AnomalyGroupResponse struct {
	ID              string           `json:"id"`
	Action          string           `json:"action"`
	Sheriff         string           `json:"sheriff,omitempty"`
	SheriffRevision string           `json:"sheriff_revision,omitempty"`
	Benchmark       string           `json:"benchmark,omitempty"`
	ReportedIssueID int64            `json:"reported_issue_id,omitempty"`
	NumAnomalies    int              `json:"num_anomalies"`
	Anomalies       []AnomalySummary `json:"anomalies"`
	Culprits        []Culprit        `json:"culprits"`
	Truncated       bool             `json:"truncated"`
}

// RevisionAnomaliesResponse is the response of FindAnomaliesForRevision.
type RevisionAnomaliesResponse struct {
	Revision  int64            `json:"revision"`
	Anomalies []AnomalySummary `json:"anomalies"`
	Truncated bool             `json:"truncated"`
}

// SummarizeRequest are the parameters of Summarize.
type SummarizeRequest struct {
	Sheriff string
	Days    int
}

// SummaryScope describes the data a summary was computed from.
type SummaryScope struct {
	Sheriff      string    `json:"sheriff"`
	Days         int       `json:"days"`
	Since        time.Time `json:"since"`
	TotalScanned int       `json:"total_scanned"`
	Truncated    bool      `json:"truncated"`
}

// SummaryTotals counts anomalies by direction and triage status.
type SummaryTotals struct {
	Regressions  int `json:"regressions"`
	Improvements int `json:"improvements"`
	Untriaged    int `json:"untriaged"`
	Triaged      int `json:"triaged"`
	Ignored      int `json:"ignored"`
	Pending      int `json:"pending"`
	Other        int `json:"other"`
}

// BenchmarkCount counts regressions per benchmark.
type BenchmarkCount struct {
	Benchmark       string   `json:"benchmark"`
	Regressions     int      `json:"regressions"`
	MedianPctChange *float64 `json:"median_pct_change,omitempty"`
}

// BotCount counts regressions per bot.
type BotCount struct {
	Bot         string `json:"bot"`
	Regressions int    `json:"regressions"`
}

// OverlapCluster is a set of regressions whose commit ranges overlap, so they may share a cause.
type OverlapCluster struct {
	// CommitRange is the intersection of the member ranges: [start, end].
	CommitRange [2]int64 `json:"commit_range"`
	Count       int      `json:"count"`
	Benchmarks  []string `json:"benchmarks"`
	AnomalyIDs  []string `json:"anomaly_ids"`
}

// RepeatOffender is a test with several regressions in the period.
type RepeatOffender struct {
	TestPath  string `json:"test_path"`
	Anomalies int    `json:"anomalies"`
}

// InsightSummary is the response of Summarize.
type InsightSummary struct {
	Scope             SummaryScope     `json:"scope"`
	Totals            SummaryTotals    `json:"totals"`
	TopRegressions    []AnomalySummary `json:"top_regressions"`
	ByBenchmark       []BenchmarkCount `json:"by_benchmark"`
	ByBot             []BotCount       `json:"by_bot"`
	OverlappingRanges []OverlapCluster `json:"overlapping_ranges"`
	RepeatOffenders   []RepeatOffender `json:"repeat_offenders"`
}

// AnomalyTraceRequest are the parameters of GetAnomalyTrace.
type AnomalyTraceRequest struct {
	ID string

	// CommitsBefore and CommitsAfter are how many commits to read before and after the
	// anomaly's commit range. 0 means the default.
	CommitsBefore int
	CommitsAfter  int
}

// TracePoint is one measurement of a trace.
type TracePoint struct {
	Commit int64   `json:"commit"`
	Value  float64 `json:"value"`
}

// AnomalyTraceResponse is the response of GetAnomalyTrace: the measurements of the anomaly's trace
// around its commit range, read from the trace store.
type AnomalyTraceResponse struct {
	Anomaly  AnomalySummary `json:"anomaly"`
	TraceKey string         `json:"trace_key"`

	// Begin and End are the commits the trace was read for, inclusive.
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`

	// Points are the measurements in commit order. Commits without data are left out.
	Points []TracePoint `json:"points"`

	// OtherTraces is the number of other traces in the anomaly's cluster, which aren't included.
	OtherTraces int `json:"other_traces,omitempty"`
}

// Triage suggestion actions.
const (
	// ActionFileOneBug: anomalies whose commit ranges intersect may share one cause, so one bug (or
	// one bisection) can cover them.
	ActionFileOneBug = "file_one_bug"

	// ActionFileBug: a clean, large regression worth its own bug.
	ActionFileBug = "file_bug"

	// ActionIgnoreLikelyNoise: weak steps, or changes that went back to the level before them.
	ActionIgnoreLikelyNoise = "ignore_likely_noise"

	// ActionReviewImprovements: untriaged improvements, usually acknowledged or ignored.
	ActionReviewImprovements = "review_improvements"

	// ActionReview: the remaining regressions, for a person to look at.
	ActionReview = "review"
)

// SuggestTriageRequest are the parameters of SuggestTriage.
type SuggestTriageRequest struct {
	Sheriff string
	Days    int
}

// BugDraft is a suggested bug title and description.
type BugDraft struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TriageSuggestion is one proposed triage action for a set of anomalies, computed with simple
// heuristics. People review it and apply triage in the Perf UI, at TriageURL.
type TriageSuggestion struct {
	Action string `json:"action"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`

	// AnomalyIDs are the anomalies, biggest change first, up to MaxSuggestionIDs of them.
	AnomalyIDs []string `json:"anomaly_ids"`

	// Representative is the anomaly with the biggest change.
	Representative *AnomalySummary `json:"representative,omitempty"`

	// CommitRange is the commit range all the anomalies share, for file_one_bug.
	CommitRange *[2]int64 `json:"commit_range,omitempty"`

	Benchmarks []string `json:"benchmarks,omitempty"`
	Bots       []string `json:"bots,omitempty"`

	// TriageURL opens the anomalies in the Perf UI, where they can be triaged.
	TriageURL string `json:"triage_url,omitempty"`

	// BugDraft is set for file_one_bug and file_bug.
	BugDraft *BugDraft `json:"bug_draft,omitempty"`
}

// TriagePlan is the response of SuggestTriage.
type TriagePlan struct {
	Scope       SummaryScope       `json:"scope"`
	Untriaged   int                `json:"untriaged"`
	Suggestions []TriageSuggestion `json:"suggestions"`

	// Note explains how the suggestions were made.
	Note string `json:"note"`
}
