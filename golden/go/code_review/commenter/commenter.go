// Package commenter contains an implementation of the code_review.ChangeListCommenter interface.
// It should be CRS-agnostic.
package commenter

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

const (
	numOpenCLsMetric = "gold_num_open_cls"
)

type Impl struct {
	crs   code_review.Client
	store clstore.Store
}

func New(c code_review.Client, s clstore.Store) *Impl {
	return &Impl{
		crs:   c,
		store: s,
	}
}

// CommentOnChangeListsWithUntriagedDigests implements the code_review.ChangeListCommenter
// interface.
func (i *Impl) CommentOnChangeListsWithUntriagedDigests(ctx context.Context) error {
	total := 0
	// This pageSize was picked arbitrarily, could be larger, but hopefully we don't have to
	// deal with that many CLs at once.
	const pageSize = 10000
	// Due to the fact that cl.Updated gets set in ingestion when new data is seen, we only need
	// to look at CLs that were Updated "recently". We make the range of time that we search
	// much wider than we need to account for either glitches in ingestion or outages of the CRS.
	recent := time.Now().Add(-48 * time.Hour)
	xcl, _, err := i.store.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx:    0,
		Limit:       pageSize,
		OpenCLsOnly: true,
		After:       recent,
	})
	if err != nil {
		return skerr.Wrapf(err, "searching for open CLs")
	}

	// stillOpen maps id to ChangeList to avoid duplication
	// (this could happen due to paging trickiness)
	stillOpen := map[string]code_review.ChangeList{}
	var openMutex sync.Mutex
	// Number of shards was picked sort of arbitrarily. Updating CLs requires multiple network
	// requests, so we can run them in parallel basically for free.
	const shards = 4
	for len(xcl) > 0 {
		err := util.ChunkIterParallel(ctx, len(xcl), len(xcl)/shards, func(ctx context.Context, startIdx int, endIdx int) error {
			for _, cl := range xcl[startIdx:endIdx] {
				open, err := i.updateCLInStoreIfNotOpen(ctx, cl)
				if err != nil {
					return skerr.Wrap(err)
				}
				if open {
					openMutex.Lock()
					stillOpen[cl.SystemID] = cl
					openMutex.Unlock()
				}
			}
			return nil
		})
		if err != nil {
			return skerr.Wrap(err)
		}

		total += len(xcl)
		// Page to the next ones using len(stillOpen) because the next iteration of this query
		// won't count the ones we just marked as Closed/Abandoned when computing the offset.
		xcl, _, err = i.store.GetChangeLists(ctx, clstore.SearchOptions{
			StartIdx:    len(stillOpen),
			Limit:       pageSize,
			OpenCLsOnly: true,
			After:       recent,
		})
		if err != nil {
			return skerr.Wrapf(err, "searching for open CLs total %d", total)
		}
	}
	metrics2.GetInt64Metric(numOpenCLsMetric, nil).Update(int64(len(stillOpen)))
	sklog.Infof("There were originally %d open CLs; after checking with CRS there are %d open CLs", total, len(stillOpen))

	// TODO(kjlubick): iterate through the open changelists and determine which, if any, should
	//   be commented on.

	return nil
}

// updateCLInStoreIfNotOpen checks with the CRS to see if the cl is still open. If it is, it returns
// true, otherwise it stores the updated CL in the store and returns false.
func (i *Impl) updateCLInStoreIfNotOpen(ctx context.Context, cl code_review.ChangeList) (bool, error) {
	up, err := i.crs.GetChangeList(ctx, cl.SystemID)
	if err == code_review.ErrNotFound {
		sklog.Debugf("CL %s might have been deleted", cl.SystemID)
		return false, nil
	}
	if err != nil {
		return false, skerr.Wrapf(err, "querying crs %s for updated CL %s", i.crs.System(), cl.SystemID)
	}
	if up.Status == code_review.Open {
		return true, nil
	}
	// Store the latest one from the CRS (with new timestamp) to the clstore so we
	// remember it is closed/abandoned in the future. This also catches things like the cl Subject
	// changing since it was opened.
	up.Updated = time.Now()
	if err := i.store.PutChangeList(ctx, up); err != nil {
		return false, skerr.Wrapf(err, "storing CL %s", up.SystemID)
	}
	return false, nil
}

// Make sure Impl fulfills the code_review.ChangeListCommenter interface.
var _ code_review.ChangeListCommenter = (*Impl)(nil)
