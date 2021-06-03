// Package simple_baseliner houses an implementation of BaselineFetcher that directly
// interfaces with a ExpectationsStore.
package simple_baseliner

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expectations"
)

// The SimpleBaselineFetcher is an implementation of BaselineFetcher that directly
// interfaces with the ExpectationsStore to retrieve the baselines.
// Reminder that baselines are the set of current expectations, but only
// the positive and negative images (i.e. no untriaged images).
type SimpleBaselineFetcher struct {
	exp expectations.Store
}

// New returns an instance of SimpleBaselineFetcher. The passed in ExpectationsStore
// can/should be read-only.
func New(e expectations.Store) *SimpleBaselineFetcher {
	return &SimpleBaselineFetcher{
		exp: e,
	}
}

// FetchBaseline implements the BaselineFetcher interface.
func (f *SimpleBaselineFetcher) FetchBaseline(ctx context.Context, clID, crs string) (*baseline.Baseline, error) {
	if clID == "" {
		exp, err := f.exp.GetCopy(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting master branch expectations")
		}
		b := baseline.Baseline{
			ChangelistID:     "",
			CodeReviewSystem: "",
			Expectations:     exp.AsBaseline(),
		}
		md5Sum, err := util.MD5Sum(b.Expectations)
		if err != nil {
			return nil, skerr.Wrapf(err, "calculating md5 hash of expectations")
		}
		b.MD5 = md5Sum
		return &b, nil
	}

	issueStore := f.exp.ForChangelist(clID, crs)

	iexp, err := issueStore.GetCopy(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting expectations for %s (%s)", clID, crs)
	}

	exp, err := f.exp.GetCopy(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting master branch expectations")
	}

	exp.MergeExpectations(iexp)

	b := baseline.Baseline{
		ChangelistID:     clID,
		CodeReviewSystem: crs,
		Expectations:     exp.AsBaseline(),
	}
	md5Sum, err := util.MD5Sum(b.Expectations)
	if err != nil {
		return nil, skerr.Wrapf(err, "calculating md5 hash of expectations")
	}
	b.MD5 = md5Sum
	return &b, nil
}

// Make sure SimpleBaselineFetcher fulfills the BaselineFetcher interface
var _ baseline.BaselineFetcher = (*SimpleBaselineFetcher)(nil)
