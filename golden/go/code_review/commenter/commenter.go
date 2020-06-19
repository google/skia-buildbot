// Package commenter contains an implementation of the code_review.ChangeListCommenter interface.
// It should be CRS-agnostic.
package commenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/tjstore"
)

const (
	numRecentOpenCLsMetric = "gold_num_recent_open_cls"
	completedCommentCycle  = "gold_comment_monitoring"

	timePeriodOfCLsToCheck = 2 * time.Hour
)

type Impl struct {
	crs             code_review.Client
	store           clstore.Store
	instanceURL     string
	logCommentsOnly bool
	messageTemplate string
	search          search.SearchAPI

	liveness metrics2.Liveness
	// used to mock the time in tests
	now func() time.Time
}

func New(c code_review.Client, s clstore.Store, search search.SearchAPI, messageTemplate, instanceURL string, logCommentsOnly bool) *Impl {
	return &Impl{
		crs:             c,
		store:           s,
		instanceURL:     instanceURL,
		logCommentsOnly: logCommentsOnly,
		messageTemplate: messageTemplate,
		liveness:        metrics2.NewLiveness(completedCommentCycle),
		search:          search,
		now:             time.Now,
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
	recent := i.now().Add(-timePeriodOfCLsToCheck)
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
		total += len(xcl)
		chunks := len(xcl) / shards
		if chunks < 1 {
			chunks = 1
		}
		beforeCount := len(stillOpen)
		err := util.ChunkIterParallel(ctx, len(xcl), chunks, func(ctx context.Context, startIdx int, endIdx int) error {
			for _, cl := range xcl[startIdx:endIdx] {
				if err := ctx.Err(); err != nil {
					return skerr.Wrap(err)
				}
				open, err := i.updateCLInStoreIfAbandoned(ctx, cl)
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

		// We paged forward and didn't identify any new CLs, so we are done.
		if beforeCount == len(stillOpen) {
			break
		}

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
	metrics2.GetInt64Metric(numRecentOpenCLsMetric, nil).Update(int64(len(stillOpen)))
	sklog.Infof("There were originally %d recent open CLs; after checking with CRS there are %d still open", total, len(stillOpen))

	for _, cl := range stillOpen {
		xps, err := i.store.GetPatchSets(ctx, cl.SystemID)
		if err != nil {
			return skerr.Wrapf(err, "looking for patchsets on open CL %s", cl.SystemID)
		}
		if len(xps) == 0 {
			// It is unclear why this happens. I wonder if it's a subtle race condition where
			// ingestion has created a CL, but not yet created the PS under the CL?
			sklog.Warningf("CL %s had no patchsets?", cl.SystemID)
			continue
		}
		// We only want to comment on the most recent PS and only if it has untriaged digests.
		// Earlier PS are probably obsolete.
		mostRecentPS := xps[len(xps)-1]
		if !mostRecentPS.CommentedOnCL && cl.Updated.After(mostRecentPS.LastCheckedIfCommentNecessary) {
			numUntriaged, indexTS := i.searchIndexForNewUntriagedDigests(ctx, cl.SystemID, mostRecentPS.SystemID)
			mostRecentPS.LastCheckedIfCommentNecessary = indexTS
			if numUntriaged > 0 {
				if err := i.maybeCommentOn(ctx, cl, mostRecentPS, numUntriaged); err != nil {
					return skerr.Wrap(err)
				}
				mostRecentPS.CommentedOnCL = true
			}
			if err := i.store.PutPatchSet(ctx, mostRecentPS); err != nil {
				return skerr.Wrapf(err, "updating PS %#v", mostRecentPS)
			}
		}
	}
	i.liveness.Reset()
	return nil
}

// maybeCommentOn either comments on the given CL/PS that there are untriaged digests on it or
// logs if this commenter is configured to not actually comment.
func (i *Impl) maybeCommentOn(ctx context.Context, cl code_review.ChangeList, ps code_review.PatchSet, untriagedDigests int) error {
	crs := i.crs.System()
	if i.logCommentsOnly {
		sklog.Infof("Should comment on CL %s with message %s", cl.SystemID, i.untriagedMessage(crs, cl, ps, untriagedDigests))
		return nil
	}
	if err := i.crs.CommentOn(ctx, cl.SystemID, i.untriagedMessage(crs, cl, ps, untriagedDigests)); err != nil {
		if err == code_review.ErrNotFound {
			sklog.Warningf("Cannot comment on %s CL %s because it does not exist", i.crs.System(), cl.SystemID)
			return nil
		}
		return skerr.Wrapf(err, "commenting on %s CL %s", i.crs.System(), cl.SystemID)
	}
	return nil
}

// untriagedMessage returns a message about untriaged images on the given CL/PS.
func (i *Impl) untriagedMessage(crs string, cl code_review.ChangeList, ps code_review.PatchSet, untriagedDigests int) string {
	return fmt.Sprintf(i.messageTemplate, untriagedDigests, ps.Order, i.instanceURL, crs, cl.SystemID)
}

// updateCLInStoreIfAbandoned checks with the CRS to see if the cl is still Open. If it is, it
// returns true. If it is Abandoned, it stores the updated CL in the store and returns false.
// If the CL is Landed, it returns false and *does not update anything* in the store.
func (i *Impl) updateCLInStoreIfAbandoned(ctx context.Context, cl code_review.ChangeList) (bool, error) {
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
	// If the CRS is reporting a CL as Landed, but we think it to be Open, that means that
	// the code_review.ChangeListLandedUpdater hasn't had a chance to process it yet, which is
	// necessary to smoothly merge the Expectations from the CL into master.
	if up.Status == code_review.Landed {
		return false, nil
	}
	// Store the latest one from the CRS (with new timestamp) to the clstore so we
	// remember it is abandoned in the future. This also catches things like the cl Subject
	// changing since it was opened.
	up.Updated = i.now()
	if err := i.store.PutChangeList(ctx, up); err != nil {
		return false, skerr.Wrapf(err, "storing CL %s", up.SystemID)
	}
	return false, nil
}

// searchIndexForNewUntriagedDigests returns the number of digests for a given CL and patchset that
// are untriaged, unignored and not on the master branch. It also returns the freshness of this
// data. If there is an error querying the search index, it is logged and 0 is returned.
func (i *Impl) searchIndexForNewUntriagedDigests(ctx context.Context, clID, psID string) (int, time.Time) {
	digestList, err := i.search.UntriagedUnignoredTryJobExclusiveDigests(ctx, tjstore.CombinedPSID{
		CL:  clID,
		CRS: i.crs.System(),
		PS:  psID,
	})
	if err != nil {
		sklog.Errorf("could not check search index for untriaged digests for CL %s: %s", clID, err)
		return 0, i.now()
	}
	return len(digestList.Digests), digestList.TS

}

// Make sure Impl fulfills the code_review.ChangeListCommenter interface.
var _ code_review.ChangeListCommenter = (*Impl)(nil)
