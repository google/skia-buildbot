package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
)

// btgit is a script that queries a BigTable based GitStore.

func main() {
	// Define the flags and parse them.
	var (
		btInstanceID = flag.String("bt_instance", "production", "Big Table instance")
		btTableID    = flag.String("bt_table", "git-repos", "BigTable table ID")
		loadGraph    = flag.Bool("load_graph", false, "Load the entire commit graph. For performance check only.")
		projectID    = flag.String("project", "skia-public", "ID of the GCP project")
		branch       = flag.String("branch", "", "Name of the branch to list. Empty means all commits across all branches.")
		limit        = flag.Int("limit", 100, "Number of commits to show. 0 means no limit")
		repoURL      = flag.String("repo_url", "", "URL of the git repo.")
		verbose      = flag.Bool("verbose", false, "Indicate whether to log the commits we find.")
	)
	common.Init()

	// Configure the bigtable instance.
	config := &gitstore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstanceID,
		TableID:    *btTableID,
	}

	// Normalize the URL as GitStore does.
	normURL, err := gitstore.NormalizeURL(*repoURL)
	if err != nil {
		sklog.Fatalf("Error getting normalized URL for %s:  %s", *repoURL, err)
	}

	ctx := context.Background()

	// Get all repos and find the one we want plus the branch we want.
	allRepoInfos, err := gitstore.AllRepos(ctx, config)
	if err != nil {
		sklog.Fatalf("Error retrieving lists of repositories: %s", err)
	}
	sklog.Infof("Got all repo info: %d", len(allRepoInfos))

	// Make sure our repoURL exists.
	repoInfo, ok := allRepoInfos[normURL]
	if !ok {
		sklog.Fatalf("Repo %s could not found in BigTable", normURL)
	}
	sklog.Infof("Found repo for %s", repoInfo.RepoURL)

	// Make sure the target branch exists
	foundBranch, ok := repoInfo.Branches[*branch]
	if !ok {
		sklog.Fatalf("Error, branch %q does not exist in BigTable git", *branch)
	}
	sklog.Infof("Found branch %q in repo for %s", *branch, repoInfo.RepoURL)

	// Create a new BT based GitStore.
	gitStore, err := gitstore.NewBTGitStore(ctx, config, *repoURL)
	if err != nil {
		sklog.Fatalf("Error instantiating git store: %s", err)
	}
	sklog.Infof("Opened gitstore")

	// Determine how many commits we fetch.
	startIndex := 0
	branchLength := foundBranch.Index + 1
	if *limit > 0 {
		startIndex = branchLength - *limit
	}

	// Fetch the graph of the repository to see if it performs well enough.
	if *loadGraph {
		ggt := timer.New("Getting graph")
		commitGraph, err := gitStore.GetGraph(ctx)
		if err != nil {
			sklog.Fatalf("Error retrieving graph: %s", err)
		}
		ggt.Stop()
		sklog.Infof("Loaded graph with %d nodes", len(commitGraph.Nodes))
	}

	// Retrieve the index commits we are interested in.
	indexCommits, err := gitStore.RangeN(ctx, startIndex, branchLength, *branch)
	if err != nil {
		sklog.Fatalf("Error retrieving branch %q: %s", *branch, err)
	}

	// Isolate the hashes and retrieve the LongCommits.
	hashes := make([]string, 0, len(indexCommits))
	for _, commit := range indexCommits {
		hashes = append(hashes, commit.Hash)
	}

	tlc := timer.New("Long commits")
	longCommits, err := gitStore.Get(ctx, hashes)
	if err != nil {
		sklog.Fatalf("Error retrieving long commits: %s", err)
	}
	tlc.Stop()
	sklog.Infof("Long commits loaded: %d", len(longCommits))

	for idx := len(longCommits) - 1; idx >= 0; idx-- {
		c := longCommits[idx]
		if c == nil {
			sklog.Fatalf("Programming error: Unable to retrieve long commit for hash %s", hashes[idx])
		}
		if *verbose {
			sklog.Infof("%s %40s %v %s", c.Hash, c.Author, c.Timestamp, c.Subject)
		}
	}
}
