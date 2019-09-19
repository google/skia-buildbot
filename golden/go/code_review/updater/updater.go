// package updater contains an implementation of the code_review.Updater interface.
// It should be CRS-agnostic.
package updater

import (
	"context"
	"time"

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
			return skerr.Fmt("cl %v was supposed to have landed, but wasn't according to %s", cl, crs)
		}

		id := cl.SystemID
		cl, err = u.store.GetChangeList(ctx, id)
		if err == clstore.ErrNotFound {
			// Wasn't in clstore, so there was data from TryJobs associated with that ChangeList
			// So there can't be any expectations associated with it.
			continue
		}
		if err != nil {
			return skerr.Wrapf(err, "retrieving CL from store with id %s", id)
		}
		if cl.Status == code_review.Landed {
			// We have already written this data.
			continue
		}

		clExp := u.expStore.ForChangeList(cl.SystemID, crs)
		e, err := clExp.Get()
		if err != nil {
			return skerr.Wrapf(err, "getting CLExpectations for %s (%s)", cl.SystemID, crs)
		}
		if len(e) > 0 {
			if err := u.expStore.AddChange(ctx, e, cl.Owner); err != nil {
				return skerr.Wrapf(err, "writing CLExpectations for %s (%s) to master: %v", cl.SystemID, crs, e)
			}
		}

		cl.Status = code_review.Landed
		cl.Updated = time.Now()
		if err := u.store.PutChangeList(ctx, cl); err != nil {
			return skerr.Wrapf(err, "storing CL %v to store", cl)
		}
	}
	return nil
}

// Make sure Impl fulfills the code_review.Updater interface.
var _ code_review.Updater = (*Impl)(nil)
