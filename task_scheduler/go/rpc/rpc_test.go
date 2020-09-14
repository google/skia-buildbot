package rpc

import (
	context "context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/testutils/mem_git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mem_gitstore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// Fake user emails.
	viewer = "viewer@google.com"
	editor = "editor@google.com"
	admin  = "admin@google.com"

	fakeRepo = "fake.git"
)

var (
	// Allow fake users.
	viewers = allowed.NewAllowedFromList([]string{viewer, editor, admin})
	editors = allowed.NewAllowedFromList([]string{editor, admin})
	admins  = allowed.NewAllowedFromList([]string{admin})
)

func setup(t *testing.T) (context.Context, *taskSchedulerServiceImpl, func()) {
	ctx := context.Background()
	d := memory.NewInMemoryDB()
	gs := mem_gitstore.New()
	gb := mem_git.New(t, gs)
	hashes := gb.CommitN(ctx, 2)
	ri, err := gitstore.NewGitStoreRepoImpl(ctx, gs)
	require.NoError(t, err)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	repos := repograph.Map{
		fakeRepo: repo,
	}
	fsClient, cleanupFS := firestore.NewClientForTesting(context.Background(), t)
	skipDB, err := skip_tasks.New(context.Background(), fsClient)
	require.NoError(t, err)
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)
	for _, hash := range hashes {
		rs := types.RepoState{
			Repo:     fakeRepo,
			Revision: hash,
		}
		cfg := &specs.TasksCfg{
			Jobs: map[string]*specs.JobSpec{
				"job": {
					TaskSpecs: []string{"task"},
				},
			},
			Tasks: map[string]*specs.TaskSpec{
				"task": {},
			},
		}
		require.NoError(t, tcc.Set(ctx, rs, cfg, nil))
	}
	srv := newTaskSchedulerServiceImpl(ctx, d, repos, skipDB, tcc, viewers, editors, admins)
	return ctx, srv, func() {
		btCleanup()
		cleanupFS()
	}
}

func TestTriggerJobs(t *testing.T) {
	unittest.LargeTest(t)

	ctx, srv, cleanup := setup(t)
	defer cleanup()

	commit := srv.repos[fakeRepo].Get(git.DefaultBranch).Hash
	req := &TriggerJobsRequest{
		Jobs: []*TriggerJob{
			{
				JobName:    "job",
				CommitHash: commit,
			},
			{
				JobName:    "job",
				CommitHash: commit,
			},
		},
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.TriggerJobs(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	mockUser = viewer
	res, err = srv.TriggerJobs(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check results.
	mockUser = editor
	res, err = srv.TriggerJobs(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 2, len(res.JobIds))
	for _, id := range res.JobIds {
		require.NotEqual(t, "", id)
	}
}

func TestGetJob(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestCancelJob(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestSearchJobs(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestGetTask(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestSearchTasks(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestGetSkipTaskRules(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestAddSkipTaskRule(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestDeleteSkipTaskRule(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestConvertRepoState(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestConvertTaskStatus(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestConvertTask(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestConvertJobStatus(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}

func TestConvertJob(t *testing.T) {
	unittest.LargeTest(t)

	// TODO
}
