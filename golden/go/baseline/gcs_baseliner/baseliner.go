package gcs_baseliner

import (
	"context"
	"math"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/patrickmn/go-cache"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tryjobstore"
	"golang.org/x/sync/errgroup"
)

// TODO(stephana): Tune issueCacheSize by either finding a good value that works across instance
// or one that can be tuned.

const (
	// issueCacheSize is the size of the baselines cache for issue.
	issueCacheSize = 10000

	// baselineExpirationTime is how long the baselines should be cached for reading.
	baselineExpirationTime = time.Minute
)

// BaselinerImpl is a helper type that provides functions to write baselines (expectations) to
// GCS and retrieve them. Other packages use it to continuously write expectations to GCS
// as they become available.
type BaselinerImpl struct {
	gStorageClient    storage.GCSClient
	expectationsStore expstorage.ExpectationsStore
	tryjobStore       tryjobstore.TryjobStore
	vcs               vcsinfo.VCS

	// mutex protects lastWrittenBaselines, baselineCache and currentTile
	mutex sync.RWMutex

	// lastWrittenBaselines maps[commit_hash]MD5_sum_of_baseline to keep track whether a baseline for
	// a specific commit has been written already and whether we need to write it again (different MD5)
	lastWrittenBaselines map[string]string

	// baselineCache caches the baselines of all commits of the current tile.
	// Maps commit hash to *baseline.Baseline
	baselineCache *cache.Cache

	// currTileInfo is the latest tileInfo we have.
	currTileInfo baseline.TileInfo

	// issueBaselineCache caches baselines for issue by mapping from issueID to baseline.
	issueBaselineCache *lru.Cache
}

// New creates a new instance of baseliner.Baseliner that interacts with baselines in GCS.
func New(gStorageClient storage.GCSClient, expectationsStore expstorage.ExpectationsStore, tryjobStore tryjobstore.TryjobStore, vcs vcsinfo.VCS) (*BaselinerImpl, error) {
	c, err := lru.New(issueCacheSize)
	if err != nil {
		return nil, skerr.Fmt("Error allocating cache: %s", err)
	}

	return &BaselinerImpl{
		gStorageClient:       gStorageClient,
		expectationsStore:    expectationsStore,
		tryjobStore:          tryjobStore,
		vcs:                  vcs,
		issueBaselineCache:   c,
		lastWrittenBaselines: map[string]string{},
		baselineCache:        cache.New(baselineExpirationTime, baselineExpirationTime),
	}, nil
}

// CanWriteBaseline implements the baseline.Baseliner interface.
func (b *BaselinerImpl) CanWriteBaseline() bool {
	return (b.gStorageClient != nil) && (b.gStorageClient.Options().BaselineGSPath != "")
}

