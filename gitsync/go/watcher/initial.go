package watcher

/*
   This file contains code related to the initial ingestion of a git repo.
*/

import (
	"context"
	"fmt"
	"path"
	"strings"

	"cloud.google.com/go/storage"
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
)

// initialIngestCommitBatch ingests the given batch of commits by adding them to
// the Graph, ensuring that there are fake branches sufficient that all of them
// are reachable from branch heads.
func initialIngestCommitBatch(ctx context.Context, graph *repograph.Graph, ri *initialIngestRepoImpl, cb *commitBatch) error {
	// "Walk" the fake branch heads forward to account for the new
	// commits.
	branchMap := map[string]string{}
	reverseBranchMap := map[string]string{}
	for _, b := range graph.BranchHeads() {
		branchMap[b.Name] = b.Head
		reverseBranchMap[b.Head] = b.Name
	}
	for _, c := range cb.commits {
		// Add the commit to the RepoImpl so that it can be found
		// during graph.Update().
		ri.Commits[c.Hash] = c

		// If this commit's parent(s) are pointed to by branch
		// heads, update the branch head(s) to point to this
		// commit.
		// Find out which branches should point to this commit.
		branches := []string{}
		for _, p := range c.Parents {
			if name, ok := reverseBranchMap[p]; ok {
				branches = append(branches, name)
			}
		}
		// Determine if any of the commits pointing to this branch are
		// real branches, as opposed to fake ones we created.
		hasRealBranch := false
		for _, branch := range branches {
			if !strings.HasPrefix(branch, fakeBranchPrefix) {
				hasRealBranch = true
			}
		}
		// Deduplicate the branches. Remove fake branches until there
		// are either only real branches left, or a single fake branch.
		// Update the remaining branches to point to the current commit.
		for _, branch := range branches {
			if strings.HasPrefix(branch, fakeBranchPrefix) {
				delete(reverseBranchMap, branchMap[branch])
				delete(branchMap, branch)
				if _, ok := reverseBranchMap[c.Hash]; !ok && !hasRealBranch {
					branchMap[branch] = c.Hash
					reverseBranchMap[c.Hash] = branch
				}
			} else {
				delete(reverseBranchMap, branchMap[branch])
				reverseBranchMap[c.Hash] = branch
				branchMap[branch] = c.Hash
			}
		}
		// If no branch points to this commit, we need to add one.
		if _, ok := reverseBranchMap[c.Hash]; !ok {
			// If we haven't already used the one listed in the
			// commitBatch, use that, otherwise create a fake one.
			var branch string
			if _, ok := branchMap[cb.branch]; !ok {
				//sklog.Errorf("new branch %s @ %s", cb.branch, c.Hash)
				//sklog.Errorf("  parents: %s", strings.Join(c.Parents, ", "))
				branch = cb.branch
			} else {
				// Find an unused fake branch name.
				fakeBranchId := 0
				for {
					branch = fmt.Sprintf("%s%d", fakeBranchPrefix, fakeBranchId)
					fakeBranchId++
					if _, ok := branchMap[branch]; !ok {
						break
					}
				}
			}
			branchMap[branch] = c.Hash
			reverseBranchMap[c.Hash] = branch
		}
	}
	branches := make([]*git.Branch, 0, len(branchMap))
	for name, head := range branchMap {
		branches = append(branches, &git.Branch{
			Name: name,
			Head: head,
		})
	}
	ri.BranchList = branches

	// Update the graph. This also saves the Graph to GCS.
	if err := graph.Update(ctx); err != nil {
		sklog.Errorf("Failed to update graph. Current Branches:")
		for _, b := range graph.BranchHeads() {
			sklog.Errorf("%s @ %s", b.Name, b.Head)
		}
		sklog.Errorf("New branches:")
		for _, b := range branches {
			sklog.Errorf("%s @ %s", b.Name, b.Head)
		}
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
	return g, ri, nil
}

// initialIngestRepoImpl is a struct used during initial ingestion of a git repo.
type initialIngestRepoImpl struct {
	*repograph.MemCacheRepoImpl
	file string
	gcs  gcs.GCSClient
}

// newInitialIngestRepoImpl returns a repograph.RepoImpl used for initial
// ingestion of a git repo.
func newInitialIngestRepoImpl(ctx context.Context, gcsClient gcs.GCSClient, file string) *initialIngestRepoImpl {
	mem := repograph.NewMemCacheRepoImpl(map[string]*vcsinfo.LongCommit{}, nil).(*repograph.MemCacheRepoImpl)
	return &initialIngestRepoImpl{
		MemCacheRepoImpl: mem,
		file:             file,
		gcs:              gcsClient,
	}
}

// See documentation for RepoImpl interface.
func (ri *initialIngestRepoImpl) UpdateCallback(ctx context.Context, _, _ []*vcsinfo.LongCommit, g *repograph.Graph) (rv error) {
	w := ri.gcs.FileWriter(ctx, ri.file, gcs.FILE_WRITE_OPTS_TEXT)
	defer func() {
		err := w.Close()
		if err != nil {
			if rv != nil {
				rv = skerr.Wrapf(rv, "And failed to close: %s", err)
			} else {
				rv = skerr.Wrap(err)
			}
		}
	}()
	return g.WriteGob(w)
}

// Delete the backing file.
func (ri *initialIngestRepoImpl) Delete(ctx context.Context) error {
	return ri.gcs.DeleteFile(ctx, ri.file)
}
