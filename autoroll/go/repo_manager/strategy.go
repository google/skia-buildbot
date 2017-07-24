package repo_manager

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/util"
)

const (
	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_LKGR   = "lkgr"
	ROLL_STRATEGY_SINGLE = "single"
)

// NextRollStrategy is an interface for modules which determine what the next roll
// revision should be.
type NextRollStrategy interface {
	// Return the next roll revision, or an error. The string parameter is
	// the last-rolled revision, which may be used by the NextRollStrategy to make
	// its decision.
	GetNextRollRev(string) (string, error)
}

// Return the NextRollStrategy indicated by the given string.
func GetNextRollStrategy(strategy string, repo *git.Checkout, branch, lkgr string) (NextRollStrategy, error) {
	switch strategy {
	case ROLL_STRATEGY_BATCH:
		return StrategyHead(repo, branch), nil
	case ROLL_STRATEGY_SINGLE:
		return StrategySingle(repo, branch), nil
	case ROLL_STRATEGY_LKGR:
		return StrategyLKGR(lkgr), nil
	default:
		return nil, fmt.Errorf("Unknown roll strategy %q", strategy)
	}
}

// headStrategy is a NextRollStrategy which always rolls to HEAD of a given branch.
type headStrategy struct {
	branch string
	repo   *git.Checkout
}

// See documentation for NextRollStrategy interface.
func (s *headStrategy) GetNextRollRev(string) (string, error) {
	head, err := s.repo.FullHash(fmt.Sprintf("origin/%s", s.branch))
	if err != nil {
		return "", err
	}
	return head, nil
}

// StrategyHead returns a NextRollStrategy which always rolls to HEAD of a given branch.
func StrategyHead(repo *git.Checkout, branch string) NextRollStrategy {
	return &headStrategy{
		branch: branch,
		repo:   repo,
	}
}

// singleStrategy is a NextRollStrategy which rolls toward HEAD of a given branch, one
// commit at a time.
type singleStrategy struct {
	*headStrategy
}

// See documentation for NextRollStrategy interface.
func (s *singleStrategy) GetNextRollRev(lastRollRev string) (string, error) {
	head, err := s.headStrategy.GetNextRollRev(lastRollRev)
	if err != nil {
		return "", err
	}
	commits, err := s.repo.RevList(fmt.Sprintf("%s..%s", lastRollRev, head))
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
func StrategySingle(repo *git.Checkout, branch string) NextRollStrategy {
	return &singleStrategy{StrategyHead(repo, branch).(*headStrategy)}
}

// urlStrategy is a NextRollStrategy which rolls to a revision specified by a web
// server.
type urlStrategy struct {
	parse func(string) (string, error)
	url   string
}

// See documentation for NextRollStrategy interface.
func (s *urlStrategy) GetNextRollRev(string) (string, error) {
	resp, err := http.Get(s.url)
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
func StrategyURL(url string, parseFn func(string) (string, error)) NextRollStrategy {
	return &urlStrategy{
		parse: parseFn,
		url:   url,
	}
}

// StrategyLKGR returns a NextRollStrategy which rolls to a Last Known Good Revision,
// which is obtainable from a web server.
func StrategyLKGR(url string) NextRollStrategy {
	return StrategyURL(url, func(body string) (string, error) {
		return strings.TrimSpace(body), nil
	})
}
