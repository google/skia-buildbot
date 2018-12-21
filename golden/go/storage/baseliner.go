package storage

import (
	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

// TODO(stephana): Baseliner needs to merged into the baseline package and
// the nomenclature should either change to Expectations or make a it clearer that
// baselines are synonymous to expectations.
// This needs to be extended to store per-commit expectations.

// Baseliner is a helper type that provides functions to write baselines (expecations) to
// GCS and retrieve them. Other packages use it to continuously write expecations to GCS
// as they become available.
type Baseliner struct {
	gStorageClient       *GStorageClient
	expectationsStore    expstorage.ExpectationsStore
	issueExpStoreFactory expstorage.IssueExpStoreFactory
	tryjobStore          tryjobstore.TryjobStore
}

// NewBaseliner creates a new instance of Baseliner.
func NewBaseliner(gStorageClient *GStorageClient, expectationsStore expstorage.ExpectationsStore, issueExpStoreFactory expstorage.IssueExpStoreFactory, tryjobStore tryjobstore.TryjobStore) *Baseliner {
	return &Baseliner{
		gStorageClient:       gStorageClient,
		expectationsStore:    expectationsStore,
		issueExpStoreFactory: issueExpStoreFactory,
		tryjobStore:          tryjobStore,
	}
}

// CanWriteBaseline returns true if this instance was configured to write baseline files.
func (b *Baseliner) CanWriteBaseline() bool {
	return (b.gStorageClient != nil) && (b.gStorageClient.options.BaselineGSPath != "")
}

// PushMasterBaselines writes the baselines for the master branch to GCS.
func (b *Baseliner) PushMasterBaselines(tile *tiling.Tile) error {
	if !b.CanWriteBaseline() {
		return skerr.Fmt("Trying to write baseline while GCS path is not configured.")
	}

	_, baseLine, err := b.getMasterBaseline(tile)
	if err != nil {
		return skerr.Fmt("Error retrieving master baseline: %s", err)
	}

	if baseLine == nil {
		sklog.Infof("No baseline available.")
		return nil
	}

	// Write the baseline to GCS.
	outputPath, err := b.gStorageClient.WriteBaseLine(baseLine)
	if err != nil {
		return skerr.Fmt("Error writing baseline to GCS: %s", err)
	}
	sklog.Infof("Baseline for master written to %s.", outputPath)
	return nil
}

// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
func (b *Baseliner) PushIssueBaseline(issueID int64, tile *tiling.Tile, tallies *tally.Tallies) error {
	if !b.CanWriteBaseline() {
		return skerr.Fmt("Trying to write baseline while GCS path is not configured.")
	}

	issueExpStore := b.issueExpStoreFactory(issueID)
	exp, err := issueExpStore.Get()
	if err != nil {
		return skerr.Fmt("Unable to get issue expecations: %s", err)
	}

	tryjobs, tryjobResults, err := b.tryjobStore.GetTryjobs(issueID, nil, true, true)
	if err != nil {
		return skerr.Fmt("Unable to get TryjobResults")
	}
	talliesByTest := tallies.ByTest()
	baseLine := baseline.GetBaselineForIssue(issueID, tryjobs, tryjobResults, exp, tile.Commits, talliesByTest)

	// Write the baseline to GCS.
	outputPath, err := b.gStorageClient.WriteBaseLine(baseLine)
	if err != nil {
		return skerr.Fmt("Error writing baseline to GCS: %s", err)
	}
	sklog.Infof("Baseline for issue %d written to %s.", issueID, outputPath)
	return nil
}

// FetchBaseline fetches the complete baseline for the given Gerrit issue by
// loading the master baseline and the issue baseline from GCS and combining
// them. If either of them doesn't exist an empty baseline is assumed.
func (b *Baseliner) FetchBaseline(commitHash string, issueID int64, patchsetID int64) (*baseline.CommitableBaseLine, error) {
	var masterBaseline *baseline.CommitableBaseLine
	var issueBaseline *baseline.CommitableBaseLine

	var egroup errgroup.Group
	egroup.Go(func() error {
		var err error
		masterBaseline, err = b.gStorageClient.ReadBaseline(commitHash, 0)
		sklog.Infof("Master: %s    %s", commitHash, spew.Sdump(masterBaseline))
		return err
	})

	if issueID > 0 {
		egroup.Go(func() error {
			var err error
			issueBaseline, err = b.gStorageClient.ReadBaseline(commitHash, issueID)
			sklog.Infof("issue %s   %d: %s", commitHash, issueID, spew.Sdump(issueBaseline))
			return err
		})
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	if issueBaseline != nil {
		masterBaseline.Baseline.Update(issueBaseline.Baseline)
	}
	return masterBaseline, nil
}

// getMasterBaseline retrieves the master baseline based on the given tile.
func (b *Baseliner) getMasterBaseline(tile *tiling.Tile) (types.Expectations, *baseline.CommitableBaseLine, error) {
	exps, err := b.expectationsStore.Get()
	if err != nil {
		return nil, nil, skerr.Fmt("Unable to retrieve expectations: %s", err)
	}

	return exps, baseline.GetBaselineForMaster(exps, tile), nil
}
