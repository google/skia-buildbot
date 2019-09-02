// Package frontend contains structs that represent how the
// frontend expects output from the search package.
package frontend

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/types"
)

// SRDigest is a single search result digest returned
// by the Search function below.
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

// SRDigestDetails contains details about a digest.
type SRDigestDetails struct {
	Digest  *SRDigest        `json:"digest"`
	Commits []*tiling.Commit `json:"commits"`
}

// Point is a single point. Used to draw the trace diagrams on the frontend.
type Point struct {
	X int `json:"x"` // The commit index [0-49].
	Y int `json:"y"`
	S int `json:"s"` // Status of the digest: 0 if the digest matches our search, 1-8 otherwise.
}

// Trace describes a single trace, used in Traces.
type Trace struct {
	Data   []Point           `json:"data"`  // One Point for each test result.
	ID     tiling.TraceId    `json:"label"` // The id of the trace. Keep the json as label to be compatible with dots-sk.
	Params map[string]string `json:"params"`
}

// DigestStatus is a digest and its status, used in Traces.
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

// DiffDigest is information about a digest different from the one in Digest.
type DiffDigest struct {
	// TODO(stephana): Replace digesttools.Closest here and above with an overall
	// consolidated structure to measure the distance between two digests.
	Closest  *digesttools.Closest `json:"closest"`
	ParamSet map[string][]string  `json:"paramset"`
}

// // Diff is only populated for digests that are untriaged?
// // Might still be useful to find diffs to closest pos for a neg, and vice-versa.
// // Will also be useful if we ever get a canonical trace or centroid.
// type Diff struct {
// 	Diff float32 `json:"diff"` // The smaller of the Pos and Neg diff.

// 	// Either may be nil if there's no positive or negative to compare against.
// 	Pos *DiffDigest `json:"pos"`
// 	Neg *DiffDigest `json:"neg"`
// 	//Centroid *DiffDigest

// 	Blame *blame.BlameDistribution `json:"blame"`
// }

// // Digests are returned from Search, one for each match to Query.
// type Digest struct {
// 	Test     string              `json:"test"`
// 	Digest   string              `json:"digest"`
// 	Status   string              `json:"status"`
// 	ParamSet map[string][]string `json:"paramset"`
// 	Traces   *TraceGroup         `json:"traces"`
// 	Diff     *Diff               `json:"diff"`
// }

// TODO(kjlubick): rename to CompareDigestsResult
// SRDigestDiff contains the result of comparing two digests.
type SRDigestDiff struct {
	Left  *SRDigest     `json:"left"`  // The left hand digest and its params.
	Right *SRDiffDigest `json:"right"` // The right hand digest, its params and the diff result.
}
