// Package frontend contains structs that represent how the frontend expects output from the search
// package.
package frontend

import (
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/comment/trace"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

// SearchResponse is the structure returned by the Search(...) function of SearchAPI and intended
// to be returned as JSON in an HTTP response.
type SearchResponse struct {
	Results []*SearchResult `json:"digests"`
	// Offset is the offset of the digest into the total list of digests.
	Offset int `json:"offset"`
	// Size is the total number of Digests that match the current query.
	Size          int               `json:"size"`
	Commits       []frontend.Commit `json:"commits"`
	TraceComments []TraceComment    `json:"trace_comments"`
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
	Status string `json:"status"`
	// TriageHistory is a history of all the times the primary digest has been retriaged for the
	// given Test.
	TriageHistory []TriageHistory `json:"triage_history"`
	// ParamSet is all the params of all traces that produce the primary digest. It is for frontend UI
	// presentation only; essentially a word cloud of what drew the primary digest.
	ParamSet paramtools.ParamSet `json:"paramset"`
	// TraceGroup represents all traces that produced this digest at least once in the sliding window
	// of commits.
	TraceGroup TraceGroup `json:"traces"`
	// RefDiffs are comparisons of the primary digest to other digests in Test. As an example, the
	// closest digest (closest being defined as least different) also triaged positive is usually
	// in here (unless there are no other positive digests).
	// TODO(kjlubick) map is confusing because it's just 2 things. Use struct instead.
	RefDiffs map[common.RefClosest]*SRDiffDigest `json:"refDiffs"`
	// ClosestRef labels the reference from RefDiffs that is the absolute closest to the primary
	// digest.
	ClosestRef common.RefClosest `json:"closestRef"` // "pos" or "neg"
}

// SRDiffDigest captures the diff information between a primary digest and the digest given here.
// The primary digest is generally shown on the left in the frontend UI, and the data here
// represents a digest on the right that the primary digest is being compared to.
type SRDiffDigest struct {
	*diff.DiffMetrics
	// Digest identifies which image we are comparing the primary digest to. Put another way, what
	// is the image on the right side of the comparison.
	Digest types.Digest `json:"digest"`
	// Status represents the expectation.Label for this digest.
	Status string `json:"status"`
	// ParamSet is all of the params of all traces that produce this digest (the digest on the right).
	// It is for frontend UI presentation only; essentially a word cloud of what drew the primary
	// digest.
	ParamSet paramtools.ParamSet `json:"paramset"`
	// TODO(kjlubick) What is is this? Is it displayed? Can it go away?
	OccurrencesInTile int `json:"n"`
}

// DigestDetails contains details about a digest.
type DigestDetails struct {
	Result        SearchResult      `json:"digest"`
	Commits       []frontend.Commit `json:"commits"`
	TraceComments []TraceComment    `json:"trace_comments"`
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
	// CommentIndices are indices into the TraceComments slice on the final result. For example,
	// a 1 means the second TraceComment in the top level TraceComments applies to this trace.
	CommentIndices []int `json:"comment_indices"`
}

// TraceComment is the frontend representation of a trace.Comment
type TraceComment struct {
	ID trace.ID `json:"id"`
	// CreatedBy is the email address of the user who created this trace comment.
	CreatedBy string `json:"created_by"`
	// UpdatedBy is the email address of the user who most recently updated this trace comment.
	UpdatedBy string `json:"updated_by"`
	// CreatedTS is when the comment was created.
	CreatedTS time.Time `json:"created_ts"`
	// UpdatedTS is when the comment was updated.
	UpdatedTS time.Time `json:"updated_ts"`
	// Text is an arbitrary string. There can be special rules that only the frontend cares about
	// (e.g. some markdown or coordinates).
	Text string `json:"text"`
	// QueryToMatch represents which traces this trace comment should apply to.
	QueryToMatch paramtools.ParamSet `json:"query"`
}

// ToTraceComment converts a trace.Comment into a TraceComment
func ToTraceComment(c trace.Comment) TraceComment {
	return TraceComment{
		ID:           c.ID,
		CreatedBy:    c.CreatedBy,
		UpdatedBy:    c.UpdatedBy,
		CreatedTS:    c.CreatedTS,
		UpdatedTS:    c.UpdatedTS,
		Text:         c.Comment,
		QueryToMatch: c.QueryToMatch,
	}
}

// TraceGroup is info about a group of traces. The concrete use of TraceGroup is to represent all
// traces that draw a given digest (known below as the "primary digest") for a given test.
type TraceGroup struct {
	// TileSize is how many digests appear in Traces.
	// TODO(kjlubick) this is no longer needed, now that Traces are dense and not skipping commits.
	TileSize int `json:"tileSize"`
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
	Digest types.Digest `json:"digest"`
	Status string       `json:"status"`
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
