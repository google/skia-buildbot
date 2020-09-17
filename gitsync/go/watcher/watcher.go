package watcher

import (
	"context"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

const (
	// batchSize is the number of commits retrieved at a time from Gitiles.
	batchSize = 10000
)

var (
	// Don't delete these branches. For some reason, this branch is
	// occasionally missing from the branch heads we get back from Gitiles,
	// And updating the branch info and re-ingesting the commits wastes
	// time. Maps repo URL to branch name to bool, indicating that deletion
	// of this branch in this repo should be skipped.
	// See http://b/139938100 for more information.
	ignoreDeletedBranch = map[string]map[string]bool{
		common.REPO_SKIA: {
			"chrome/m65": true,
		},
	}
)

// Start creates a GitStore with the provided information and starts periodic
// ingestion.
func Start(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL string, includeBranches []string, gitilesURL, gcsBucket, gcsPath string, interval time.Duration, ts oauth2.TokenSource) error {
	sklog.Infof("Initializing watcher for %s", repoURL)
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return skerr.Wrapf(err, "Error instantiating git store for %s.", repoURL)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gr := gitiles.NewRepo(gitilesURL, client)
	s, err := storage.NewClient(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create storage client for %s.", gcsBucket)
	}
	gcsClient := gcsclient.New(s, gcsBucket)
	p, err := pubsub.NewPublisher(ctx, conf, gitStore.RepoID, ts)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create PubSub publisher for %s", repoURL)
	}
	ri, err := newRepoImpl(ctx, gitStore, gr, gcsClient, gcsPath, p, includeBranches)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create RepoImpl for %s; using gs://%s/%s.", repoURL, gcsBucket, gcsPath)
	}
	sklog.Infof("Building Graph for %s...", repoURL)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create repo graph for %s.", repoURL)
	}
	repo.UpdateBranchInfo()

	// Start periodic ingestion.
	lvGitSync := metrics2.NewLiveness("last_successful_git_sync", map[string]string{"repo": repoURL})
	cleanup.Repeat(interval, func(ctx context.Context) {
		defer metrics2.FuncTimer().Stop()
		// Catch any panic and log relevant information to find the root cause.
		defer func() {
			if err := recover(); err != nil {
				sklog.Errorf("Panic updating %s:  %s\n%s", repoURL, err, string(debug.Stack()))
			}
		}()

		sklog.Infof("Updating %s...", repoURL)
		if err := repo.Update(ctx); err != nil {
			sklog.Errorf("Error updating %s: %s", repoURL, err)
		} else {
			gotBranches, err := gitStore.GetBranches(ctx)
			if err != nil {
				sklog.Errorf("Successfully updated %s but failed to retrieve branch heads: %s", repoURL, err)
			} else {
				sklog.Infof("Successfully updated %s", repoURL)
				for name, branch := range gotBranches {
					sklog.Debugf("  %s@%s: %d, %s", path.Base(repoURL), name, branch.Index, branch.Head)
				}
			}
			lvGitSync.Reset()
		}
	}, nil)
	return nil
}

// repoImpl is an implementation of repograph.RepoImpl which loads commits into
// a GitStore.
type repoImpl struct {
	*repograph.MemCacheRepoImpl
	gcsClient       gcs.GCSClient
	gcsPath         string
	gitiles         *gitiles.Repo
	gitstore        gitstore.GitStore
	includeBranches []string
	// The Publisher may be nil, in which case no pubsub messages are sent.
	pubsub *pubsub.Publisher
}

// newRepoImpl returns a repograph.RepoImpl which uses both Gitiles and
// GitStore.  If includeBranches is non-empty, only the specified branches are
// synced.
func newRepoImpl(ctx context.Context, gs gitstore.GitStore, repo *gitiles.Repo, gcsClient gcs.GCSClient, gcsPath string, p *pubsub.Publisher, includeBranches []string) (repograph.RepoImpl, error) {
	indexCommits, err := gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading IndexCommits from GitStore.")
	}
	var commits []*vcsinfo.LongCommit
	if len(indexCommits) > 0 {
		hashes := make([]string, 0, len(indexCommits))
		for _, c := range indexCommits {
			hashes = append(hashes, c.Hash)
		}
		commits, err = gs.Get(ctx, hashes)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed loading LongCommits from GitStore.")
		}
	}
	gb, err := gs.GetBranches(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading branches from GitStore.")
	}
	branches := make([]*git.Branch, 0, len(gb))
	for name, branch := range gb {
		branches = append(branches, &git.Branch{
			Name: name,
			Head: branch.Head,
		})
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}
	sklog.Infof("Repo %s has %d commits and %d branches.", repo.URL, len(commits), len(branches))
	for _, b := range branches {
		sklog.Infof("  branch %s @ %s", b.Name, b.Head)
	}
	return &repoImpl{
		MemCacheRepoImpl: repograph.NewMemCacheRepoImpl(commitsMap, branches),
		gcsClient:        gcsClient,
		gcsPath:          gcsPath,
		gitiles:          repo,
		gitstore:         gs,
		pubsub:           p,
		includeBranches:  includeBranches,
	}, nil
}