// PushMasterBaselines implements the baseline.Baseliner interface.
func (b *BaselinerImpl) PushMasterBaselines(tileInfo baseline.TileInfo, targetHash string) (*baseline.Baseline, error) {
	defer shared.NewMetricsTimer("push_master_baselines").Stop()
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if tileInfo == nil {
		tileInfo = b.currTileInfo
	}
	if tileInfo == nil {
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
	tileCommits := tileInfo.AllCommits()
	extraCommits, err := b.getCommitsSince(tileCommits[len(tileCommits)-1])
	if err != nil {
		return nil, skerr.Fmt("error getting commits since %v: %s", tileCommits[len(tileCommits)-1], err)
	}

	perCommitBaselines, err := baseline.GetBaselinesPerCommit(exps, tileInfo, extraCommits)
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

		func(commit string, bLine *baseline.Baseline) {
			egroup.Go(func() error {
				// Write the baseline to GCS.
				_, err := b.gStorageClient.WriteBaseline(bLine)
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

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Fmt("Problem writing per-commit baselines to GCS: %s", err)
	}

	// If a specific baseline was also requested we find it now
	var ret *baseline.Baseline
	if targetHash != "" {
		var ok bool
		ret, ok = perCommitBaselines[targetHash]
		if !ok {
			return nil, skerr.Fmt("Unable to find requested commit %s", targetHash)
		}
	}

	// Swap out the baseline cache and the list of last written files.
	for c, bl := range perCommitBaselines {
		b.baselineCache.Set(c, bl, cache.DefaultExpiration)
	}
	b.currTileInfo = tileInfo
	b.lastWrittenBaselines = written
	return ret, nil
}

// PushIssueBaseline implements the baseline.Baseliner interface.
func (b *BaselinerImpl) PushIssueBaseline(issueID int64, tileInfo baseline.TileInfo, dCounter digest_counter.DigestCounter) error {
	issueExpStore := b.expectationsStore.ForIssue(issueID)
	exp, err := issueExpStore.Get()
	if err != nil {
		return skerr.Fmt("Unable to get issue expectations: %s", err)
	}

	tryjobs, tryjobResults, err := b.tryjobStore.GetTryjobs(issueID, nil, true, true)
	if err != nil {
		return skerr.Fmt("Unable to get TryjobResults")
	}

	base, err := baseline.GetBaselineForIssue(issueID, tryjobs, tryjobResults, exp, tileInfo.AllCommits())
	if err != nil {
		return skerr.Fmt("Error calculating issue baseline: %s", err)
	}

	// Add it to the cache.
	_ = b.issueBaselineCache.Add(issueID, base)

	if !b.CanWriteBaseline() {
		return skerr.Fmt("Trying to write baseline while GCS path is not configured.")
	}

	// Write the baseline to GCS.
	outputPath, err := b.gStorageClient.WriteBaseline(base)
	if err != nil {
		return skerr.Fmt("Error writing baseline to GCS: %s", err)
	}
	sklog.Infof("Baseline for issue %d written to %s.", issueID, outputPath)
	return nil
}

// FetchBaseline implements the baseline.Baseliner interface.
func (b *BaselinerImpl) FetchBaseline(commitHash string, issueID int64, issueOnly bool) (*baseline.Baseline, error) {
	isIssue := issueID > baseline.MasterBranch

	var masterBaseline *baseline.Baseline
	var issueBaseline *baseline.Baseline
	var egroup errgroup.Group

	// Retrieve the baseline on master.
	egroup.Go(func() error {
		var err error
		masterBaseline, err = b.getMasterExpectations(commitHash)
		if err != nil {
			return skerr.Fmt("Could not get master baseline: %s", err)
		}
		return nil
	})

	if isIssue {
		egroup.Go(func() error {
			val, ok := b.issueBaselineCache.Get(issueID)
			if ok {
				issueBaseline = val.(*baseline.Baseline)
				return nil
			}

			var err error
			issueBaseline, err = b.gStorageClient.ReadBaseline("", issueID)
			if err != nil {
				return skerr.Fmt("Could not get baseline for issue %d: %s", issueID, err)
			}

			// If no baseline was found. We place an empty one.
			if issueBaseline == nil {
				issueBaseline = baseline.EmptyBaseline(nil, nil)
			}

			return nil
		})
	}

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Fmt("Could not fetch baselines: %s", err)
	}

	if isIssue {
		if issueOnly {
			// Only return the portion of the baseline that would be contributed by the issue
			issueBaseline.Issue = issueID
			masterBaseline = issueBaseline
		} else {
			// Clone the retrieved baseline before we inject the issue information.
			masterBaseline = masterBaseline.Copy()
			masterBaseline.Expectations.MergeExpectations(issueBaseline.Expectations)
			masterBaseline.Issue = issueID
		}
	}
	return masterBaseline, nil
}

// getCommitSince returns all the commits have been added to the repo since the given commit.
// The returned instances of tiling.Commit do not contain a valid Author field.
func (b *BaselinerImpl) getCommitsSince(firstCommit *tiling.Commit) ([]*tiling.Commit, error) {
	defer shared.NewMetricsTimer("baseliner_get_commits_since").Stop()

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

func (b *BaselinerImpl) getMasterExpectations(commitHash string) (*baseline.Baseline, error) {
	rv := func() *baseline.Baseline {
		b.mutex.RLock()
		defer b.mutex.RUnlock()
		tileInfo := b.currTileInfo

		// If no commit hash was given use current HEAD.
		if commitHash == "" {
			// If we have no tile yet, we cannot get the HEAD of it.
			if tileInfo == nil {
				return baseline.EmptyBaseline(nil, nil)
			}
			// Get the last commit that has data.
			allCommits := tileInfo.AllCommits()
			commitHash = allCommits[len(allCommits)-1].Hash
		}

		if base, ok := b.baselineCache.Get(commitHash); ok {
			return base.(*baseline.Baseline).Copy()
		}
		return nil
	}()
	if rv != nil {
		return rv, nil
	}

	// We did not find it in the cache so lets load it from GCS.
	ret, err := b.gStorageClient.ReadBaseline(commitHash, baseline.MasterBranch)
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
	// Since we fetched from GCS - go ahead and store to cache.
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baselineCache.Set(commitHash, ret, cache.DefaultExpiration)
	return ret, nil
}

// Make sure BaselinerImpl fulfills the Baseliner interface
var _ baseline.Baseliner = (*BaselinerImpl)(nil)
