package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	ROLL_STRATEGY_AFDO         = "afdo"
	ROLL_STRATEGY_BATCH        = "batch"
	ROLL_STRATEGY_FUCHSIA_SDK  = "fuchsiaSDK"
	ROLL_STRATEGY_LKGR         = "lkgr"
	ROLL_STRATEGY_REMOTE_BATCH = "remote batch"
	ROLL_STRATEGY_SINGLE       = "single"
)

// NextRollStrategy is an interface for modules which determine what the next roll
// revision should be.
type NextRollStrategy interface {
	// Return the next roll revision, or an error. Parameters are the child
	// git checkout and the last roll revision.
	GetNextRollRev(context.Context, *git.Checkout, string) (string, error)
}

// Return the NextRollStrategy indicated by the given string.
func GetNextRollStrategy(strategy string, branch, lkgr string) (NextRollStrategy, error) {
	switch strategy {
	case ROLL_STRATEGY_AFDO:
		return nil, nil // Handled by ChromiumAFDORepoManager.
	case ROLL_STRATEGY_BATCH:
		return StrategyHead(branch), nil
	case ROLL_STRATEGY_FUCHSIA_SDK:
		return nil, nil // Handled by FuchsiaSDKRepoManager.
	case ROLL_STRATEGY_LKGR:
		return StrategyLKGR(lkgr), nil
	case ROLL_STRATEGY_REMOTE_BATCH:
		return StrategyRemoteHead(branch), nil
	case ROLL_STRATEGY_SINGLE:
		return StrategySingle(branch), nil
	default:
		return nil, fmt.Errorf("Unknown roll strategy %q", strategy)
	}
}

// headStrategy is a NextRollStrategy which always rolls to HEAD of a given branch.
type headStrategy struct {
	branch string
}

// See documentation for NextRollStrategy interface.
func (s *headStrategy) GetNextRollRev(ctx context.Context, repo *git.Checkout, _ string) (string, error) {
	return repo.FullHash(ctx, fmt.Sprintf("origin/%s", s.branch))
}

// StrategyHead returns a NextRollStrategy which always rolls to HEAD of a given branch.
func StrategyHead(branch string) NextRollStrategy {
	return &headStrategy{
		branch: branch,
	}
}

// remoteHeadStrategy is a NextRollStrategy which always rolls to HEAD of a
// given branch, as defined by "git ls-remote".
type remoteHeadStrategy struct {
	branch string
}

// See documentation for NextRollStrategy interface.
func (s *remoteHeadStrategy) GetNextRollRev(ctx context.Context, repo *git.Checkout, _ string) (string, error) {
	output, err := repo.Git(ctx, "ls-remote", UPSTREAM_REMOTE_NAME, fmt.Sprintf("refs/heads/%s", s.branch), "-1")
	if err != nil {
		return "", err
	}
	tokens := strings.Split(output, "\t")
	return tokens[0], nil
}

// StrategyRemoteHead returns a NextRollStrategy which always rolls to HEAD of a
// given branch, as defined by "git ls-remote".
func StrategyRemoteHead(branch string) NextRollStrategy {
	return &remoteHeadStrategy{
		branch: branch,
	}
}

// singleStrategy is a NextRollStrategy which rolls toward HEAD of a given branch, one
// commit at a time.
type singleStrategy struct {
	*headStrategy
}

// See documentation for NextRollStrategy interface.
func (s *singleStrategy) GetNextRollRev(ctx context.Context, repo *git.Checkout, lastRollRev string) (string, error) {
	head, err := s.headStrategy.GetNextRollRev(ctx, repo, lastRollRev)
	if err != nil {
		return "", err
	}
	commits, err := repo.RevList(ctx, fmt.Sprintf("%s..%s", lastRollRev, head))
	if err != nil {
		return "", fmt.Errorf("Failed to list revisions: %s", err)
	}
	if len(commits) == 0 {
		return lastRollRev, nil
	} else {
		return commits[len(commits)-1], nil
	}
}

// StrategySingle returns a NextRollStrategy which rolls toward HEAD of a given branch,
// one commit at a time.
func StrategySingle(branch string) NextRollStrategy {
	return &singleStrategy{StrategyHead(branch).(*headStrategy)}
}

// urlStrategy is a NextRollStrategy which rolls to a revision specified by a web
// server.
type urlStrategy struct {
	client *http.Client
	parse  func(string) (string, error)
	url    string
}

// See documentation for NextRollStrategy interface.
func (s *urlStrategy) GetNextRollRev(ctx context.Context, _ *git.Checkout, _ string) (string, error) {
	resp, err := s.client.Get(s.url)
	if err != nil {
		return "", err
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return s.parse(string(body))
}

// StrategyURL returns a NextRollStrategy which rolls to a revision specified by a web
// server.
func StrategyURL(client *http.Client, url string, parseFn func(string) (string, error)) NextRollStrategy {
	return &urlStrategy{
		client: client,
		parse:  parseFn,
		url:    url,
	}
}

// StrategyLKGR returns a NextRollStrategy which rolls to a Last Known Good Revision,
// which is obtainable from a web server.
func StrategyLKGR(url string) NextRollStrategy {
	return StrategyURL(nil, url, func(body string) (string, error) {
		return strings.TrimSpace(body), nil
	})
}

// gitilesStrategy is a NextRollStrategy which uses the Gitiles API to obtain
// the list of not-yet-rolled commits.
type gitilesStrategy struct {
	branch string
	r      *gitiles.Repo
	fn     func(string, []*vcsinfo.LongCommit) string
}

// See documentation for NextRollStrategy interface.
func (s *gitilesStrategy) GetNextRollRev(ctx context.Context, _ *git.Checkout, lastRollRev string) (string, error) {
	commits, err := s.r.Log(lastRollRev, s.branch)
	if err != nil {
		return "", err
	}
	return s.fn(lastRollRev, commits), nil
}

// StrategyGitilesBatch returns a NextRollStrategy which rolls to HEAD of a
// given branch, using the Gitiles API instead of a local checkout.
func StrategyGitilesBatch(client *http.Client, repoUrl, branch string) NextRollStrategy {
	return &gitilesStrategy{
		branch: branch,
		r:      gitiles.NewRepo(repoUrl, client),
		fn: func(lastRollRev string, newCommits []*vcsinfo.LongCommit) string {
			if len(newCommits) == 0 {
				return lastRollRev
			}
			return newCommits[0].Hash
		},
	}
}

// StrategyGitilesSingle returns a NextRollStrategy which rolls toward HEAD of a
// given branch one commit at a time, using the Gitiles API instead of a local
// checkout.
func StrategyGitilesSingle(client *http.Client, repoUrl, branch string) NextRollStrategy {
	return &gitilesStrategy{
		branch: branch,
		r:      gitiles.NewRepo(repoUrl, client),
		fn: func(lastRollRev string, newCommits []*vcsinfo.LongCommit) string {
			if len(newCommits) == 0 {
				return lastRollRev
			}
			return newCommits[len(newCommits)-1].Hash
		},
	}
}
