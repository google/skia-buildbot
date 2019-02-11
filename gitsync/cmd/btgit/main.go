package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/litevcs"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define the flags and parse them.
	btInstanceID := flag.String("bt-instance", "production", "Big Table instance")
	btTableID := flag.String("bt-table", "git-repos", "BigTable table ID")
	projectID := flag.String("project", "skia-public", "ID of the GCP project")
	branch := flag.String("branch", "", "Name of the branch to list. Empty means all commits across all branches.")
	limit := flag.Int("limit", 100, "Number of commits to show. 0 means no limit")
	repoURL := flag.String("repo-url", "", "URL of the git repo.")
	common.Init()

	// Configure the bigtable instance.
	config := &litevcs.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstanceID,
		TableID:    *btTableID,
		Shards:     32,
	}

	normURL, err := litevcs.NormalizeURL(*repoURL)
	if err != nil {
		sklog.Fatalf("Error getting normalized URL for %s:  %s", *repoURL, err)
	}

	ctx := context.Background()
	allRepoInfos, err := litevcs.AllRepos(ctx, config)
	if err != nil {
		sklog.Fatalf("Error retrieving lists of repositories: %s", err)
	}

	// Make sure our repoURL exists.
	repoInfo, ok := allRepoInfos[normURL]
	if !ok {
		sklog.Fatalf("Repo %s could not found in BigTable", normURL)
	}

	// Make sure the target branch exists
	foundBranch, ok := repoInfo.Branches[*branch]
	if !ok {
		sklog.Fatalf("Error, branch %q does not exist in BigTable git", *branch)
	}

	gitStore, err := litevcs.NewBTGitStore(config, *repoURL)
	if err != nil {
		sklog.Fatalf("Error instantiating git store: %s", err)
	}

	startIndex := 0
	branchLength := foundBranch.Index + 1
	if *limit > 0 {
		startIndex = branchLength - *limit
	}

	indexCommits, err := gitStore.RangeN(ctx, startIndex, branchLength, *branch)
	if err != nil {
		sklog.Fatalf("Error retrieving branch %q: %s", *branch, err)
	}

	hashes := make([]string, 0, len(indexCommits))
	for _, commit := range indexCommits {
		hashes = append(hashes, commit.Hash)
	}

	longCommits, err := gitStore.Get(ctx, hashes)
	if err != nil {
		sklog.Fatalf("Error retrieving long commits: %s", err)
	}

	for idx := len(longCommits) - 1; idx >= 0; idx-- {
		c := longCommits[idx]
		if c == nil {
			sklog.Fatalf("Programming error: Unable to retrieve long commit for hash %s", hashes[idx])
		}
		// sklog.Infof("%s %40s %v %s", c.Hash, c.Author, c.Timestamp, c.Subject)
	}
}