type commitBatch struct {
	branch  string
	commits []*vcsinfo.LongCommit
}

// processCommits processes commits in a separate goroutine while the
// passed-in func loads commits from Gitiles.
func (r *repoImpl) processCommits(ctx context.Context, process func(context.Context, *commitBatch) error, loadCommits func(context.Context, chan<- *commitBatch) error) error {
	// Run GitStore ingestion in a goroutine. Create a cancelable context
	// to halt requests to Gitiles if GitStore ingestion fails.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	commitsCh := make(chan *commitBatch)
	errCh := make(chan error)
	go func() {
		var err error
		for cb := range commitsCh {
			if err != nil {
				// We've hit an error but we need to consume
				// all of the commits from the channel before
				// returning, or the passed-in func may block
				// forever.
				sklog.Warningf("Skipping %d commits due to previous error.", len(cb.commits))
				continue
			}
			// Process the commits.
			if process != nil {
				err = process(ctx, cb)
			}
			if err != nil {
				if err != gitiles.ErrStopIteration {
					sklog.Errorf("processCommits encountered error: %s", err)
				}
				// Cancel the context passed to loadCommits().
				// If it respects context.Done() as it's
				// supposed to, then it should exit early with
				// an error.
				cancel()
			}
		}
		// Signal that we're done ingesting commits.
		errCh <- err
	}()

	// Run the passed-in func.
	loadingErr := loadCommits(ctx, commitsCh)

	// Close the commits channel, wait for the goroutine to complete.
	close(commitsCh)
	processErr := <-errCh

	// The error returned from the gitstore goroutine takes precedence,
	// because we cancel the context when gitstore.Put fails, and thus
	// loadCommits() may return an error simply stating that the context was
	// canceled.
	if processErr != nil {
		if processErr == gitiles.ErrStopIteration {
			// Ignore the loadingErr in this case, since it's almost
			// certainly just "context canceled".
			return nil
		}
		if loadingErr != nil {
			return skerr.Wrapf(processErr, "GitStore ingestion failed, and commit-loading func failed with: %s", loadingErr)
		}
		return processErr
	}
	return loadingErr
}

// loadCommitsFromGitiles loads commits from Gitiles and pushes them onto the
// given channel, until we reach the optional from commit, or any other commit
// we've seen before.
func (r *repoImpl) loadCommitsFromGitiles(ctx context.Context, branch, logExpr string, commitsCh chan<- *commitBatch, opts ...gitiles.LogOption) error {
	return r.gitiles.LogFnBatch(ctx, logExpr, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		commitsCh <- &commitBatch{
			branch:  branch,
			commits: commits,
		}
		return nil
	}, opts...)
}

