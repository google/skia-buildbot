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
	IncludePositiveDigests           bool `json:"pos"`
	IncludeNegativeDigests           bool `json:"neg"`
	IncludeUntriagedDigests          bool `json:"unt"`
	OnlyIncludeDigestsProducedAtHead bool `json:"head"`
	IncludeIgnoredTraces             bool `json:"include"`

	// URL encoded query string
	QueryStr    string              `json:"query"`
	TraceValues paramtools.ParamSet `json:"-"`
	// Not given to us by the frontend yet.
	OptionsValues paramtools.ParamSet `json:"-"`

	// URL encoded query string to select the right hand side of comparisons.
	RightQueryStr    string              `json:"rquery"`
	RightTraceValues paramtools.ParamSet `json:"-"`

	// TryJob support.
	ChangelistID       string `json:"issue"`
	CodeReviewSystemID string `json:"crs_id"`
	// TODO(kjlubick) Change this so only one patchset is allowed. It will simplify the backend code.
	PatchsetsStr string  `json:"patchsets"` // Comma-separated list of patchsets.
	Patchsets    []int64 `json:"-"`
	// By default, we typically only want to see digests that were created exclusively on this CL,
	// but sometimes the user wants to also see digests that are the same as on master, so this option
	// allows for that.
	IncludeDigestsProducedOnMaster bool `json:"master"`

	// Filtering.
	RGBAMinFilter              int  `json:"frgbamin"` // Min RGBA delta
	RGBAMaxFilter              int  `json:"frgbamax"` // Max RGBA delta
	MustIncludeReferenceFilter bool `json:"fref"`     // Only digests with reference.

	// Pagination.
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
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
