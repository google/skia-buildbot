// Package frontend houses a variety of types that represent how the frontend
// expects the format of data. The data types here are those shared by
// multiple packages.
package frontend

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

const urlPlaceholder = "%s"

// ChangeList encapsulates how the frontend expects to get information
// about a code_review.ChangeList that has Gold results associated with it.
// We have a separate struct so we can decouple the JSON representation
// and the backend representation (if it needs changing or use by another project
// with its own JSON requirements).
type ChangeList struct {
	System   string    `json:"system"`
	SystemID string    `json:"id"`
	Owner    string    `json:"owner"`
	Status   string    `json:"status"`
	Subject  string    `json:"subject"`
	Updated  time.Time `json:"updated"`
	URL      string    `json:"url"`
}

// ConvertChangeList turns a code_review.ChangeList into a ChangeList for the frontend.
func ConvertChangeList(cl code_review.ChangeList, system, urlTempl string) ChangeList {
	return ChangeList{
		System:   system,
		SystemID: cl.SystemID,
		Owner:    cl.Owner,
		Status:   cl.Status.String(),
		Subject:  cl.Subject,
		Updated:  cl.Updated,
		URL:      strings.Replace(urlTempl, urlPlaceholder, cl.SystemID, 1),
	}
}

// ChangeListSummary encapsulates how the frontend expects to get a summary of
// the TryJob information we have associated with a given ChangeList. These
// TryJobs are those we've noticed that uploaded results to Gold.
type ChangeListSummary struct {
	CL ChangeList `json:"cl"`
	// these are only those patchsets with data.
	PatchSets         []PatchSet `json:"patch_sets"`
	NumTotalPatchSets int        `json:"num_total_patch_sets"`
}

// PatchSet represents the data the frontend needs for PatchSets.
type PatchSet struct {
	SystemID string   `json:"id"`
	Order    int      `json:"order"`
	TryJobs  []TryJob `json:"try_jobs"`
}

// TryJob represents the data the frontend needs for TryJobs.
type TryJob struct {
	SystemID    string    `json:"id"`
	DisplayName string    `json:"name"`
	Updated     time.Time `json:"updated"`
	System      string    `json:"system"`
	URL         string    `json:"url"`
}

// ConvertTryJob turns a ci.TryJob into a TryJob for the frontend.
func ConvertTryJob(tj ci.TryJob, urlTempl string) TryJob {
	return TryJob{
		System:      tj.System,
		SystemID:    tj.SystemID,
		DisplayName: tj.DisplayName,
		Updated:     tj.Updated,
		URL:         strings.Replace(urlTempl, urlPlaceholder, tj.SystemID, 1),
	}
}

// TriageRequest is the form of the JSON posted by the frontend when triaging
// (both single and bulk).
type TriageRequest struct {
	// TestDigestStatus maps status to test name and digests. The strings are
	// expectation.Label.String() values
	TestDigestStatus map[types.TestName]map[types.Digest]expectations.Label `json:"testDigestStatus"`

	// ChangeListID is the id of the ChangeList for which we want to change the expectations.
	// "issue" is the JSON field for backwards compatibility.
	ChangeListID string `json:"issue"`

	// CodeReviewSystem is the id of the crs that the ChangeListID belongs. If ChangeListID is set,
	// CodeReviewSystem should be also.
	CodeReviewSystem string `json:"crs"`

	// ImageMatchingAlgorithm is the name of the non-exact image matching algorithm requesting the
	// triage (see http://go/gold-non-exact-matching). If set, the algorithm name will be used as
	// the author of the triage action.
	//
	// An empty image matching algorithm indicates this is a manual triage operation, in which case
	// the username that initiated the triage operation via Gold's UI will be used as the author of
	// the operation.
	ImageMatchingAlgorithm string `json:"imageMatchingAlgorithm"`
}

// TriageDelta represents one changed digest and the label that was
// assigned as part of the triage operation.
type TriageDelta struct {
	TestName types.TestName     `json:"test_name"`
	Digest   types.Digest       `json:"digest"`
	Label    expectations.Label `json:"label"`
}

