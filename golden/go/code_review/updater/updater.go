// Package updater contains an implementation of the code_review.ChangeListLandedUpdater interface.
// It should be CRS-agnostic.
package updater

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/expectations"
)

type Impl struct {
	expStore      expectations.Store
	reviewSystems []clstore.ReviewSystem
}

func New(e expectations.Store, reviewSystems []clstore.ReviewSystem) *Impl {
	return &Impl{
		expStore:      e,
		reviewSystems: reviewSystems,
	}
}

// UpdateChangeListsAsLanded implements the code_review.ChangeListLandedUpdater interface.
// This implementation is *not* thread safe.
func (u *Impl) UpdateChangeListsAsLanded(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	if len(commits) > 100 {
		// For new instances, or very sparse instances, we'll have many many many commits to check,
		// which can make startup take tens of minutes (due to having to poll the CRS about many
		// many commits [and there's a very conservative QPS limit set]).
		sklog.Warningf("Got more than 100 commits to update. This usually means we are starting up; We'll only check the last 100.")
		commits = commits[len(commits)-100:]
	}
	for _, c := range commits {
		var clID string
		var system clstore.ReviewSystem
		for _, rs := range u.reviewSystems {
			// GetChangeListIDForCommit is smart enough to distinguish between two different Gerrit
			// systems because it looks at the review URL in the CL message.
			if id, err := rs.Client.GetChangeListIDForCommit(ctx, c); err == nil {
				clID = id
				system = rs
				break
			}
		}
		if clID == "" {
			sklog.Warningf("Saw a commit %s that did not line up with a code review", c.Hash)
			continue
		}

		storedCL, err := system.Store.GetChangeList(ctx, clID)
		if err == clstore.ErrNotFound {
			// Wasn't in clstore, so there was no data from TryJobs associated with that
			//  ChangeList, so there can't be any expectations associated with it.
			continue
		}
		if err != nil {
			return skerr.Wrap(err)
		}
		if storedCL.Status == code_review.Landed {
			// We have already written this data.
			continue
		}

		cl, err := system.Client.GetChangeList(ctx, clID)
		if err == code_review.ErrNotFound {
			return skerr.Fmt("somehow got an invalid CLID %s from commit %s", clID, c.Hash)
		}
		if err != nil {
			return skerr.Wrapf(err, "querying CRS for CL %s", c.Hash)
		}
		if cl.Status != code_review.Landed {
			return skerr.Fmt("cl %v of revision %s was supposed to have landed, but wasn't according to %s", cl, c.Hash, system.ID)
		}

		// Write the expectations (if any) for the CL to master
		clExp := u.expStore.ForChangeList(cl.SystemID, system.ID)
		e, err := clExp.Get(ctx)
		if err != nil {
			return skerr.Wrapf(err, "getting CLExpectations for %s (%s)", cl.SystemID, system.ID)
		}
		if !e.Empty() {
			delta := expectations.AsDelta(e)
			if err := u.expStore.AddChange(ctx, delta, cl.Owner); err != nil {
				return skerr.Wrapf(err, "writing CLExpectations for %s (%s) to master: %v", cl.SystemID, system.ID, e)
			}
		}
		// cl.Status must be Landed at this point and the CRS has set the cl's Updated time to
		// the time that it was closed or marked as landed.
		if err := system.Store.PutChangeList(ctx, cl); err != nil {
			return skerr.Wrapf(err, "storing CL %v to store", cl)
		}
	}
	return nil
}

// Make sure Impl fulfills the code_review.ChangeListLandedUpdater interface.
var _ code_review.ChangeListLandedUpdater = (*Impl)(nil)
