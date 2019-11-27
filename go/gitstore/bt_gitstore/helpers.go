package bt_gitstore

// This file has some helper functions for working with BigTable.

import (
	"context"
	"strconv"

	"cloud.google.com/go/bigtable"
	"github.com/google/uuid"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
)

// BTTestConfig returns a BTConfig which can be used for testing.
func BTTestConfig() *BTConfig {
	return &BTConfig{
		ProjectID:  uuid.New().String(),
		InstanceID: uuid.New().String(),
		TableID:    "test-git-repos",
		AppProfile: "testing",
	}
}

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
	tags := map[string]string{
		"project":  conf.ProjectID,
		"instance": conf.InstanceID,
		"table":    conf.TableID,
		"repo":     "all",
	}
	metrics2.GetCounter(METRIC_BT_REQS_READ, tags).Inc(1)
	rowCounter := metrics2.GetCounter(METRIC_BT_ROWS_READ, tags)
	err = table.ReadRows(ctx, bigtable.PrefixRange(rowNamesPrefix), func(row bigtable.Row) bool {
		rowCounter.Inc(1)
		if readRowErr != nil {
			return false
		}

		var repoInfo *gitstore.RepoInfo
		repoInfo, readRowErr = extractRepoInfo(row)
		if readRowErr != nil {
			return false
		}
		// save the repo info.
		ret[repoInfo.RepoURL] = repoInfo
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

// NewGitStoreMap returns a Map instance with Graphs for the given GitStores.
func NewBTGitStoreMap(ctx context.Context, repoUrls []string, btConf *BTConfig) (repograph.Map, error) {
	rv := make(map[string]*repograph.Graph, len(repoUrls))
	for _, repoUrl := range repoUrls {
		gs, err := New(ctx, btConf, repoUrl)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create GitStore for %s", repoUrl)
		}
		graph, err := gitstore.GetRepoGraph(ctx, gs)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create Graph from GitStore for %s", repoUrl)
		}
		rv[repoUrl] = graph
	}
	return rv, nil
}
