// Package updater contains an implementation of the code_review.Updater interface.
// It should be CRS-agnostic.
package updater

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/expstorage"
)

type Impl struct {
	client   code_review.Client
	expStore expstorage.ExpectationsStore
	store    clstore.Store
}

func New(c code_review.Client, e expstorage.ExpectationsStore, s clstore.Store) *Impl {
	return &Impl{
		client:   c,
		expStore: e,
		store:    s,
	}
}

// UpdateChangeListsAsLanded implements the code_review.Updater interface.
// This implementation is *not* thread safe.
func (u *Impl) UpdateChangeListsAsLanded(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	if len(commits) > 100 {
		// For new instances, or very sparse instances, we'll have many many many commits to check,
		// which can make startup take tens of minutes (due to having to poll the CRS about many
		// many commits [and there's a very conservative QPS limit set]).
		sklog.Warningf("Got more than 100 commits to update. This usually means we are starting up; We'll only check the last 100.")
		commits = commits[len(commits)-100:]
	}
	crs := u.client.System()
	for _, c := range commits {
		cl, err := u.client.GetChangeListForCommit(ctx, c)
		if err == code_review.ErrNotFound {
			sklog.Warningf("Saw a commit %s that did not line up with a code review", c.Hash)
			continue
		}
		if err != nil {
			return skerr.Wrapf(err, "looking up commit %s", c.Hash)
		}
		if cl.Status != code_review.Landed {
			return skerr.Fmt("cl %v of revision %s was supposed to have landed, but wasn't according to %s", cl, c.Hash, crs)
		}

		storedCL, err := u.store.GetChangeList(ctx, cl.SystemID)
		if err == clstore.ErrNotFound {
			// Wasn't in clstore, so there was no data from TryJobs associated with that
			//  ChangeList, so there can't be any expectations associated with it.
			continue
		}
		if err != nil {
			return skerr.Wrapf(err, "retrieving CL from store with id %s", cl.SystemID)
		}
		if storedCL.Status == code_review.Landed {
			// We have already written this data.
			continue
		}

		clExp := u.expStore.ForChangeList(cl.SystemID, crs)
		e, err := clExp.Get(ctx)
		if err != nil {
			return skerr.Wrapf(err, "getting CLExpectations for %s (%s)", cl.SystemID, crs)
		}
		if !e.Empty() {
			delta := expstorage.AsDelta(e)
			if err := u.expStore.AddChange(ctx, delta, cl.Owner); err != nil {
				return skerr.Wrapf(err, "writing CLExpectations for %s (%s) to master: %v", cl.SystemID, crs, e)
			}
		}
		// cl.Status must be Landed at this point
		if err := u.store.PutChangeList(ctx, cl); err != nil {
			return skerr.Wrapf(err, "storing CL %v to store", cl)
		}
	}
	return nil
}

// Make sure Impl fulfills the code_review.Updater interface.
var _ code_review.Updater = (*Impl)(nil)
