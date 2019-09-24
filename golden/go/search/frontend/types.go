// Package frontend contains structs that represent how the
// frontend expects output from the search package.
package frontend

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/types"
)

// SearchResponse is the structure returned by the
// Search(...) function of SearchAPI and intended to be
// returned as JSON in an HTTP response.
type SearchResponse struct {
	Digests []*SRDigest      `json:"digests"`
	Offset  int              `json:"offset"`
	Size    int              `json:"size"`
	Commits []*tiling.Commit `json:"commits"`
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
	Digest  *SRDigest        `json:"digest"`
	Commits []*tiling.Commit `json:"commits"`
}

// Point is a single point. Used to draw the trace diagrams on the frontend.
type Point struct {
	X int `json:"x"` // The commit index [0-49].
	Y int `json:"y"`
	S int `json:"s"` // Status of the digest: 0 if the digest matches our search, 1-8 otherwise.
}

// Trace describes a single trace, used in TraceGroup.
type Trace struct {
	Data   []Point           `json:"data"`  // One Point for each test result.
	ID     tiling.TraceId    `json:"label"` // The id of the trace. Keep the json as label to be compatible with dots-sk.
	Params map[string]string `json:"params"`
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
}

// DigestComparison contains the result of comparing two digests.
type DigestComparison struct {
	Left  *SRDigest     `json:"left"`  // The left hand digest and its params.
	Right *SRDiffDigest `json:"right"` // The right hand digest, its params and the diff result.
}

// DigestTable is the structure returned by GetDigestTable.
type DigestTable struct {
	Grid      *DTGrid                       `json:"grid"`
	Name      string                        `json:"name"`
	Corpus    string                        `json:"source_type"`
	Summaries map[types.TestName]*DTSummary `json:"summaries"`
	Positive  int                           `json:"pos"`
	Negative  int                           `json:"neg"`
	Untriaged int                           `json:"unt"`
}

// DTGrid contains the grid of diff values returned in DigestTable.
type DTGrid struct {
	// Rows contains the row digest and the number of times it occurs.
	Rows []*DTRow `json:"rows"`

	// RowsTotal contains the total number of rows for the given query.
	RowsTotal int `json:"rowTotal"`

	// Columns contains the reference points calculated for each row digests.
	Columns []string `json:"columns"` // Contains the column types.

	// ColumnsTotal contains the total number of column digests.
	ColumnsTotal int `json:"columnsTotal"`
}

// DTDigestCount captures the digest and how often it appears in the tile.
type DTDigestCount struct {
	Digest types.Digest `json:"digest"`
	N      int          `json:"n"`
}

// DTRow is used by DTGrid to encode row digest information.
type DTRow struct {
	DTDigestCount
	TestName types.TestName   `json:"test"`
	Values   []*DTDiffMetrics `json:"values"`
}

// DTDiffMetrics contains diff metric between the contain digest and the
// corresponding row digest.
type DTDiffMetrics struct {
	*diff.DiffMetrics
	DTDigestCount
}

// DTSummary summarizes the counts for a given test.
type DTSummary struct {
	Pos       int `json:"pos"`
	Neg       int `json:"neg"`
	Untriaged int `json:"untriaged"`
}