// getFilteredBranches obtains the updated branch heads from the repo. If
// r.includeBranches is non-empty, only those branches are returned.
func (r *repoImpl) getFilteredBranches(ctx context.Context) ([]*git.Branch, error) {
	gitilesBranches, err := r.gitiles.Branches(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Filter by includeBranches.
	numBranches := len(gitilesBranches)
	if len(r.includeBranches) > 0 {
		numBranches = len(r.includeBranches)
	}
	branches := make([]*git.Branch, 0, numBranches)
	for _, branch := range gitilesBranches {
		if len(r.includeBranches) == 0 || util.In(branch.Name, r.includeBranches) {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// initialIngestion performs the first-time ingestion of the repo.
func (r *repoImpl) initialIngestion(ctx context.Context) error {
	sklog.Warningf("Performing initial ingestion of %s.", r.gitiles.URL)
	defer metrics2.FuncTimer().Stop()

	// Create a tmpGitStore.
	sklog.Info("Retrieving graph from temporary store.")
	t := timer.New("Retrieving graph from temp store")
	graph, ri, err := setupInitialIngest(ctx, r.gcsClient, r.gcsPath, r.gitiles.URL)
	if err != nil {
		return skerr.Wrapf(err, "Failed initial ingestion of %s using GCS file %s", r.gitiles.URL, r.gcsPath)
	}
	for _, c := range graph.GetAll() {
		r.Commits[c.Hash] = c.LongCommit
	}
	// oldBranches maps branch names to commit hashes for the existing
	// branches.
	oldBranches := map[string]string{}
	for _, b := range graph.BranchHeads() {
		oldBranches[b.Name] = b.Head
	}
	t.Stop()

	// Find the current set of branches.
	t = timer.New("Loading commits from gitiles")
	branches, err := r.getFilteredBranches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve branches")
	}

	// Load commits from gitiles.

	// We assume that the main branch contains the majority of the commits in
	// the repo, and the other branches are comparatively small, with most of
	// their ancestry being on the main branch itself.
	var mainBranch *git.Branch
	for _, b := range branches {
		if b.Name == git.DefaultBranch {
			mainBranch = b
			break
		}
	}
	if mainBranch != nil {
		if mainBranch.Head == oldBranches[mainBranch.Name] {
			sklog.Infof("%q is up to date; skipping.", mainBranch.Name)
		} else {
			sklog.Info("Loading commits from gitiles for %q.", mainBranch.Name)
			if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
				// Add the new commits to our local cache.
				sklog.Infof("Adding batch of %d commits", len(cb.commits))
				for _, c := range cb.commits {
					r.Commits[c.Hash] = c
				}
				return initialIngestCommitBatch(ctx, graph, ri.MemCacheRepoImpl, cb)
			}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
				logExpr := mainBranch.Head
				if oldHead, ok := oldBranches[mainBranch.Name]; ok {
					sklog.Errorf("Have %s @ %s; requesting %s", mainBranch.Name, oldHead, mainBranch.Head)
					logExpr = git.LogFromTo(oldHead, mainBranch.Head)
				}
				return r.loadCommitsFromGitiles(ctx, mainBranch.Name, logExpr, commitsCh, gitiles.LogReverse(), gitiles.LogBatchSize(batchSize))
			}); err != nil {
				return skerr.Wrapf(err, "Failed to ingest commits for %q.", mainBranch.Name)
			}
		}
	}
	ri.Wait()

	// mtx protects graph and ri.commits as we load commits for non-main
	// branches in different goroutines.
	var mtx sync.Mutex

	// Load commits for other branches, in non-reverse order, so that we can
	// stop once we reach commits already on the main branch.
	var egroup errgroup.Group

	for _, branch := range branches {
		// https://golang.org/doc/faq#closures_and_goroutines
		branch := branch
		if branch == mainBranch {
			continue
		}
		sklog.Infof("Loading commits for %s", branch.Name)
		mtx.Lock()
		_, exists := r.Commits[branch.Head]
		mtx.Unlock()
		if exists {
			sklog.Infof(" ... already have %s, skip", branch.Head)
			continue
		}
		egroup.Go(func() error {
			var commits []*vcsinfo.LongCommit
			mtx.Lock()
			localCache := make(map[string]*vcsinfo.LongCommit, len(r.Commits))
			for h, c := range r.Commits {
				localCache[h] = c
			}
			mtx.Unlock()

			// lookingFor tracks which commits are wanted by this
			// branch. As we traverse back through git history, we
			// may find commits which we already have in our local
			// cache. One might think that we could stop requesting
			// batches of commits at that point, but if there's a
			// commit with multiple parents on this branch, we have
			// to make sure we follow all lines of history, which
			// means that we need to track all of the commit hashes
			// which we expect to find but have not yet. We can stop
			// when we've found all of the hashes we're looking for.
			lookingFor := map[string]bool{}
			if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
				numIngested := 0
				defer func() {
					sklog.Infof("Added %d of batch of %d commits", numIngested, len(cb.commits))
				}()
				for _, c := range cb.commits {
					// Remove this commit from lookingFor,
					// now that we've found it.
					delete(lookingFor, c.Hash)
					// Add the commit to the local cache,
					// if it's not already present. Track
					// the number of new commits we've seen.
					if _, ok := localCache[c.Hash]; !ok {
						commits = append(commits, c)
						localCache[c.Hash] = c
						numIngested++
					}
					// Add any parents of this commit which
					// are not already in our cache to the
					// lookingFor set.
					for _, p := range c.Parents {
						if _, ok := localCache[p]; !ok {
							lookingFor[p] = true
						}
					}
					// If we've found all the commits we
					// need, we can stop.
					if len(lookingFor) == 0 {
						return gitiles.ErrStopIteration
					}
				}
				return nil
			}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
				logExpr := branch.Head
				if oldHead, ok := oldBranches[branch.Name]; ok {
					logExpr = git.LogFromTo(oldHead, branch.Head)
				}
				return r.loadCommitsFromGitiles(ctx, branch.Name, logExpr, commitsCh, gitiles.LogBatchSize(batchSize))
			}); err != nil {
				return skerr.Wrapf(err, "Failed to ingest commits for branch %s.", branch.Name)
			}
			// Reverse the slice of commits so that they can be added in
			// order.
			for i := 0; i < len(commits)/2; i++ {
				j := len(commits) - i - 1
				commits[i], commits[j] = commits[j], commits[i]
			}
			mtx.Lock()
			defer mtx.Unlock()
			if err := initialIngestCommitBatch(ctx, graph, ri.MemCacheRepoImpl, &commitBatch{
				branch:  branch.Name,
				commits: commits,
			}); err != nil {
				return skerr.Wrapf(err, "Failed to add commits to graph for %s", branch.Name)
			}
			for _, c := range commits {
				r.Commits[c.Hash] = c
			}
			sklog.Infof("Loading commits for %s done", branch.Name)
			return nil
		})
	}
	// Wait for the above goroutines to finish.
	if err := egroup.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	// Wait for the initialIngestRepoImpl to finish backing up to GCS.
	ri.Wait()
	t.Stop()

	// Replace the fake branches with real ones. Update branch membership.
	ri.BranchList = branches
	if err := graph.Update(ctx); err != nil {
		return skerr.Wrapf(err, "Failed final Graph update.")
	}
	ri.Wait()
	r.BranchList = branches
	sklog.Infof("Finished initial ingestion of %s; have %d commits and %d branches.", r.gitiles.URL, graph.Len(), len(graph.Branches()))
	return nil
}

