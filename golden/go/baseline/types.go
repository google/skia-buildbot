package baseline

import (
	"context"

	"go.skia.org/infra/golden/go/expectations"
)

// Baseline captures the data necessary to verify test results on the
// commit queue. A baseline is essentially just the positive expectations
// for a branch.
type Baseline struct {
	// MD5 is the hash of the Expectations field. Can be used to quickly test equality.
	MD5 string `json:"md5"`

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

type BaselineFetcher interface {
	// FetchBaseline fetches a Baseline. If clID and crs are non-empty, the given Changelist will be
	// created by loading the master baseline and the CL baseline and combining them.
	FetchBaseline(ctx context.Context, clID, crs string) (*Baseline, error)
}
