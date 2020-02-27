// Package frontend contains structs that represent how the
// frontend expects output from the search package.
package frontend

import (
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/comment/trace"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/types"
)

// SearchResponse is the structure returned by the
// Search(...) function of SearchAPI and intended to be
// returned as JSON in an HTTP response.
type SearchResponse struct {
	Digests       []*SRDigest      `json:"digests"`
	Offset        int              `json:"offset"`
	Size          int              `json:"size"`
	Commits       []*tiling.Commit `json:"commits"`
	TraceComments []TraceComment   `json:"trace_comments"`
}

// SRDigest is a single search result digest returned
// by SearchAPI.Search.
type SRDigest struct {
	Test       types.TestName                      `json:"test"`
	Digest     types.Digest                        `json:"digest"`
	Status     string                              `json:"status"`
	ParamSet   paramtools.ParamSet                 `json:"paramset"`
	Traces     *TraceGroup                         `json:"traces"`
	ClosestRef common.RefClosest                   `json:"closestRef"` // "pos" or "neg"
	RefDiffs   map[common.RefClosest]*SRDiffDigest `json:"refDiffs"`
}

// SRDiffDigest captures the diff information between
// a primary digest and the digest given here. The primary
// digest is given by the context where this is used.
type SRDiffDigest struct {
	*diff.DiffMetrics
	Digest            types.Digest        `json:"digest"`
	Status            string              `json:"status"`
	ParamSet          paramtools.ParamSet `json:"paramset"`
	OccurrencesInTile int                 `json:"n"`
}

// DigestDetails contains details about a digest.
type DigestDetails struct {
	Digest        *SRDigest        `json:"digest"`
	Commits       []*tiling.Commit `json:"commits"`
	TraceComments []TraceComment   `json:"trace_comments"`
}

// Trace describes a single trace, used in TraceGroup.
type Trace struct {
	// Data represents the index of the digest that was part of the trace. -1 means we did not get
	// a digest at this commit. There is one entry per commit. Index 0 is the oldest commit in the
	// trace, index N-1 is the most recent.
	Data []int `json:"data"`
	// The id of the trace. Keep the json as label to be compatible with dots-sk.
	ID     tiling.TraceID    `json:"label"`
	Params map[string]string `json:"params"`
	// CommentIndices are indices into the TraceComments slice on the final result. For example,
	// a 1 means the second TraceComment in the top level TraceComments applies to this trace.
	CommentIndices []int `json:"comment_indicies"`
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

// DigestStatus is a digest and its status, used in TraceGroup.
type DigestStatus struct {
	Digest types.Digest `json:"digest"`
	Status string       `json:"status"`
}

// TraceGroup is info about a group of traces.
type TraceGroup struct {
	TileSize int            `json:"tileSize"`
	Traces   []Trace        `json:"traces"`  // The traces where this digest appears.
	Digests  []DigestStatus `json:"digests"` // The other digests that appear in Traces.
	// TODO(skbug.com/4310) Add in a count for total Digests.
}

// DigestComparison contains the result of comparing two digests.
type DigestComparison struct {
	Left  *SRDigest     `json:"left"`  // The left hand digest and its params.
	Right *SRDiffDigest `json:"right"` // The right hand digest, its params and the diff result.
}

// DigestList represents multiple returned digests.
type DigestList struct {
	Digests []types.Digest `json:"digests"`
}
