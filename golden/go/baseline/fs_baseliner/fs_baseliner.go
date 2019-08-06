// Package fs_baseliner houses an implementation of BaselineFetcher that directly
// interfaces with the expectations stored on FireStore (FS).
package fs_baseliner

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// The FirestoreBaseliner is an implementation of BaselineFetcher that directly
// interfaces with the expectations stored on FireStore
type FirestoreBaseliner struct {
	exp expstorage.ExpectationsStore
}

// New returns an instance of FirestoreBaseliner
func New(e expstorage.ExpectationsStore) *FirestoreBaseliner {
	return &FirestoreBaseliner{
		exp: e,
	}
}

// FetchBaseline implements the BaselineFetcher interface.
func (f *FirestoreBaseliner) FetchBaseline(_ string, issueID int64, issueOnly bool) (*baseline.Baseline, error) {
	if types.IsMasterBranch(issueID) {
		exp, err := f.exp.Get()
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get master branchexpectations")
		}
		b := baseline.Baseline{
			Issue:        types.MasterBranch,
			Expectations: exp.AsBaseline(),
			MD5:          "",
		}
		md5Sum, err := util.MD5Sum(b.Expectations)
		if err != nil {
			return nil, skerr.Wrapf(err, "calculating md5 hash of expectations")
		}
		b.MD5 = md5Sum
		return &b, nil
	}

	issueStore := f.exp.ForIssue(issueID)

	iexp, err := issueStore.Get()
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get expectations for %d", issueID)
	}
	if issueOnly {
		return &baseline.Baseline{
			Issue:        issueID,
			Expectations: iexp,
		}, nil
	}

	exp, err := f.exp.Get()
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get master branch expectations")
	}

	exp.MergeExpectations(iexp)

	b := baseline.Baseline{
		Issue:        issueID,
		Expectations: exp.AsBaseline(),
		MD5:          "",
	}
	md5Sum, err := util.MD5Sum(b.Expectations)
	if err != nil {
		return nil, skerr.Wrapf(err, "calculating md5 hash of expectations")
	}
	b.MD5 = md5Sum
	return &b, nil
}

// Make sure FirestoreBaseliner fulfills the BaselineFetcher interface
var _ baseline.BaselineFetcher = (*FirestoreBaseliner)(nil)
