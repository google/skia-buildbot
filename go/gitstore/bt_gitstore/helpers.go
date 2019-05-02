package bt_gitstore

// This file has some helper functions for working with BigTable.

import (
	"context"
	"strconv"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
)

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{
		cfCommit,
		cfMeta,
		cfBranches,
		cfTsCommit,
	})
}

// AllRepos returns a map of all repos contained in given BigTable project/instance/table.
// It returns map[normalized_URL]RepoInfo.
func AllRepos(ctx context.Context, conf *BTConfig) (map[string]*gitstore.RepoInfo, error) {
	// Create the client.
	client, err := bigtable.NewClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	table := client.Open(conf.TableID)
	rowNamesPrefix := getRepoInfoRowNamePrefix()
	ret := map[string]*gitstore.RepoInfo{}
	var readRowErr error = nil
	err = table.ReadRows(ctx, bigtable.PrefixRange(rowNamesPrefix), func(row bigtable.Row) bool {
		if readRowErr != nil {
			return false
		}

		var repoInfo *gitstore.RepoInfo
		repoInfo, readRowErr = extractRepoInfo(row)
		if readRowErr != nil {
			return false
		}
		// save the repo info and set the all-commits branch.
		ret[repoInfo.RepoURL] = repoInfo
		if found, ok := repoInfo.Branches[allCommitsBranch]; ok {
			repoInfo.Branches[""] = found
			delete(repoInfo.Branches, allCommitsBranch)
		}

		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1)))

	if err != nil {
		return nil, skerr.Fmt("Error reading repo info: %s", err)
	}
	return ret, nil
}

// RepoURLFromID retrieves the URL of a repository by its corresponding numeric ID.
// If a repository with the given ID can be found it will be returned and the second return value
// is true. In any other case "" and false will be returned.
func RepoURLFromID(ctx context.Context, conf *BTConfig, repoIDStr string) (string, bool) {
	id, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		return "", false
	}

	repoInfos, err := AllRepos(ctx, conf)
	if err != nil {
		return "", false
	}

	for repoURL, info := range repoInfos {
		if info.ID == id {
			return repoURL, false
		}
	}
	return "", false
}
