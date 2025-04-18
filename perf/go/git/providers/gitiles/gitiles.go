// ckage gitiles imlements provider.Provider using the Gitiles API.
package gitiles

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
)

const (
	batchSize = 100
)

// Gitiles implements provider.Provider.
type Gitiles struct {
	gr gitiles.GitilesRepo

	// startCommit is the commit in the repo where we start tracking commits. If
	// not supplied then we start with the first commit in the repo as reachable
	// from HEAD.
	startCommit string

	branch string
}

// New returns a new instance of Gitiles.
func New(c *http.Client, instanceConfig *config.InstanceConfig) *Gitiles {
	return &Gitiles{
		gr:          gitiles.NewRepo(instanceConfig.GitRepoConfig.URL, c),
		startCommit: instanceConfig.GitRepoConfig.StartCommit,
		branch:      instanceConfig.GitRepoConfig.Branch,
	}
}

// CommitsFromMostRecentGitHashToHead implements provider.Provider.
func (g *Gitiles) CommitsFromMostRecentGitHashToHead(ctx context.Context, mostRecentGitHash string, cb provider.CommitProcessor) error {
	if mostRecentGitHash == "" {
		mostRecentGitHash = g.startCommit
	}

	var expr string
	opts := []gitiles.LogOption{
		gitiles.LogBatchSize(batchSize),
		gitiles.LogReverse(),
	}
	if g.isMainBranch() {
		expr = git.LogFromTo(mostRecentGitHash, "HEAD")
		if mostRecentGitHash == "" {
			expr = git.MainBranch
		}
	} else {
		sklog.Infof("Ingesting from branch %s", g.branch)
		expr = git.FullyQualifiedBranchName(g.branch)
		opts = append(opts, gitiles.LogStartCommit(mostRecentGitHash))
	}

	sklog.Infof("Populating from gitiles from %q", expr)
	err := g.gr.LogFnBatch(ctx, expr, func(ctx context.Context, lcs []*vcsinfo.LongCommit) error {
		sklog.Infof("Processing %s commits: ", len(lcs))
		for _, longCommit := range lcs {
			c := provider.Commit{
				GitHash:   longCommit.Hash,
				Timestamp: longCommit.Timestamp.Unix(),
				Author:    longCommit.Author,
				Subject:   longCommit.Subject,
				Body:      longCommit.Body,
			}
			err := cb(c)
			if err != nil {
				return skerr.Wrapf(err, "processing callback")
			}
		}
		return nil
	}, opts...)

	if err != nil {
		return skerr.Wrapf(err, "loading commits")
	}
	return nil
}

// GitHashesInRangeForFile implements provider.Provider.
func (g *Gitiles) GitHashesInRangeForFile(ctx context.Context, begin, end, filename string) ([]string, error) {

	lc, err := g.gr.Log(ctx, git.LogFromTo(begin, end), gitiles.LogPath(filename), gitiles.LogReverse())
	if err != nil {
		return nil, skerr.Wrapf(err, "loading commits")
	}
	ret := make([]string, len(lc))
	for i, c := range lc {
		ret[i] = c.Hash
	}
	return ret, nil
}

// LogEntry implements provider.Provider.
func (g *Gitiles) LogEntry(ctx context.Context, gitHash string) (string, error) {
	lc, err := g.gr.Log(ctx, gitHash, gitiles.LogLimit(1))
	if err != nil {
		return "", skerr.Wrapf(err, "loading log entry")
	}
	if len(lc) != 1 {
		return "", skerr.Fmt("received %d log entries when expecting 1", len(lc))
	}
	commit := lc[0]
	sklog.Debugf("[gitiles] Fetch logEntry %s by githash: %s ", commit, gitHash)
	return fmt.Sprintf(`commit %s
Parent %s
Subject %s
Author %s
Date %s
Body %s`, commit.Hash, commit.Parents[0], commit.Subject, commit.Author,
		commit.Timestamp.Format(time.RFC822Z), commit.Body), nil
}

// Update implements provider.Provider.
func (g *Gitiles) Update(ctx context.Context) error {
	return nil
}

func (g *Gitiles) isMainBranch() bool {
	return g.branch == "" || g.branch == "main"
}

// Confirm *Gitiles implements provider.Provider.
var _ provider.Provider = (*Gitiles)(nil)