// TriageLogEntry represents a set of changes by a single person.
type TriageLogEntry struct {
	ID          string        `json:"id"`
	User        string        `json:"name"`
	TS          int64         `json:"ts"` // is milliseconds since the epoch
	ChangeCount int           `json:"changeCount"`
	Details     []TriageDelta `json:"details"`
}

// ConvertLogEntry turns an expectations.TriageLogEntry into its frontend representation.
func ConvertLogEntry(entry expectations.TriageLogEntry) TriageLogEntry {
	tle := TriageLogEntry{
		ID:          entry.ID,
		User:        entry.User,
		TS:          entry.TS.Unix() * 1000,
		ChangeCount: entry.ChangeCount,
	}
	for _, d := range entry.Details {
		tle.Details = append(tle.Details, TriageDelta{
			TestName: d.Grouping,
			Digest:   d.Digest,
			Label:    d.Label,
		})
	}
	return tle
}

// DigestListResponse is the response for "what digests belong to..."
type DigestListResponse struct {
	Digests []types.Digest `json:"digests"`
}

// IgnoreRule represents an ignore.Rule as well as how many times the rule
// was applied. This allows for the decoupling of the rule as stored in the
// DB from how we present it to the UI.
type IgnoreRule struct {
	ID          string              `json:"id"`
	CreatedBy   string              `json:"name"` // TODO(kjlubick) rename this on the frontend.
	UpdatedBy   string              `json:"updatedBy"`
	Expires     time.Time           `json:"expires"`
	Query       string              `json:"query"`
	ParsedQuery map[string][]string `json:"-"`
	Note        string              `json:"note"`
	// Count represents how many traces are affected by this ignore rule.
	Count int `json:"countAll"`
	// ExclusiveCount represents how many traces are affected *exclusively* by this ignore rule,
	// that is, they are only matched by this rule.
	ExclusiveCount int `json:"exclusiveCountAll"`
	// UntriagedCount represents how many traces with an untriaged digest at HEAD are affected
	// by this ignore rule.
	UntriagedCount int `json:"count"`
	// ExclusiveUntriagedCount represents how many traces with an untriaged digest at HEAD are
	// affected *exclusively* by this ignore rule, that is, they are only matched by this rule.
	ExclusiveUntriagedCount int `json:"exclusiveCount"`
}

// ConvertIgnoreRule converts a backend ignore.Rule into its frontend
// counterpart.
func ConvertIgnoreRule(r ignore.Rule) (IgnoreRule, error) {
	pq, err := url.ParseQuery(r.Query)
	if err != nil {
		return IgnoreRule{}, skerr.Wrapf(err, "invalid rule id %q; query %q", r.ID, r.Query)
	}
	return IgnoreRule{
		ID:          r.ID,
		CreatedBy:   r.CreatedBy,
		UpdatedBy:   r.UpdatedBy,
		Expires:     r.Expires,
		Query:       r.Query,
		ParsedQuery: pq,
		Note:        r.Note,
	}, nil
}

// IgnoreRuleBody encapsulates a single ignore rule that is submitted for addition or update.
type IgnoreRuleBody struct {
	// Duration is a human readable string like "2w", "4h" to specify a duration.
	Duration string `json:"duration"`
	// Filter is a url-encoded set of key-value pairs that can be used to match traces.
	// For example: "config=angle_d3d9_es2&cpu_or_gpu_value=RadeonHD7770"
	// Filter is limited to 10 KB.
	Filter string `json:"filter"`
	// Note is a short comment by a developer, typically a bug. Note is limited to 1 KB.
	Note string `json:"note"`
}

// MostRecentPositiveDigestResponse is the response for /json/latestpositivedigest.
type MostRecentPositiveDigestResponse struct {
	Digest types.Digest `json:"digest"`
}

// GetPerTraceDigestsByTestNameResponse is the response for /json/digestsbytestname.
type GetPerTraceDigestsByTestNameResponse map[tiling.TraceID][]types.Digest

// Commit represents a git Commit for use on the frontend.
type Commit struct {
	// CommitTime is in seconds since the epoch
	CommitTime    int64  `json:"commit_time"`
	Hash          string `json:"hash"` // For CLs, this is the CL ID.
	Author        string `json:"author"`
	Subject       string `json:"message"`
	ChangeListURL string `json:"cl_url"`
}

