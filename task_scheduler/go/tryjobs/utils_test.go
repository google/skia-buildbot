package tryjobs

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now"
	pubsub_mocks "go.skia.org/infra/go/pubsub/mocks"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	cacher_mocks "go.skia.org/infra/task_scheduler/go/cacher/mocks"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	tcc_mocks "go.skia.org/infra/task_scheduler/go/task_cfg_cache/mocks"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	repoUrl = "skia.git"

	gerritIssue    = 2112
	gerritPatchset = 3
	patchProject   = "skia"
	parentProject  = "parent-project"

	fakeGerritUrl     = "https://fake-skia-review.googlesource.com"
	oldBranchName     = "old-branch"
	bbPubSubProject   = "fake-bb-pubsub-project"
	bbPubSubTopic     = "fake-bb-pubsub-topic"
	bbFakeStartToken  = "fake-bb-start-token"
	bbFakeUpdateToken = "fake-bb-update-token"
)

var (
	gerritPatch = types.Patch{
		Server:    fakeGerritUrl,
		Issue:     fmt.Sprintf("%d", gerritIssue),
		PatchRepo: repoUrl,
		Patchset:  fmt.Sprintf("%d", gerritPatchset),
	}

	commit1 = &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "abc123",
			Author:  "me@google.com",
			Subject: "initial commit",
		},
		Branches: map[string]bool{
			git.MainBranch: true,
		},
	}

	commit2 = &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "def456",
			Author:  "me@google.com",
			Subject: "second commit",
		},
		Branches: map[string]bool{
			git.MainBranch: true,
		},
		Parents: []string{commit1.Hash},
	}

	repoState1 = types.RepoState{
		Patch:    gerritPatch,
		Repo:     repoUrl,
		Revision: commit1.Hash,
	}
	repoState2 = types.RepoState{
		Patch:    gerritPatch,
		Repo:     repoUrl,
		Revision: commit2.Hash,
	}

	// Arbitrary start time to keep tests consistent.
	ts = time.Unix(1632920378, 0)
)

// setup prepares the tests to run. Returns the created temporary dir,
// TryJobIntegrator instance, and URLMock instance.
func setup(t sktest.TestingT) (context.Context, *TryJobIntegrator, *mockhttpclient.URLMock, *mocks.BuildBucketInterface, *pubsub_mocks.Topic) {
	ctx := context.WithValue(context.Background(), now.ContextKey, ts)

	// Set up other TryJobIntegrator inputs.
	taskCfgCache := tcc_mocks.FixedTasksCfg(tcc_testutils.TasksCfg1)
	d := memory.NewInMemoryDB()
	mock := mockhttpclient.NewURLMock()
	projectRepoMapping := map[string]string{
		patchProject: repoUrl,
	}
	g, err := gerrit.NewGerrit(fakeGerritUrl, mock.Client())
	require.NoError(t, err)
	chr := &cacher_mocks.Cacher{}
	chr.On("GetOrCacheRepoState", testutils.AnyContext, repoState1).Return(tcc_testutils.TasksCfg1, nil)
	chr.On("GetOrCacheRepoState", testutils.AnyContext, repoState2).Return(tcc_testutils.TasksCfg1, nil)

	branch1 := &git.Branch{
		Name: git.MainBranch,
		Head: commit2.Hash,
	}
	branch2 := &git.Branch{
		Name: oldBranchName,
		Head: commit1.Hash,
	}
	repoImpl := repograph.NewMemCacheRepoImpl(map[string]*vcsinfo.LongCommit{
		commit1.Hash: commit1,
		commit2.Hash: commit2,
	}, []*git.Branch{branch1, branch2})
	repo, err := repograph.NewWithRepoImpl(ctx, repoImpl)
	require.NoError(t, err)
	rm := map[string]*repograph.Graph{
		repoUrl: repo,
	}
	window, err := window.New(ctx, time.Hour, 100, rm)
	require.NoError(t, err)
	jCache, err := cache.NewJobCache(ctx, d, window, nil)
	require.NoError(t, err)
	pubsubClient := &pubsub_mocks.Client{}
	pubsubClient.On("Project").Return(bbPubSubProject)
	pubsubTopic := &pubsub_mocks.Topic{}
	pubsubClient.On("TopicInProject", bbPubSubTopic, bbPubSubProject).Return(pubsubTopic, nil)
	integrator, err := NewTryJobIntegrator(ctx, "skia", "fake-bb-target", BUCKET_TESTING, "fake-server", mock.Client(), d, jCache, projectRepoMapping, rm, taskCfgCache, chr, g, pubsubClient)
	require.NoError(t, err)
	return ctx, integrator, mock, MockBuildbucket(integrator), pubsubTopic
}

func MockBuildbucket(tj *TryJobIntegrator) *mocks.BuildBucketInterface {
	bbMock := &mocks.BuildBucketInterface{}
	tj.bb2 = bbMock
	return bbMock
}

func tryjobV2(ctx context.Context) *types.Job {
	job := &types.Job{
		BuildbucketBuildId:  rand.Int63(),
		BuildbucketLeaseKey: rand.Int63(),
		Created:             now.Now(ctx),
		Name:                tcc_testutils.BuildTaskName,
		RepoState:           repoState2,
	}
	job.BuildbucketLeaseKey = 0
	job.BuildbucketToken = bbFakeStartToken
	job.BuildbucketPubSubTopic = fmt.Sprintf("projects/%s/topics/%s", bbPubSubProject, bbPubSubTopic)
	return job
}
