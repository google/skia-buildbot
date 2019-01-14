package storage

import (
	"sync"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
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

	lastWrittenBaselines map[string]string
	baselineCache        map[string]*baseline.CommitableBaseLine
	mutex                sync.RWMutex
}

// NewBaseliner creates a new instance of Baseliner.
func NewBaseliner(gStorageClient *GStorageClient, expectationsStore expstorage.ExpectationsStore, issueExpStoreFactory expstorage.IssueExpStoreFactory, tryjobStore tryjobstore.TryjobStore) *Baseliner {
	return &Baseliner{
		gStorageClient:       gStorageClient,
		expectationsStore:    expectationsStore,
		issueExpStoreFactory: issueExpStoreFactory,
		tryjobStore:          tryjobStore,
		lastWrittenBaselines: map[string]string{},
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

	perCommitBaselines, err := b.calcMasterBaselines(tile)
	if err != nil {
		return skerr.Fmt("Error getting master baseline: %s", err)
	}

	b.mutex.Lock()
	lastWritten := b.lastWrittenBaselines
	b.mutex.Unlock()

	// Write the ones to disk that have not been written
	writeBaselines := make(util.StringSet, len(perCommitBaselines))
	for commit, bLine := range perCommitBaselines {
		md5Sum, ok := lastWritten[commit]
		if ok && md5Sum == bLine.MD5 {
			continue
		}
		writeBaselines[commit] = true
	}

	written := make(map[string]string, len(writeBaselines))
	for commit, bLine := range perCommitBaselines {
		if _, ok := writeBaselines[commit]; !ok {
			written[commit] = bLine.MD5
			continue
		}

		// Write the baseline to GCS.
		_, err := b.gStorageClient.WriteBaseLine(bLine)
		if err != nil {
			return skerr.Fmt("Error writing baseline to GCS: %s", err)
		}
		written[commit] = bLine.MD5
	}

	// Swap out the baseline cache and the list of last written files.
	b.mutex.Lock()
	b.baselineCache = perCommitBaselines
	b.lastWrittenBaselines = written
	b.mutex.Unlock()
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
	isIssue := issueID > 0

	if isIssue {
		commitHash = ""
	}

	var masterBaseline *baseline.CommitableBaseLine
	var issueBaseline *baseline.CommitableBaseLine
	var egroup errgroup.Group

	// Retrieve the baseline on master.
	egroup.Go(func() error {
		var err error
		masterBaseline, err = b.getMasterExpectations(commitHash)
		return err
	})

	if isIssue {
		egroup.Go(func() error {
			var err error
			issueBaseline, err = b.getIssueExpectations(issueID)
			return err
		})
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	if isIssue {
		masterBaseline.Baseline.Update(issueBaseline.Baseline)
	}
	return masterBaseline, nil
}

// calcMasterBaselines retrieves the master baseline based on the given tile.
func (b *Baseliner) calcMasterBaselines(tile *tiling.Tile) (map[string]*baseline.CommitableBaseLine, error) {
	exps, err := b.expectationsStore.Get()
	if err != nil {
		return nil, skerr.Fmt("Unable to retrieve expectations: %s", err)
	}

	return baseline.GetBaselinesPerCommit(exps, tile), nil
}

func (b *Baseliner) updateBaselineCache(tile *tiling.Tile) {
}

func (b *Baseliner) currentHEAD() string {
	return ""
}

func (b *Baseliner) getMasterExpectations(commitHash string) (*baseline.CommitableBaseLine, error) {
	// 	if masterExp != nil {
	// 		return nil
	// 	}

	// 	masterBaseline, err = b.gStorageClient.ReadBaseline(b.currentHEAD(), 0)
	// 	sklog.Infof("Master: %s    %s", commitHash, spew.Sdump(masterBaseline))
	// 	return err
	// })

	return nil, nil
}

func (b *Baseliner) getIssueExpectations(issueID int64) (*baseline.CommitableBaseLine, error) {
	return nil, nil
}