// FromTilingCommit converts a tiling.Commit into a frontend.Commit.
func FromTilingCommit(tc tiling.Commit) Commit {
	return Commit{
		CommitTime: tc.CommitTime.Unix(),
		Hash:       tc.Hash,
		Author:     tc.Author,
		Subject:    tc.Subject,
	}
}

// FromTilingCommits converts a slice of tiling.Commit into a slice of frontend.Commit.
func FromTilingCommits(xtc []tiling.Commit) []Commit {
	rv := make([]Commit, len(xtc))
	for i, tc := range xtc {
		rv[i] = FromTilingCommit(tc)
	}
	return rv
}

// ByBlameEntry is a helper structure that is serialized to
// JSON and sent to the front-end.
type ByBlameEntry struct {
	GroupID       string       `json:"groupID"`
	NDigests      int          `json:"nDigests"`
	NTests        int          `json:"nTests"`
	AffectedTests []TestRollup `json:"affectedTests"`
	Commits       []Commit     `json:"commits"`
}

// ByBlame describes a single digest and its blames.
type ByBlame struct {
	Test          types.TestName          `json:"test"`
	Digest        types.Digest            `json:"digest"`
	Blame         blame.BlameDistribution `json:"blame"`
	CommitIndices []int                   `json:"commit_indices"`
	Key           string
}

type TestRollup struct {
	Test         types.TestName `json:"test"`
	Num          int            `json:"num"`
	SampleDigest types.Digest   `json:"sample_digest"`
}

type FlakyTrace struct {
	ID            tiling.TraceID `json:"trace_id"`
	UniqueDigests int            `json:"unique_digests_count"`
}

// FlakyTracesDataResponse represents the data needed to identify flaky traces. This data is
// based on the current sliding window of commits.
type FlakyTracesDataResponse struct {
	// Traces represents all traces that are flaky given the input parameters. This slice will be
	// limited to the first 10k results and sorted high to low by UniqueDigests.
	Traces []FlakyTrace `json:"traces"`
	// TileSize is the number of commits in the sliding window.
	TileSize int `json:"tile_size"`
	// TotalFlakyTraces is the number of traces that are flaky given the input parameters. This
	// number may be greater than the length of Traces, if the total was greater than 10k.
	TotalFlakyTraces int `json:"num_flaky"`
	// TotalTraces is the total number of traces in the current sliding window.
	TotalTraces int `json:"num_traces"`
}

// ListTestsQuery encapsulates the inputs to ListTestsHandler.
type ListTestsQuery struct {
	Corpus                           string
	TraceValues                      paramtools.ParamSet
	OnlyIncludeDigestsProducedAtHead bool
	IgnoreState                      types.IgnoreState
}

// ParseListTestsQuery returns a ListTestsQuery by parsing the given request or error if the
// inputs are invalid.
func ParseListTestsQuery(r *http.Request) (ListTestsQuery, error) {
	if err := r.ParseForm(); err != nil {
		return ListTestsQuery{}, skerr.Wrapf(err, "parsing form")
	}

	ltq := ListTestsQuery{}
	ltq.Corpus = r.FormValue("corpus")
	if ltq.Corpus == "" {
		return ListTestsQuery{}, skerr.Fmt("must include corpus")
	}
	ltq.OnlyIncludeDigestsProducedAtHead = r.FormValue("at_head_only") == "true"

	if r.FormValue("include_ignored_traces") == "true" {
		ltq.IgnoreState = types.IncludeIgnoredTraces
	} else {
		ltq.IgnoreState = types.ExcludeIgnoredTraces
	}

	validate := shared.Validation{}
	ltq.TraceValues = validate.QueryFormValue(r, "trace_values")

	if err := validate.Errors(); err != nil {
		return ListTestsQuery{}, skerr.Wrapf(err, "validating params")
	}
	return ltq, nil
}

// TestSummary summarizes the digest count for a given test (and a series of search params).
type TestSummary struct {
	Name             types.TestName `json:"name"`
	PositiveDigests  int            `json:"positive_digests"`
	NegativeDigests  int            `json:"negative_digests"`
	UntriagedDigests int            `json:"untriaged_digests"`
}
