package watcher

/*
   This file contains code related to the initial ingestion of a git repo.
*/

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	fakeBranchPrefix = "///Fake"
	gcsIdleWait      = time.Second
	gcsRetryWait     = 3 * time.Second
)

// isFakeBranch returns true iff the given branch name is a fake one created by
// getFakeBranch.
func isFakeBranch(branch string) bool {
	return strings.HasPrefix(branch, fakeBranchPrefix)
}

// getFakeBranch returns an unused fake branch name, given the map of existing
// branch names to commit hashes.
func getFakeBranch(existingBranches util.StringSet) string {
	return fmt.Sprintf("%s%s", fakeBranchPrefix, uuid.New())
}

// initialIngestCommitBatch ingests the given batch of commits by adding them to
// the Graph, ensuring that all of them are reachable from branch heads. Because
// we're dealing with an incomplete Graph at this stage, it's possible that the
// commitBatch may contain commits which are not reachable from any branch. In
// that case, initialIngestCommitBatch will create fake branches as needed to
// ensure that all commits are reachable. The commits in the batch must be in
// topological order, ie. a commit's parents must appear before the commit.
func initialIngestCommitBatch(ctx context.Context, graph *repograph.Graph, ri *repograph.MemCacheRepoImpl, cb *commitBatch) error {
	// Get the current state of the branch heads.

	// branchSet tracks which branches are present.
	branchSet := util.NewStringSet()
	// reverseBranchMap has commit hashes as keys and branch names as
	// values. This will cause branches which point to the same commit to be
	// deduplicated, which is desirable in the case of fake branches and is
	// not a problem for real branches, because they will be fixed once the
	// initial ingestion is complete.
	reverseBranchMap := map[string]string{}
	for _, b := range graph.BranchHeads() {
		branchSet[b.Name] = true
		reverseBranchMap[b.Head] = b.Name
	}

	// Loop over the commits in the commitBatch, "walking" the branch heads
	// forward to account for the new commits.
	for _, c := range cb.commits {
		// Add the commit to the RepoImpl so that it can be found
		// during graph.Update().
		ri.Commits[c.Hash] = c

		// Figure out which branches point to this commit's parents.
		// Only keep the first real and fake branches we find; the rest
		// can be thrown away. We assume that the first parent is the
		// more important line of history.
		var realBranch string
		var fakeBranch string
		for _, p := range c.Parents {
			if name, ok := reverseBranchMap[p]; ok {
				delete(branchSet, name)
				delete(reverseBranchMap, p)
				if isFakeBranch(name) {
					if fakeBranch == "" {
						fakeBranch = name
					}
				} else {
					if realBranch == "" {
						realBranch = name
					}
				}
			}
		}
		var branch string
		if realBranch != "" {
			// If we have a real branch, use that.
			branch = realBranch
		} else if !branchSet[cb.branch] {
			// If we haven't yet used the branch on the commitBatch,
			// use that.
			branch = cb.branch
		} else if fakeBranch != "" {
			// If we have a fake branch, fall back to that.
			branch = fakeBranch
		} else {
			// No branch points to any of this commit's parents;
			// create a fake branch to point to this commit.
			branch = getFakeBranch(branchSet)
		}
		branchSet[branch] = true
		reverseBranchMap[c.Hash] = branch
	}

	// Create the new set of branch heads.
	branches := make([]*git.Branch, 0, len(reverseBranchMap))
	for head, name := range reverseBranchMap {
		branches = append(branches, &git.Branch{
			Name: name,
			Head: head,
		})
	}
	ri.BranchList = branches

	// Update the graph. This triggers a request to save the Graph to GCS.
	if err := graph.Update(ctx); err != nil {
		return skerr.Wrapf(err, "Failed to update Graph with new commits and branches.")
	}
	return nil
}

