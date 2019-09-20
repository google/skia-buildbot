// package baseline contains functions to gather the current baseline and return them.
package baseline

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

// GetBaselineForCL returns the baseline for the given issue. This baseline
// contains all triaged digests that are not in the master tile.
func GetBaselineForCL(id, crs string, tryjobs []*tryjobstore.Tryjob, tryjobResults [][]*tryjobstore.TryjobResult, exp types.Expectations) (*Baseline, error) {
	b := types.Expectations{}
	for idx := range tryjobs {
		for _, result := range tryjobResults[idx] {
			if result.Digest != types.MISSING_DIGEST && exp.Classification(result.TestName, result.Digest) == types.POSITIVE {
				b.AddDigest(result.TestName, result.Digest, types.POSITIVE)
			}
		}
	}

	md5Sum, err := util.MD5Sum(b)
	if err != nil {
		return nil, skerr.Wrapf(err, "calculating MD5 sum of %v", b)
	}

	ret := &Baseline{
		Expectations:     b,
		ChangeListID:     id,
		CodeReviewSystem: crs,
		MD5:              md5Sum,
	}
	return ret, nil
}
