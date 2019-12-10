// Package commenter contains an implementation of the code_review.ChangeListCommenter interface.
// It should be CRS-agnostic.
package commenter

import (
	"context"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

const (
	numOpenCLsMetric = "gold_num_open_cls"
)

type Impl struct {
	client code_review.Client
	store  clstore.Store
}

func New(c code_review.Client, s clstore.Store) *Impl {
	return &Impl{
		client: c,
		store:  s,
	}
}

// CommentOnChangeListsWithUntriagedDigests implements the code_review.ChangeListCommenter
// interface.
func (i *Impl) CommentOnChangeListsWithUntriagedDigests(ctx context.Context) error {
	offset := 0
	const pageSize = 1000
	xcl, _, err := i.store.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx:    offset,
		Limit:       pageSize,
		OpenCLsOnly: true,
	})
	if err != nil {
		return skerr.Wrapf(err, "searching for open CLs")
	}
	for len(xcl) > 0 {
		// TODO(kjlubick): Check the crs to see if these CLs are still open. Put still open ones
		//   in another slice and then check to see if those need a comment.

		// Page to the next ones
		offset += len(xcl)
		xcl, _, err = i.store.GetChangeLists(ctx, clstore.SearchOptions{
			StartIdx:    offset,
			Limit:       pageSize,
			OpenCLsOnly: true,
		})
		if err != nil {
			return skerr.Wrapf(err, "searching for open CLs offset %d", offset)
		}
	}
	metrics2.GetInt64Metric(numOpenCLsMetric, nil).Update(int64(offset))
	sklog.Infof("There were %d open CLs", offset)
	return nil
}

// Make sure Impl fulfills the code_review.ChangeListCommenter interface.
var _ code_review.ChangeListCommenter = (*Impl)(nil)
