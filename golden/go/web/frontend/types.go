// Package frontend houses a variety of types that represent how the frontend
// expects the format of data. The data types here are those shared by
// multiple packages.
package frontend

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/validation"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// RefClosest is effectively an enum of two values - positive/negative.
type RefClosest string

const (
	// PositiveRef identifies the diff to the closest positive digest.
	PositiveRef = RefClosest("pos")

	// NegativeRef identifies the diff to the closest negative digest.
	NegativeRef = RefClosest("neg")

	// NoRef indicates no other digests match.
	NoRef = RefClosest("")

	urlPlaceholder = "%s"
)

// Define common routes used by multiple servers and goldctl
const (
	// ExpectationsRouteV2 serves the expectations of the master branch. If a changelist ID is
	// provided via the "issue" GET parameter, the expectations associated with that CL will be
	// merged onto the returned baseline.
	ExpectationsRouteV2 = "/json/v2/expectations"

	// KnownHashesRoute serves the list of known hashes.
	KnownHashesRoute   = "/json/hashes"
	KnownHashesRouteV1 = "/json/v1/hashes"
)

// Changelist encapsulates how the frontend expects to get information
// about a code_review.Changelist that has Gold results associated with it.
// We have a separate struct so we can decouple the JSON representation
// and the backend representation (if it needs changing or use by another project
// with its own JSON requirements).
type Changelist struct {
	System   string    `json:"system"`
	SystemID string    `json:"id"`
	Owner    string    `json:"owner"`
	Status   string    `json:"status"`
	Subject  string    `json:"subject"`
	Updated  time.Time `json:"updated"`
	URL      string    `json:"url"`
}

// ChangelistsResponse is the response for /json/v1/changelists.
type ChangelistsResponse struct {
	Changelists []Changelist `json:"changelists"`
	httputils.ResponsePagination
}

// ConvertChangelist turns a code_review.Changelist into a Changelist for the frontend.
func ConvertChangelist(cl code_review.Changelist, system, urlTempl string) Changelist {
	return Changelist{
		System:   system,
		SystemID: cl.SystemID,
		Owner:    cl.Owner,
		Status:   cl.Status.String(),
		Subject:  cl.Subject,
		Updated:  cl.Updated,
		URL:      strings.Replace(urlTempl, urlPlaceholder, cl.SystemID, 1),
	}
}

// ChangelistSummary encapsulates how the frontend expects to get a summary of
// the TryJob information we have associated with a given Changelist. These
// TryJobs are those we've noticed that uploaded results to Gold.
type ChangelistSummary struct {
	CL Changelist `json:"cl"`
	// these are only those patchsets with data.
	Patchsets         []Patchset `json:"patch_sets" go2ts:"ignorenil"`
	NumTotalPatchsets int        `json:"num_total_patch_sets"`
}

