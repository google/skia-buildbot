package catapult

import (
	"context"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"

	"go.skia.org/infra/pinpoint/go/backends"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"golang.org/x/oauth2/google"
)

type httpClientContext struct{}

var httpClientKey = &httpClientContext{}

// FetchTaskActivity fetches the task used for the given swarming task.
func FetchTaskActivity(ctx context.Context, taskID string) (*swarming.SwarmingRpcsTaskResult, error) {
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	task, err := sc.GetTask(ctx, taskID, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not fetch task %s", taskID)
	}

	return task, nil
}

func createNewRepo(ctx context.Context, repository string) (*gitiles.Repo, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "problem setting up default token source")
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()
	return gitiles.NewRepo(repository, httpClient), nil
}

func getRepository(ctx context.Context, repository string) (gitiles.GitilesRepo, error) {
	repositories, ok := ctx.Value(httpClientKey).(map[string]gitiles.GitilesRepo)
	if !ok {
		repo, err := createNewRepo(ctx, repository)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return repo, nil
	}
	repo, ok := repositories[repository]
	if !ok {
		repo, err := createNewRepo(ctx, repository)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return repo, nil
	}

	return repo, nil
}

// FetchCommitActivity fetches commit information and modifies the provided commit with additional information.
func FetchCommitActivity(ctx context.Context, commit *pinpoint_proto.Commit) (*vcsinfo.LongCommit, error) {
	repo, err := getRepository(ctx, commit.Repository)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	longCommit, err := repo.Details(ctx, commit.GitHash)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return longCommit, nil
}

// WriteBisectToCatapultActivity wraps the call to WriteBisectToCatapult
func WriteBisectToCatapultActivity(ctx context.Context, content *pinpoint_proto.LegacyJobResponse, staging bool) (*DatastoreResponse, error) {
	cc, err := NewCatapultClient(ctx, staging)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return cc.WriteBisectToCatapult(ctx, content)
}
