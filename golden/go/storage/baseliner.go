package storage

import (
	"context"
	"math"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
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

// Baseliner is a helper type that provides functions to write baselines (expectations) to
// GCS and retrieve them. Other packages use it to continuously write expectations to GCS
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

	// cpxTile is the latest tile we have.
	currCpxTile *types.ComplexTile

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
// If cpxTile is nil the tile of the last call to PushMasterBaselines is used. If the function
// was never called before and cpxTile is nil, an error is returned.
// If targetHash != "" we also return the baseline for corresponding commit as the first return
// value. Otherwise the first return value is nil.
// It is assumed that the target commit is one of the commits that are written as part of this call.
func (b *Baseliner) PushMasterBaselines(cpxTile *types.ComplexTile, targetHash string) (*baseline.CommitableBaseLine, error) {
	defer timer.New("PushmasterBaselines").Stop()
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if cpxTile == nil {
		cpxTile = b.currCpxTile
	}
	if cpxTile == nil {
		return nil, skerr.Fmt("Received nil tile and no previous tile defined")
	}

	if !b.CanWriteBaseline() {
		return nil, skerr.Fmt("Trying to write baseline while GCS path is not configured.")
	}

	// Calculate the baselines for the master tile.
	exps, err := b.expectationsStore.Get()
	if err != nil {
		return nil, skerr.Fmt("Unable to retrieve expectations: %s", err)
	}

	// Make sure we have all commits, not just the ones that are in the tile. Currently a tile is
	// fetched in intervals. New commits might have arrived since the last tile was read. Below we
	// extrapolate the baselines of the new commits to be identical to the last commit in the tile.
	// As new data arrive in the next tile, we update the baselines for these commits.
	tileCommits := cpxTile.AllCommits()
	extraCommits, err := b.getCommitsSince(tileCommits[len(tileCommits)-1])
	if err != nil {
		return nil, err
	}

	perCommitBaselines, err := baseline.GetBaselinesPerCommit(exps, cpxTile, extraCommits)
	if err != nil {
		return nil, skerr.Fmt("Error getting master baseline: %s", err)
	}

	// Get the current list of files that have been written.
	lastWritten := b.lastWrittenBaselines

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

	// If a specific baseline was also requested we find it now
	var ret *baseline.CommitableBaseLine
	if targetHash != "" {
		var ok bool
		ret, ok = perCommitBaselines[targetHash]
		if !ok {
			return nil, skerr.Fmt("Unable to find requested commit %s", targetHash)
		}
	}

	// Swap out the baseline cache and the list of last written files.
	b.currCpxTile = cpxTile
	b.baselineCache = perCommitBaselines
	b.lastWrittenBaselines = written
	return ret, nil
}

// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
func (b *Baseliner) PushIssueBaseline(issueID int64, cpxTile *types.ComplexTile, tallies *tally.Tallies) error {
	issueExpStore := b.issueExpStoreFactory(issueID)
	exp, err := issueExpStore.Get()
	if err != nil {
		return skerr.Fmt("Unable to get issue expectations: %s", err)
	}

	tryjobs, tryjobResults, err := b.tryjobStore.GetTryjobs(issueID, nil, true, true)
	if err != nil {
		return skerr.Fmt("Unable to get TryjobResults")
	}

	baseLine, err := baseline.GetBaselineForIssue(issueID, tryjobs, tryjobResults, exp, cpxTile.AllCommits())
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
// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
// returned (omitting the baselines of the master branch). This is primarily used for debugging.
func (b *Baseliner) FetchBaseline(commitHash string, issueID int64, patchsetID int64, issueOnly bool) (*baseline.CommitableBaseLine, error) {
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
		if issueOnly {
			// Only return the portion of the baseline that would be contributed by the issue
			issueBaseline.Issue = issueID
			masterBaseline = issueBaseline
		} else {
			// Clone the retrieved baseline before we inject the issue information.
			masterBaseline.Baseline.Update(issueBaseline.Baseline)
			masterBaseline.Issue = issueID
		}
	}
	return masterBaseline, nil
}

// getCommitSince returns all the commits have been added to the repo since the given commit.
// The returned instances of tiling.Commit do not contain a valid Author field.
func (b *Baseliner) getCommitsSince(firstCommit *tiling.Commit) ([]*tiling.Commit, error) {
	defer timer.New("getCommitsSince").Stop()

	// If there is an underlying gitstore retrieve it, otherwise this function becomes a no-op.
	gitStoreBased, ok := b.vcs.(gitstore.GitStoreBased)
	if !ok {
		return []*tiling.Commit{}, nil
	}

	gitStore := gitStoreBased.GetGitStore()
	ctx := context.TODO()
	startTime := time.Unix(firstCommit.CommitTime, 0)
	endTime := startTime.Add(time.Second)
	branch := b.vcs.GetBranch()
	commits, err := gitStore.RangeByTime(ctx, startTime, endTime, branch)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, skerr.Fmt("No commits found while querying for commit %s", firstCommit.Hash)
	}

	var target *vcsinfo.IndexCommit
	for _, c := range commits {
		if c.Hash == firstCommit.Hash {
			target = c
		}
	}

	if target == nil {
		return nil, skerr.Fmt("Commit %s not found in gitstore", firstCommit.Hash)
	}

	// Fetch all commits after the first one which we already have.
	if commits, err = gitStore.RangeN(ctx, target.Index, int(math.MaxInt32), branch); err != nil {
		return nil, err
	}

	ret := make([]*tiling.Commit, len(commits))
	for idx, c := range commits {
		// Note: For the task at hand we don't need to populate the Author field of tiling.Commit.
		ret[idx] = &tiling.Commit{
			Hash:       c.Hash,
			CommitTime: c.Timestamp.Unix(),
		}
	}

	return ret[1:], nil
}

func (b *Baseliner) getMasterExpectations(commitHash string) (*baseline.CommitableBaseLine, error) {
	b.mutex.RLock()
	cache := b.baselineCache
	cpxTile := b.currCpxTile
	b.mutex.RUnlock()

	// If no commit hash was given use current HEAD.
	if commitHash == "" {
		// If we have no tile yet, we cannot get the HEAD of it.
		if cpxTile == nil {
			return baseline.EmptyBaseline(nil, nil), nil
		}
		// Get the last commit that has data.
		allCommits := cpxTile.AllCommits()
		commitHash = allCommits[len(allCommits)-1].Hash
	}

	if bLine, ok := cache[commitHash]; ok {
		bLine = bLine.DeepCopyBaseline()
		return bLine, nil
	}

	// We did not find it in the cache so lets load it from GCS.
	ret, err := b.gStorageClient.ReadBaseline(commitHash, 0)
	if err != nil {
		return nil, err
	}

	// Look up the commit to see if it's valid.
	if ret == nil {
		// Load the commit and determine if it's on the current branch.
		details, err := b.vcs.Details(context.TODO(), commitHash, true)
		if err != nil {
			return nil, err
		}

		// Get the branch we are tracking and make sure that the commit is in that branch.
		branch := b.vcs.GetBranch()
		if !details.Branches[branch] {
			return nil, skerr.Fmt("Commit %s is not in branch %s", commitHash, branch)
		}

		// Make sure all expecations are up to date.
		if ret, err = b.PushMasterBaselines(nil, commitHash); err != nil {
			return nil, err
		}
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
