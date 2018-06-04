package strategy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/api/option"
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
	// Return the next roll revision, given the list of not-yet-rolled
	// commits in reverse chronological order. Returning the empty string
	// implies that we are up-to-date.
	GetNextRollRev(context.Context, []*vcsinfo.LongCommit) (string, error)
}

// Return the NextRollStrategy indicated by the given string.
func GetNextRollStrategy(ctx context.Context, strategy, branch, lkgr, upstreamRemote string, repo *git.Checkout, authClient *http.Client) (NextRollStrategy, error) {
	switch strategy {
	case ROLL_STRATEGY_AFDO:
		storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
		if err != nil {
			return nil, err
		}
		return &AFDOStrategy{
			gcs: gcs.NewGCSClient(storageClient, AFDO_GS_BUCKET),
		}, nil
	case ROLL_STRATEGY_BATCH:
		return StrategyHead(branch), nil
	case ROLL_STRATEGY_FUCHSIA_SDK:
		return nil, nil // Handled by FuchsiaSDKRepoManager.
	case ROLL_STRATEGY_LKGR:
		return StrategyLKGR(lkgr), nil
	case ROLL_STRATEGY_REMOTE_BATCH:
		return StrategyRemoteHead(branch, upstreamRemote, repo), nil
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
func (s *headStrategy) GetNextRollRev(ctx context.Context, notRolled []*vcsinfo.LongCommit) (string, error) {
	if len(notRolled) > 0 {
		// Commits are listed in reverse chronological order.
		return notRolled[0].Hash, nil
	}
	return "", nil
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
	branch         string
	repo           *git.Checkout
	upstreamRemote string
}

// See documentation for NextRollStrategy interface.
func (s *remoteHeadStrategy) GetNextRollRev(ctx context.Context, _ []*vcsinfo.LongCommit) (string, error) {
	output, err := s.repo.Git(ctx, "ls-remote", s.upstreamRemote, fmt.Sprintf("refs/heads/%s", s.branch), "-1")
	if err != nil {
		return "", err
	}
	tokens := strings.Split(output, "\t")
	return tokens[0], nil
}

// StrategyRemoteHead returns a NextRollStrategy which always rolls to HEAD of a
// given branch, as defined by "git ls-remote".
func StrategyRemoteHead(branch, upstreamRemote string, repo *git.Checkout) NextRollStrategy {
	return &remoteHeadStrategy{
		branch:         branch,
		repo:           repo,
		upstreamRemote: upstreamRemote,
	}
}

// singleStrategy is a NextRollStrategy which rolls toward HEAD of a given branch, one
// commit at a time.
type singleStrategy struct {
	*headStrategy
}

// See documentation for NextRollStrategy interface.
func (s *singleStrategy) GetNextRollRev(ctx context.Context, notRolled []*vcsinfo.LongCommit) (string, error) {
	if len(notRolled) > 0 {
		return notRolled[len(notRolled)-1].Hash, nil
	}
	return "", nil
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
func (s *urlStrategy) GetNextRollRev(ctx context.Context, _ []*vcsinfo.LongCommit) (string, error) {
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
