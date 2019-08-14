package baseline

import (
	"go.skia.org/infra/golden/go/types"
)

// Baseline captures the data necessary to verify test results on the
// commit queue. A baseline is essentially just the positive expectations
// for a branch.
type Baseline struct {
	// MD5 is the hash of the Expectations field. Can be used to quickly test equality.
	MD5 string `json:"md5"`

	// Expectations captures the "baseline expectations", that is, the Expectations
	// with only the positive digests of the current commit.
	Expectations types.Expectations `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. -1 indicates the master branch.
	Issue int64
}

// Copy returns a deep copy of the given instance of Baseline.
// Note: It assumes all members except for Baseline to be immutable, thus only
// Baseline is "deep" copied.
func (c *Baseline) Copy() *Baseline {
	ret := &Baseline{}
	*ret = *c
	ret.Expectations = c.Expectations.DeepCopy()
	return ret
}

// EmptyBaseline returns an instance of Baseline with the provided commits and nil
// values in all other fields. The Baseline field contains an empty instance of types.Expectations.
func EmptyBaseline() *Baseline {
	return &Baseline{
		Expectations: types.Expectations{},
		MD5:          md5SumEmptyExp,
	}
}

type BaselineFetcher interface {
	// FetchBaseline fetches the complete baseline for the given Gerrit issue by
	// loading the master baseline and the issue baseline from GCS and combining
	// them. If either of them doesn't exist an empty baseline is assumed.
	// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
	// returned (omitting the baselines of the master branch).
	// issueOnly is primarily used for debugging.
	// TODO(kjlubick): remove commitHash as it has no meaning anymore, now that per-commit
	// baselines have been removed.
	FetchBaseline(commitHash string, issueID int64, issueOnly bool) (*Baseline, error)
}