// addCommitsToCacheFn returns a function, intended to be passed as a
// parameter to processCommits, which adds any new commits to the local cache
// and stops iteration when all new commits have been added.
func (r *repoImpl) addCommitsToCacheFn() func(context.Context, *commitBatch) error {
	lookingFor := map[string]bool{}
	return func(_ context.Context, cb *commitBatch) error {
		// Add the new commits to our local cache.
		for _, c := range cb.commits {
			delete(lookingFor, c.Hash)
			for _, p := range c.Parents {
				if _, ok := r.Commits[p]; !ok {
					lookingFor[p] = true
				}
			}
			r.Commits[c.Hash] = c
			if len(lookingFor) == 0 {
				return gitiles.ErrStopIteration
			}
		}
		return nil
	}
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Update(ctx context.Context) error {
	sklog.Infof("repoImpl.Update for %s", r.gitiles.URL)
	defer metrics2.FuncTimer().Stop()
	if len(r.BranchList) == 0 {
		if err := r.initialIngestion(ctx); err != nil {
			return skerr.Wrapf(err, "Failed initial ingestion.")
		}
	}

	// Find the old and new branch heads.
	sklog.Infof("Getting branches for %s.", r.gitiles.URL)
	oldBranches := make(map[string]*git.Branch, len(r.BranchList))
	for _, branch := range r.BranchList {
		oldBranches[branch.Name] = branch
	}
	branches, err := r.getFilteredBranches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}
	// If any of the ignoreDeletedBranches disappeared, add it back.
	newBranches := make(map[string]string, len(branches))
	for _, branch := range branches {
		newBranches[branch.Name] = branch.Head
	}
	for name, b := range oldBranches {
		if _, ok := newBranches[name]; !ok && ignoreDeletedBranch[r.gitiles.URL][name] {
			sklog.Warningf("Branch %q missing from new branches; ignoring.", name)
			branches = append(branches, b)
		}
	}

	// Download any new commits and add them to the local cache.
	sklog.Infof("Processing new commits for %s.", r.gitiles.URL)
	for _, branch := range branches {
		logExpr := branch.Head
		if oldBranch, ok := oldBranches[branch.Name]; ok {
			// If there's nothing new, skip this branch.
			if branch.Head == oldBranch.Head {
				continue
			}
			// Only load back to the previous branch head.
			logExpr = git.LogFromTo(oldBranch.Head, branch.Head)
		}
		if err := r.processCommits(ctx, r.addCommitsToCacheFn(), func(ctx context.Context, commitsCh chan<- *commitBatch) error {
			err := r.loadCommitsFromGitiles(ctx, branch.Name, logExpr, commitsCh)
			if err != nil && strings.Contains(err.Error(), "404 Not Found") {
				// If history was changed, the old branch head
				// may not be present on the server. Try again
				// as if the branch is new.
				sklog.Errorf("Failed loading commits for %s (%q); trying %s: %s", branch.Name, logExpr, branch.Head, err)
				err = r.loadCommitsFromGitiles(ctx, branch.Name, branch.Head, commitsCh)
			}
			if err != nil {
				return skerr.Wrapf(err, "Failed loading commits for %s (%s); %q", branch.Name, branch.Head, logExpr)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	r.BranchList = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	c, err := r.MemCacheRepoImpl.Details(ctx, hash)
	if err == nil {
		return c, nil
	}
	// Fall back to retrieving from Gitiles, store any new commits in the
	// local cache.
	sklog.Errorf("Missing commit %s in %s", hash[:7], r.gitiles.URL)
	if err := r.processCommits(ctx, r.addCommitsToCacheFn(), func(ctx context.Context, commitsCh chan<- *commitBatch) error {
		return r.loadCommitsFromGitiles(ctx, "", hash, commitsCh)
	}); err != nil {
		return nil, err
	}
	c, ok := r.Commits[hash]
	if !ok {
		return nil, skerr.Fmt("Commit %s in %s is still missing despite attempting to load it from gitiles.", hash, r.gitiles.URL)
	}
	return c, nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) UpdateCallback(ctx context.Context, added, removed []*vcsinfo.LongCommit, graph *repograph.Graph) error {
	sklog.Infof("repoImpl.UpdateCallback for %s", r.gitiles.URL)
	defer metrics2.FuncTimer().Stop()
	// Ensure that branch membership is up to date.
	modified := graph.UpdateBranchInfo()
	modifiedMap := make(map[string]*vcsinfo.LongCommit, len(modified)+len(added))
	for _, c := range added {
		modifiedMap[c.Hash] = c
	}
	for _, c := range modified {
		modifiedMap[c.Hash] = c
	}
	// Don't include commits in the 'removed' list.
	for _, c := range removed {
		delete(modifiedMap, c.Hash)
	}
	putCommits := make([]*vcsinfo.LongCommit, 0, len(modifiedMap))
	for _, c := range modifiedMap {
		putCommits = append(putCommits, c)
	}
	// TODO(borenet): Should we delete commits which were removed?
	sklog.Infof("Put %d new and %d modified commits for %s.", len(added), len(modified), r.gitiles.URL)
	if err := r.gitstore.Put(ctx, putCommits); err != nil {
		return skerr.Wrapf(err, "Failed putting commits into GitStore.")
	}
	// Figure out which branches changed.
	oldBranches, err := r.gitstore.GetBranches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve old branch heads.")
	}
	branchHeads := graph.BranchHeads()
	allBranches := make(map[string]string, len(branchHeads))
	updateBranches := make(map[string]string, len(branchHeads))
	for _, b := range branchHeads {
		allBranches[b.Name] = b.Head
		if old, ok := oldBranches[b.Name]; !ok || old.Head != b.Head {
			updateBranches[b.Name] = b.Head
		}
	}
	// Explicitly delete any old branches which are no longer present.
	for name := range oldBranches {
		if _, ok := allBranches[name]; !ok {
			updateBranches[name] = gitstore.DELETE_BRANCH
		}
	}
	if err := r.gitstore.PutBranches(ctx, updateBranches); err != nil {
		return skerr.Wrapf(err, "Failed to put new branch heads.")
	}
	if r.pubsub != nil && len(updateBranches) > 0 {
		r.pubsub.Publish(ctx, updateBranches)
	}
	return nil
}