// Patchset represents the data the frontend needs for Patchsets.
type Patchset struct {
	SystemID string   `json:"id"`
	Order    int      `json:"order"`
	TryJobs  []TryJob `json:"try_jobs" go2ts:"ignorenil"`
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

// TriageRequestData contains the digests in a TriageRequest and their desired labels.
type TriageRequestData map[types.TestName]map[types.Digest]expectations.Label

// TriageRequest is the form of the JSON posted by the frontend when triaging (both single and
// bulk).
type TriageRequest struct {
	// TestDigestStatus maps status to test name and digests. The strings are
	// expectation.Label.String() values
	TestDigestStatus TriageRequestData `json:"testDigestStatus" go2ts:"ignorenil"`

	// ChangelistID is the id of the Changelist for which we want to change the expectations.
	ChangelistID string `json:"changelist_id"`

	// CodeReviewSystem is the id of the crs that the ChangelistID belongs. If ChangelistID is set,
	// CodeReviewSystem should be also.
	CodeReviewSystem string `json:"crs"`

	// ImageMatchingAlgorithm is the name of the non-exact image matching algorithm requesting the
	// triage (see http://go/gold-non-exact-matching). If set, the algorithm name will be used as
	// the author of the triage action.
	//
	// An empty image matching algorithm indicates this is a manual triage operation, in which case
	// the username that initiated the triage operation via Gold's UI will be used as the author of
	// the operation.
	ImageMatchingAlgorithm string `json:"imageMatchingAlgorithm,omitempty"`
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

// TriageLogResponse is the response for /json/v1/triagelog.
type TriageLogResponse struct {
	httputils.ResponsePagination
	Entries []TriageLogEntry `json:"entries"`
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

// IgnoresResponse is the response for /json/v1/ignores.
type IgnoresResponse struct {
	Rules []IgnoreRule `json:"rules"`
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
	CommitTime int64  `json:"commit_time"`
	ID         string `json:"id"`
	// Hash refers to the githash of a commit. It is deprecated, we should refer to commit IDs, not
	// hashes.
	Hash          string `json:"hash"` // For CLs, this is the CL ID.
	Author        string `json:"author"`
	Subject       string `json:"message"`
	ChangelistURL string `json:"cl_url"`
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

// ByBlameResponse is the response for /json/v1/byblame.
type ByBlameResponse struct {
	Data []ByBlameEntry `json:"data"`
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

	validate := validation.Validation{}
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
	TotalDigests     int            `json:"total_digests"`
}

// ListTestsResponse is the response for /json/v1/list.
type ListTestsResponse struct {
	Tests []TestSummary `json:"tests"`
}

// SearchResponse is the structure returned by the Search(...) function of SearchAPI and intended
// to be returned as JSON in an HTTP response.
type SearchResponse struct {
	Results []*SearchResult `json:"digests" go2ts:"ignorenil"`
	// Offset is the offset of the digest into the total list of digests.
	Offset int `json:"offset"`
	// Size is the total number of Digests that match the current query.
	Size    int      `json:"size"`
	Commits []Commit `json:"commits"`
	// BulkTriageData contains *all* digests that match the query as keys. The value for each key is
	// an expectations.Label value giving the label of the closest triaged digest to the key digest
	// or empty string if there is no "closest digest". Note the similarity to the
	// frontend.TriageRequest type.
	BulkTriageData TriageRequestData `json:"bulk_triage_data" go2ts:"ignorenil"`
}

// TriageHistory represents who last triaged a certain digest for a certain test.
type TriageHistory struct {
	User string    `json:"user"`
	TS   time.Time `json:"ts"`
}

// SearchResult is a single digest produced by one or more traces for a given test.
type SearchResult struct {
	// Digest is the primary digest to which the rest of the data in this struct belongs.
	Digest types.Digest `json:"digest"`
	// Test is the name of the test that produced the primary digest. This is needed because
	// we might have a case where, for example, a blank 100x100 image is correct for one test,
	// but not for another test and we need to distinguish between the two cases.
	Test types.TestName `json:"test"`
	// Status is positive, negative, or untriaged. This is also known as the expectation for the
	// primary digest (for Test).
	Status expectations.Label `json:"status"`
	// TriageHistory is a history of all the times the primary digest has been retriaged for the
	// given Test.
	TriageHistory []TriageHistory `json:"triage_history"`
	// ParamSet is all the keys and options of all traces that produce the primary digest and
	// match the given search constraints. It is for frontend UI presentation only; essentially a
	// word cloud of what drew the primary digest.
	ParamSet paramtools.ParamSet `json:"paramset"`
	// TODO(kjlubick) make use of these instead of the combined ParamSet.
	TracesKeys    paramtools.ParamSet `json:"-"`
	TracesOptions paramtools.ParamSet `json:"-"`
	// TraceGroup represents all traces that produced this digest at least once in the sliding window
	// of commits.
	TraceGroup TraceGroup `json:"traces"`
	// RefDiffs are comparisons of the primary digest to other digests in Test. As an example, the
	// closest digest (closest being defined as least different) also triaged positive is usually
	// in here (unless there are no other positive digests).
	// TODO(kjlubick) map is confusing because it's just 2 things. Use struct instead.
	RefDiffs map[RefClosest]*SRDiffDigest `json:"refDiffs"`
	// ClosestRef labels the reference from RefDiffs that is the absolute closest to the primary
	// digest.
	ClosestRef RefClosest `json:"closestRef"` // "pos" or "neg"
}

// SRDiffDigest captures the diff information between a primary digest and the digest given here.
// The primary digest is generally shown on the left in the frontend UI, and the data here
// represents a digest on the right that the primary digest is being compared to.
type SRDiffDigest struct {
	// NumDiffPixels is the absolute number of pixels that are different.
	NumDiffPixels int `json:"numDiffPixels"`

	// CombinedMetric is a value in [0, 10] that represents how large the diff is between two
	// images. It is based off the MaxRGBADiffs and PixelDiffPercent.
	CombinedMetric float32 `json:"combinedMetric"`

	// PixelDiffPercent is the percentage of pixels that are different.
	PixelDiffPercent float32 `json:"pixelDiffPercent"`

	// MaxRGBADiffs contains the maximum difference of each channel.
	MaxRGBADiffs [4]int `json:"maxRGBADiffs"`

	// One of CombinedMetric, PixelDiffPercent, or NumDiffPixels depending on the requested
	// metric name (see query.go). Used internally in search.
	QueryMetric float32 `json:"-"`

	// DimDiffer is true if the dimensions between the two images are different.
	DimDiffer bool `json:"dimDiffer"`

	// Digest identifies which image we are comparing the primary digest to. Put another way, what
	// is the image on the right side of the comparison.
	Digest types.Digest `json:"digest"`
	// Status represents the expectation.Label for this digest.
	Status expectations.Label `json:"status"`
	// ParamSet is all of the params of all traces that produce this digest (the digest on the right).
	// It is for frontend UI presentation only; essentially a word cloud of what drew the primary
	// digest.
	ParamSet paramtools.ParamSet `json:"paramset"`
	// TODO(kjlubick) make use of these instead of the combined ParamSet.
	TracesKeys    paramtools.ParamSet `json:"-"`
	TracesOptions paramtools.ParamSet `json:"-"`
}

// DigestDetails contains details about a digest.
type DigestDetails struct {
	Result  SearchResult `json:"digest"`
	Commits []Commit     `json:"commits"`
}

// Trace describes a single trace, used in TraceGroup.
type Trace struct {
	// The id of the trace. Keep the json as label to be compatible with dots-sk.
	ID tiling.TraceID `json:"label"`
	// RawTrace is meant to be used to hold the raw trace (that is, the tiling.Trace which has not yet
	// been converted for frontend display) until all the raw traces for a given
	// TraceGroup can be converted to the frontend representation. The conversion process needs to be
	// done once all the RawTraces are available so the digest indices can be in agreement for a given
	// TraceGroup. It is not meant to be exposed to the frontend in its raw form.
	RawTrace *tiling.Trace `json:"-"`
	// DigestIndices represents the index of the digest that was part of the trace. -1 means we did
	// not get a digest at this commit. There is one entry per commit. DigestIndices[0] is the oldest
	// commit in the trace, DigestIndices[N-1] is the most recent. The index here matches up with
	// the Digests in the parent TraceGroup.
	DigestIndices []int `json:"data"`
	// Params are the key/value pairs that describe this trace.
	Params map[string]string `json:"params"`
	// TODO(kjlubick) Use these split values instead of the combined one.
	Keys    map[string]string `json:"-"`
	Options map[string]string `json:"-"`
	// CommentIndices are indices into the TraceComments slice on the final result. For example,
	// a 1 means the second TraceComment in the top level TraceComments applies to this trace.
	CommentIndices []int `json:"comment_indices"`
}

// TraceGroup is info about a group of traces. The concrete use of TraceGroup is to represent all
// traces that draw a given digest (known below as the "primary digest") for a given test.
type TraceGroup struct {
	// Traces represents all traces in the TraceGroup. All of these traces have the primary digest.
	Traces []Trace `json:"traces"`
	// Digests represents the triage status of the primary digest and the first N-1 digests that
	// appear in Traces, starting at head on the first trace. N is search.maxDistinctDigestsToPresent.
	Digests []DigestStatus `json:"digests"`
	// TotalDigests is the count of all unique digests in the set of Traces. This number can
	// exceed search.maxDistinctDigestsToPresent.
	TotalDigests int `json:"total_digests"`
}

// DigestStatus is a digest and its status, used in TraceGroup.
type DigestStatus struct {
	Digest types.Digest       `json:"digest"`
	Status expectations.Label `json:"status"`
}

// DigestComparison contains the result of comparing two digests.
type DigestComparison struct {
	Left  SearchResult  `json:"left"`  // The left hand digest and its params.
	Right *SRDiffDigest `json:"right"` // The right hand digest, its params and the diff result.
}

// UntriagedDigestList represents multiple digests that are untriaged for a given query.
type UntriagedDigestList struct {
	Digests []types.Digest `json:"digests"`

	// Corpora is filed with the strings representing a corpus that has one or more Digests belong
	// to it. In other words, it summarizes where the Digests come from.
	Corpora []string `json:"corpora"`

	// TS is the time that this data was created. It might be served from a cache, so this time will
	// not necessarily be "now".
	TS time.Time `json:"ts"`
}

// ChangelistSummaryResponseV1 is a summary of the results associated with a given CL. It focuses on
// the untriaged and new images produced.
type ChangelistSummaryResponseV1 struct {
	// ChangelistID is the nonqualified id of the CL.
	ChangelistID string `json:"changelist_id"`
	// PatchsetSummaries is a summary for all Patchsets for which we have data.
	PatchsetSummaries []PatchsetNewAndUntriagedSummaryV1 `json:"patchsets"`
	// Outdated will be true if this is a stale cached entry. Clients are free to try again later
	// for the latest results.
	Outdated bool `json:"outdated"`
}

// PatchsetNewAndUntriagedSummaryV1 is the summary for a specific PS. It focuses on the untriaged
// and new images produced.
type PatchsetNewAndUntriagedSummaryV1 struct {
	// NewImages is the number of new images (digests) that were produced by this patchset by
	// non-ignored traces and not seen on the primary branch.
	NewImages int `json:"new_images"`
	// NewUntriagedImages is the number of NewImages which are still untriaged. It is less than or
	// equal to NewImages.
	NewUntriagedImages int `json:"new_untriaged_images"`
	// TotalUntriagedImages is the number of images produced by this patchset by non-ignored traces
	// that are untriaged. This includes images that are untriaged and observed on the primary
	// branch (i.e. might not be the fault of this CL/PS). It is greater than or equal to
	// NewUntriagedImages.
	TotalUntriagedImages int `json:"total_untriaged_images"`
	// PatchsetID is the nonqualified id of the patchset. This is usually a git hash.
	PatchsetID string `json:"patchset_id"`
	// PatchsetOrder is represents the chronological order the patchsets are in. It starts at 1.
	PatchsetOrder int `json:"patchset_order"`
}

// ClusterDiffResult contains the result of comparing all digests within a test.
// It is structured to be easy to render by the D3.js.
type ClusterDiffResult struct {
	// Nodes represents each digest that matched the cluster criteria.
	Nodes []Node `json:"nodes"`
	// Links represents all possible comparisons between the nodes. There should be
	// (n-1) + (n-2) ... + 2 + 1 links for n nodes, or n(n-1)/2
	Links []Link `json:"links"`

	Test types.TestName `json:"test"`
	// ParamsetByDigest is a mapping of digest to the ParamSet created by combining all Params
	// for all traces that produce this digest.
	ParamsetByDigest map[types.Digest]paramtools.ParamSet `json:"paramsetByDigest" go2ts:"ignorenil"`
	// ParamsetsUnion is the union of all Params from all Traces that matched the cluster criteria.
	// It is also a union of all ParamSet in ParamsetByDigest.
	ParamsetsUnion paramtools.ParamSet `json:"paramsetsUnion"`
}

// Node represents a single node in a d3 diagram. Used in ClusterDiffResult.
type Node struct {
	Digest types.Digest       `json:"name"`
	Status expectations.Label `json:"status"`
}

// Link represents a link between d3 nodes, used in ClusterDiffResult.
type Link struct {
	// LeftIndex is the index in the sibling Nodes slice corresponding to the "left" digest.
	LeftIndex int `json:"source"`
	// RightIndex is the index in the sibling Nodes slice corresponding to the "right" digest.
	RightIndex int `json:"target"`
	// Distance is how far apart the two digests are. This distance is currently the percentage
	// of pixels different between the two images.
	Distance float32 `json:"value"`
}

// BaselineV2Response captures the data necessary to verify test results on the
// commit queue. A baseline is essentially just the positive and negative expectations
// for a branch.
type BaselineV2Response struct {
	// Expectations captures the "baseline expectations", that is, the expectations with only the
	// positive and negative digests (i.e. no untriaged digest) of the current commit.
	Expectations expectations.Baseline `json:"primary,omitempty"`

	// ChangelistID indicates the Gerrit or GitHub issue id of this baseline.
	// "" indicates the master branch.
	ChangelistID string `json:"cl_id,omitempty"`

	// CodeReviewSystem indicates which CRS system (if any) this baseline is tied to.
	// (e.g. "gerrit", "github") "" indicates the master branch.
	CodeReviewSystem string `json:"crs,omitempty"`
}
