package storage

import (
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
	"golang.org/x/sync/errgroup"
)

// TODO(stephana): Tune issueCacheSize by either finding a good value that works across instance
// or one that can be tuned.

// TODO(stephana): Add tests for all functions in this file.

const (
	// issueCacheSize is the size of the baselines cache for issue.
	issueCacheSize = 10000
)

// TODO(stephana): Baseliner needs to merged into the baseline package and
// the nomenclature should either change to Expectations or make a it clearer that
// baselines are synonymous to expectations.

// Baseliner is a helper type that provides functions to write baselines (expecations) to
// GCS and retrieve them. Other packages use it to continuously write expecations to GCS
// as they become available.
type Baseliner struct {
	gStorageClient       *GStorageClient
	expectationsStore    expstorage.ExpectationsStore
	issueExpStoreFactory expstorage.IssueExpStoreFactory
	tryjobStore          tryjobstore.TryjobStore
	vcs                  vcsinfo.VCS

	// mutex protects lastWrittenBaselines, baselineCache and currentTile
	mutex sync.RWMutex

	// lastWrittenBaselines maps[commit_hash]MD5_sum_of_baseline to keep track whether a baseline for
	// a specific commit has been written already and whether we need to write it again (different MD5)
	lastWrittenBaselines map[string]string

	// baselineCache caches the baselines all commits of the current tile.
	baselineCache map[string]*baseline.CommitableBaseLine

	// currentTile is the newest tile.
	currentTile *tiling.Tile

	// issueBaselineCache caches baselines for issue by mapping from issueID to baseline.
	issueBaselineCache *lru.Cache
}

// NewBaseliner creates a new instance of Baseliner.
func NewBaseliner(gStorageClient *GStorageClient, expectationsStore expstorage.ExpectationsStore, issueExpStoreFactory expstorage.IssueExpStoreFactory, tryjobStore tryjobstore.TryjobStore, vcs vcsinfo.VCS) (*Baseliner, error) {
	cache, err := lru.New(issueCacheSize)
	if err != nil {
		return nil, skerr.Fmt("Error allocating cache: %s", err)
	}

	return &Baseliner{
		gStorageClient:       gStorageClient,
		expectationsStore:    expectationsStore,
		issueExpStoreFactory: issueExpStoreFactory,
		tryjobStore:          tryjobStore,
		vcs:                  vcs,
		issueBaselineCache:   cache,
		lastWrittenBaselines: map[string]string{},
	}, nil
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

	// Get the current list of files that have been written.
	b.mutex.Lock()
	lastWritten := b.lastWrittenBaselines
	b.mutex.Unlock()

	// Write the ones to disk that have not been written
	written := make(map[string]string, len(perCommitBaselines))
	var wMutex sync.Mutex
	var egroup errgroup.Group

	for commit, bLine := range perCommitBaselines {
		// If we have written this baseline before, we mark it as written and process the next one.
		if md5Sum, ok := lastWritten[commit]; ok && md5Sum == bLine.MD5 {
			wMutex.Lock()
			written[commit] = bLine.MD5
			wMutex.Unlock()
			continue
		}

		func(commit string, bLine *baseline.CommitableBaseLine) {
			egroup.Go(func() error {
				// Write the baseline to GCS.
				_, err := b.gStorageClient.WriteBaseLine(bLine)
				if err != nil {
					return skerr.Fmt("Error writing baseline to GCS: %s", err)
				}
				wMutex.Lock()
				defer wMutex.Unlock()

				written[commit] = bLine.MD5
				return nil
			})
		}(commit, bLine)
	}

	// Swap out the baseline cache and the list of last written files.
	b.mutex.Lock()
	b.currentTile = tile
	b.baselineCache = perCommitBaselines
	b.lastWrittenBaselines = written
	b.mutex.Unlock()
	return nil
}

// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
func (b *Baseliner) PushIssueBaseline(issueID int64, tile *tiling.Tile, tallies *tally.Tallies) error {
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
	baseLine, err := baseline.GetBaselineForIssue(issueID, tryjobs, tryjobResults, exp, tile.Commits, talliesByTest)
	if err != nil {
		return skerr.Fmt("Error calculating issue baseline: %s", err)
	}

	// Add it to the cache.
	_ = b.issueBaselineCache.Add(issueID, baseLine)

	if !b.CanWriteBaseline() {
		return skerr.Fmt("Trying to write baseline while GCS path is not configured.")
	}

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
			val, ok := b.issueBaselineCache.Get(issueID)
			if ok {
				issueBaseline = val.(*baseline.CommitableBaseLine)
				return nil
			}

			var err error
			issueBaseline, err = b.gStorageClient.ReadBaseline("", issueID)
			if err != nil {
				return err
			}

			// If no baseline was found. We place an empty one.
			if issueBaseline == nil {
				issueBaseline = baseline.EmptyBaseline(nil, nil)
			}

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
	return baseline.GetBaselinesPerCommit(exps, tile)
}

func (b *Baseliner) getMasterExpectations(commitHash string) (*baseline.CommitableBaseLine, error) {
	b.mutex.RLock()
	cache := b.baselineCache
	tile := b.currentTile
	b.mutex.RUnlock()

	if commitHash == "" {
		// If we have no tile yet, we cannot get the HEAD of it.
		if tile == nil {
			return baseline.EmptyBaseline(nil, nil), nil
		}
		commitHash = tile.Commits[tile.LastCommitIndex()].Hash
	}

	if bLine, ok := cache[commitHash]; ok {
		return bLine, nil
	}

	// We did not find it in the cache so lets load it from GCS.
	ret, err := b.gStorageClient.ReadBaseline(commitHash, 0)
	if err != nil {
		return nil, err
	}

	// Look up the commit to see if it's valid.
	if ret == nil {
		// TODO(stephana): This should verify that the given commit is valid, i.e. check it against
		// a git commit.
		sklog.Infof("Commit %s not found", commitHash)
		ret = baseline.EmptyBaseline(nil, nil)
	}
	return ret, nil
}

// fromLongCommit converts a *vcsinfo.LongCommit to a *tiling.Commit
func fromLongCommit(lc *vcsinfo.LongCommit) *tiling.Commit {
	return &tiling.Commit{
		CommitTime: lc.Timestamp.Unix(),
		Hash:       lc.Hash,
		Author:     lc.Author,
	}
}