// setupInitialIngest creates a repograph.Graph and a RepoImpl to be used for
// the initial ingestion of a git repo.
func setupInitialIngest(ctx context.Context, gcsClient gcs.GCSClient, gcsPath, repoUrl string) (*repograph.Graph, *initialIngestRepoImpl, error) {
	normUrl, err := git.NormalizeURL(repoUrl)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to normalize repo URL: %s", repoUrl)
	}
	file := path.Join(gcsPath, strings.ReplaceAll(normUrl, "/", "_"))
	ri := newInitialIngestRepoImpl(ctx, gcsClient, file)
	r, err := gcsClient.FileReader(ctx, file)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			g, err := repograph.NewWithRepoImpl(ctx, ri)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "Failed to create repo graph.")
			}
			ri.graph = g
			return g, ri, nil
		} else {
			return nil, nil, skerr.Wrapf(err, "Failed to read Graph from GCS.")
		}
	}
	defer util.Close(r)
	g, err := repograph.NewFromGob(ctx, r, ri)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to create Graph from GCS.")
	}
	ri.graph = g
	return g, ri, nil
}

// initialIngestRepoImpl is a struct used during initial ingestion of a git repo.
type initialIngestRepoImpl struct {
	*repograph.MemCacheRepoImpl
	file             string
	gcs              gcs.GCSClient
	graph            *repograph.Graph
	writeRequests    int
	writeRequestsMtx sync.Mutex
}

// newInitialIngestRepoImpl returns a repograph.RepoImpl used for initial
// ingestion of a git repo.
func newInitialIngestRepoImpl(ctx context.Context, gcsClient gcs.GCSClient, file string) *initialIngestRepoImpl {
	mem := repograph.NewMemCacheRepoImpl(map[string]*vcsinfo.LongCommit{}, nil)
	ri := &initialIngestRepoImpl{
		MemCacheRepoImpl: mem,
		file:             file,
		gcs:              gcsClient,
	}
	go func() {
		for {
			ri.writeRequestsMtx.Lock()
			writeRequests := ri.writeRequests
			ri.writeRequestsMtx.Unlock()
			if writeRequests > 0 {
				if err := ri.write(ctx); err != nil {
					sklog.Errorf("Failed to write Graph to GCS: %s; will retry in %s", err, gcsRetryWait)
					time.Sleep(gcsRetryWait)
				} else {
					ri.writeRequestsMtx.Lock()
					ri.writeRequests -= writeRequests
					ri.writeRequestsMtx.Unlock()
				}
			} else {
				time.Sleep(gcsIdleWait)
			}
		}
	}()
	return ri
}

// See documentation for RepoImpl interface.
func (ri *initialIngestRepoImpl) UpdateCallback(ctx context.Context, _, _ []*vcsinfo.LongCommit, _ *repograph.Graph) (rv error) {
	ri.writeRequestsMtx.Lock()
	defer ri.writeRequestsMtx.Unlock()
	ri.writeRequests += 1
	return nil
}

// Write the Graph to the backing store.
func (ri *initialIngestRepoImpl) write(ctx context.Context) error {
	sklog.Infof("Backing up graph with %d commits.", ri.graph.Len())
	w := ri.gcs.FileWriter(ctx, ri.file, gcs.FILE_WRITE_OPTS_TEXT)
	writeErr := ri.graph.WriteGob(w)
	closeErr := w.Close()
	if writeErr != nil && closeErr != nil {
		return skerr.Wrapf(writeErr, "Failed to write Graph to GCS and failed to close GCS file with: %s", closeErr)
	} else if writeErr != nil {
		return skerr.Wrapf(writeErr, "Failed to write Graph to GCS.")
	} else if closeErr != nil {
		return skerr.Wrapf(closeErr, "Failed to close GCS file.")
	}
	return nil
}

// Wait for any push to the backing store to be finished.
func (ri *initialIngestRepoImpl) Wait() {
	for {
		ri.writeRequestsMtx.Lock()
		writeRequests := ri.writeRequests
		ri.writeRequestsMtx.Unlock()
		if writeRequests == 0 {
			return
		}
		time.Sleep(time.Second)
	}
}
