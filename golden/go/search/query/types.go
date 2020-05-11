package query

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
)

// Search represents the params to the Search function.
type Search struct {
	// Diff metric to use.
	Metric string   `json:"metric"`
	Sort   string   `json:"sort"`
	Match  []string `json:"match"`

	// Blaming
	BlameGroupID string `json:"blame"`

	// Image classification
	Pos            bool `json:"pos"`
	Neg            bool `json:"neg"`
	Head           bool `json:"head"`
	Unt            bool `json:"unt"`
	IncludeIgnores bool `json:"include"`

	// URL encoded query string
	QueryStr    string              `json:"query"`
	TraceValues paramtools.ParamSet `json:"-"`

	// URL encoded query string to select the right hand side of comparisons.
	RQueryStr    string              `json:"rquery"`
	RTraceValues paramtools.ParamSet `json:"-"`

	// Trybot support.
	ChangeListID  string  `json:"issue"`
	PatchSetsStr  string  `json:"patchsets"` // Comma-separated list of patchsets.
	PatchSets     []int64 `json:"-"`
	IncludeMaster bool    `json:"master"` // Include digests also contained in master when searching code review issues.

	// Filtering.
	FCommitBegin string  `json:"fbegin"`     // Start commit
	FCommitEnd   string  `json:"fend"`       // End commit
	FRGBAMin     int32   `json:"frgbamin"`   // Min RGBA delta
	FRGBAMax     int32   `json:"frgbamax"`   // Max RGBA delta
	FDiffMax     float32 `json:"fdiffmax"`   // Max diff according to metric
	FGroupTest   string  `json:"fgrouptest"` // Op within grouped by test.
	FRef         bool    `json:"fref"`       // Only digests with reference.

	// Pagination.
	Offset int32 `json:"offset"`
	Limit  int32 `json:"limit"`

	// Do not include diffs in search.
	NoDiff bool `json:"nodiff"`
}

// IgnoreState returns the types.IgnoreState that this
// Search query is configured for.
func (q *Search) IgnoreState() types.IgnoreState {
	is := types.ExcludeIgnoredTraces
	if q.IncludeIgnores {
		is = types.IncludeIgnoredTraces
	}
	return is
}

// ExcludesClassification returns true if the given label/status for a digest
// should be excluded based on the values in the query.
func (q *Search) ExcludesClassification(cl expectations.Label) bool {
	return ((cl == expectations.Negative) && !q.Neg) ||
		((cl == expectations.Positive) && !q.Pos) ||
		((cl == expectations.Untriaged) && !q.Unt)
}
