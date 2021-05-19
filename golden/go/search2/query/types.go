package query

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
)

// Search represents the params to the Search function.
type Search struct {
	// Diff metric to use.
	Metric string
	Sort   string
	Match  []string

	// Blaming
	BlameGroupID string

	// Image classification
	IncludePositiveDigests           bool
	IncludeNegativeDigests           bool
	IncludeUntriagedDigests          bool
	OnlyIncludeDigestsProducedAtHead bool
	IncludeIgnoredTraces             bool

	// URL encoded query string
	QueryStr    string
	TraceValues paramtools.ParamSet
	// Not given to us by the frontend yet.
	OptionsValues paramtools.ParamSet

	// URL encoded query string to select the right hand side of comparisons.
	RightQueryStr    string
	RightTraceValues paramtools.ParamSet

	// TryJob support.
	ChangelistID       string
	CodeReviewSystemID string
	// TODO(kjlubick) Change this so only one patchset is allowed. It will simplify the backend code.
	PatchsetsStr string // Comma-separated list of patchsets.
	Patchsets    []int64
	// By default, we typically only want to see digests that were created exclusively on this CL,
	// but sometimes the user wants to also see digests that are the same as on master, so this option
	// allows for that.
	IncludeDigestsProducedOnMaster bool

	// Filtering.
	RGBAMinFilter              int  // Min RGBA delta
	RGBAMaxFilter              int  // Max RGBA delta
	MustIncludeReferenceFilter bool // Only digests with reference.

	// Pagination.
	Offset int
	Limit  int
}

// IgnoreState returns the types.IgnoreState that this
// Search query is configured for.
func (q *Search) IgnoreState() types.IgnoreState {
	is := types.ExcludeIgnoredTraces
	if q.IncludeIgnoredTraces {
		is = types.IncludeIgnoredTraces
	}
	return is
}

// ExcludesClassification returns true if the given label/status for a digest
// should be excluded based on the values in the query.
func (q *Search) ExcludesClassification(cl expectations.Label) bool {
	return ((cl == expectations.Negative) && !q.IncludeNegativeDigests) ||
		((cl == expectations.Positive) && !q.IncludePositiveDigests) ||
		((cl == expectations.Untriaged) && !q.IncludeUntriagedDigests)
}
