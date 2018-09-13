package storage

import (
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
)

type Baseliner struct {
	gStorageClient       *GStorageClient
	expectationsStore    expstorage.ExpectationsStore
	issueExpStoreFactory expstorage.IssueExpStoreFactory
	tryjobStore          tryjobstore.TryjobStore
}

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
		return sklog.FmtErrorf("Trying to write baseline while GCS path is not configured.")
	}

	changedBaselines, err := b.getMasterBaselines(tile)
	if err != nil {
		return err
	}

	var egroup errgroup.Group
	for _, bLine := range changedBaselines {
		func(bLine *baseline.CommitableBaseLine) {
			egroup.Go(func() error {
				// Write the baseline to GCS.
				outputPath, err := b.gStorageClient.WriteBaseLineForCommit(bLine)
				if err != nil {
					return sklog.FmtErrorf("Error writing baseline to GCS: %s", err)
				}
				sklog.Infof("Baselines for master written to %s.", outputPath)
				return nil
			})
		}(bLine)
	}

	if err := egroup.Wait(); err != nil {
		return sklog.FmtErrorf("Error writing baselines to GCS: %s", err)
	}

	return b.saveBaselineState(nil)
}

func (b *Baseliner) saveBaselineState(bLineState map[string]string) error {
	return nil
}

func (b *Baseliner) loadBaselineState(tile *tiling.Tile) (map[string]string, error) {
	return nil, nil
}

// getMasterBaseline retrieves the master baseline based on the given tile.
func (b *Baseliner) getMasterBaselines(tile *tiling.Tile) ([]*baseline.CommitableBaseLine, error) {
	_, err := b.loadBaselineState(tile)
	if err != nil {
		return nil, err
	}

	// exps, err := b.expectationsStore.Get()
	// if err != nil {
	// 	return nil, sklog.FmtErrorf("Unable to retrieve expectations: %s", err)
	// }

	// tileHash := types.HashGoldTile(tile)
	// changedColumnHashes := make([]string, len(tile.Commits))
	// for idx := range commits {
	// 	if prev == nil || (prev.ColumnHashes[idx] != tileHash.ColumnHashes[idx]) {
	// 		commits[idx] = tileHash.ColumnHashes
	// 	}
	// }

	// diffHashes := getTileHashDiff(b.previousTileHash, tileHash)

	// return exps, baseline.GetBaselineForMasterCommit(exps, tile), nil
	return nil, nil
}

// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
func (b *Baseliner) PushIssueBaseline(issueID int64, tile *tiling.Tile, tallies *tally.Tallies) error {
	if !b.CanWriteBaseline() {
		return sklog.FmtErrorf("Trying to write baseline while GCS path is not configured.")
	}

	issueExpStore := b.issueExpStoreFactory(issueID)
	exp, err := issueExpStore.Get()
	if err != nil {
		return sklog.FmtErrorf("Unable to get issue expecations: %s", err)
	}

	tryjobs, tryjobResults, err := b.tryjobStore.GetTryjobs(issueID, nil, true, true)
	if err != nil {
		return sklog.FmtErrorf("Unable to get TryjobResults")
	}
	talliesByTest := tallies.ByTest()
	baseLine := baseline.GetBaselineForIssue(issueID, tryjobs, tryjobResults, exp, tile.Commits, talliesByTest)

	// Write the baseline to GCS.
	outputPath, err := b.gStorageClient.WriteBaseLine(baseLine)
	if err != nil {
		return sklog.FmtErrorf("Error writing baseline to GCS: %s", err)
	}
	sklog.Infof("Baseline for issue %d written to %s.", issueID, outputPath)
	return nil
}

// FetchBaseline fetches the complete baseline for the given Gerrit issue by
// loading the master baseline and the issue baseline from GCS and combining
// them. If either of them doesn't exist an empty baseline is assumed.
func (b *Baseliner) FetchBaseline(issueID int64) (*baseline.CommitableBaseLine, error) {
	var masterBaseline *baseline.CommitableBaseLine
	var issueBaseline *baseline.CommitableBaseLine

	var egroup errgroup.Group
	egroup.Go(func() error {
		var err error
		masterBaseline, err = b.gStorageClient.ReadBaseline(0)
		return err
	})

	if issueID > 0 {
		egroup.Go(func() error {
			var err error
			issueBaseline, err = b.gStorageClient.ReadBaseline(issueID)
			return err
		})
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	if issueBaseline != nil {
		masterBaseline.Baseline.Merge(issueBaseline.Baseline)
	}
	return masterBaseline, nil
}
