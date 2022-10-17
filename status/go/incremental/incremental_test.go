package incremental

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

func setup(t *testing.T) (context.Context, string, *IncrementalCacheImpl, repograph.Map, *memory.InMemoryDB, *git_testutils.GitBuilder, func()) {
	d := memory.NewInMemoryDB()

	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	c0 := gb.CommitGen(ctx, "placeholder")
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	repo, err := repograph.NewLocalGraph(ctx, gb.Dir(), workdir)
	require.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}

	initialTask := &types.Task{
		Created:    time.Now(),
		DbModified: time.Now(),
		Id:         "0",
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     gb.RepoUrl(),
				Revision: c0,
			},
			Name: "PlaceholderTask",
		},
	}
	require.NoError(t, d.PutTask(ctx, initialTask))

	w, err := window.New(ctx, 24*time.Hour, 100, repos)
	require.NoError(t, err)

	cache, err := NewIncrementalCacheImpl(ctx, d, w, repos, 100, "https://swarming", "https://task-scheduler")
	require.NoError(t, err)

	return ctx, workdir, cache, repos, d, gb, func() {
		testutils.RemoveAll(t, workdir)
		gb.Cleanup()
	}
}

func update(t *testing.T, ctx context.Context, repo string, c *IncrementalCacheImpl, ts time.Time) (*Update, time.Time) {
	require.NoError(t, c.Update(ctx, false))
	now := time.Now()
	u, err := c.Get(repo, ts, 100)
	require.NoError(t, err)
	return u, now
}

func TestIncrementalCacheImpl(t *testing.T) {
	ctx, _, cache, repos, taskDb, gb, cleanup := setup(t)
	defer cleanup()

	repoUrl := ""
	for r := range repos {
		repoUrl = r
		break
	}

	// Verify the initial state.
	require.Equal(t, 1, len(cache.updates[repoUrl]))
	ts := time.Now()
	ts0 := ts // Used later.
	u, err := cache.GetAll(repoUrl, 100)
	require.NoError(t, err)
	startOver := new(bool)
	*startOver = true
	require.Equal(t, 1, len(u.BranchHeads))
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, 1, len(u.Commits))
	require.Equal(t, startOver, u.StartOver)
	require.Equal(t, "https://swarming", u.SwarmingUrl)
	require.Equal(t, map[string]map[string][]*TaskComment(nil), u.TaskComments)
	require.Equal(t, 1, len(u.Tasks))
	require.Equal(t, "https://task-scheduler", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// Add different types of elements, one by one, and verify that they
	// are represented in new updates.

	// Modify the task.
	wait := make(chan struct{})
	cache.tasks.setTasksCallback(func() {
		wait <- struct{}{}
	})
	t0, err := taskDb.GetTaskById(ctx, u.Tasks[0].Id)
	require.NoError(t, err)
	t0.Status = types.TASK_STATUS_SUCCESS
	require.NoError(t, taskDb.PutTask(ctx, t0))
	taskDb.Wait()
	<-wait
	u, ts = update(t, ctx, repoUrl, cache, ts)
	// Expect a mostly-empty update with just the updated task.
	require.Equal(t, []*git.Branch(nil), u.BranchHeads)
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, []*vcsinfo.LongCommit(nil), u.Commits)
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	require.Equal(t, map[string]map[string][]*TaskComment(nil), u.TaskComments)
	require.Equal(t, 1, len(u.Tasks))
	require.Equal(t, "", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// Add a TaskComment.
	tc := types.TaskComment{
		Repo:      t0.Repo,
		Revision:  t0.Revision,
		Name:      t0.Name,
		Timestamp: time.Now(),
		TaskId:    t0.Id,
		User:      "me",
		Message:   "here's a task comment.",
	}
	require.NoError(t, taskDb.PutTaskComment(ctx, &tc))
	u, ts = update(t, ctx, repoUrl, cache, ts)
	// Expect a mostly-empty update with just the new TaskComment.
	require.Equal(t, []*git.Branch(nil), u.BranchHeads)
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, []*vcsinfo.LongCommit(nil), u.Commits)
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	assertdeep.Equal(t, tc, u.TaskComments[t0.Revision][t0.Name][0].TaskComment)
	require.Equal(t, []*Task(nil), u.Tasks)
	require.Equal(t, "", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// Verify that both the task from the previous update AND the
	// TaskComment appear if we request an earlier timestamp.
	u, err = cache.Get(repoUrl, ts0, 100)
	require.Equal(t, []*git.Branch(nil), u.BranchHeads)
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, []*vcsinfo.LongCommit(nil), u.Commits)
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	assertdeep.Equal(t, tc, u.TaskComments[t0.Revision][t0.Name][0].TaskComment)
	require.Equal(t, 1, len(u.Tasks))
	require.Equal(t, "", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// CommitComment.
	cc := types.CommitComment{
		Repo:          t0.Repo,
		Revision:      t0.Revision,
		Timestamp:     time.Now(),
		User:          "me",
		IgnoreFailure: true,
		Message:       "here's a commit comment",
	}
	require.NoError(t, taskDb.PutCommitComment(ctx, &cc))
	u, ts = update(t, ctx, repoUrl, cache, ts)
	// Expect a mostly-empty update with just the new CommitComment.
	require.Equal(t, []*git.Branch(nil), u.BranchHeads)
	assertdeep.Equal(t, cc, u.CommitComments[t0.Revision][0].CommitComment)
	require.Equal(t, []*vcsinfo.LongCommit(nil), u.Commits)
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	require.Equal(t, map[string]map[string][]*TaskComment(nil), u.TaskComments)
	require.Equal(t, []*Task(nil), u.Tasks)
	require.Equal(t, "", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// TaskSpecComment.
	tsc := types.TaskSpecComment{
		Repo:          t0.Repo,
		Name:          t0.Name,
		Timestamp:     time.Now(),
		User:          "me",
		Flaky:         true,
		IgnoreFailure: true,
		Message:       "here's a task spec comment",
	}
	require.NoError(t, taskDb.PutTaskSpecComment(ctx, &tsc))
	u, ts = update(t, ctx, repoUrl, cache, ts)
	// Expect a mostly-empty update with just the new TaskSpecComment.
	require.Equal(t, []*git.Branch(nil), u.BranchHeads)
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, []*vcsinfo.LongCommit(nil), u.Commits)
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	require.Equal(t, map[string]map[string][]*TaskComment(nil), u.TaskComments)
	require.Equal(t, []*Task(nil), u.Tasks)
	require.Equal(t, "", u.TaskSchedulerUrl)
	assertdeep.Equal(t, tsc, u.TaskSpecComments[t0.Name][0].TaskSpecComment)

	// Add a new commit.
	gb.CommitGen(ctx, "placeholder")
	u, ts = update(t, ctx, repoUrl, cache, ts)
	// Expect a mostly-empty update with just the new commit and the branch heads..
	require.Equal(t, 1, len(u.BranchHeads))
	require.Equal(t, map[string][]*CommitComment(nil), u.CommitComments)
	require.Equal(t, 1, len(u.Commits))
	require.Equal(t, (*bool)(nil), u.StartOver)
	require.Equal(t, "", u.SwarmingUrl)
	require.Equal(t, map[string]map[string][]*TaskComment(nil), u.TaskComments)
	require.Equal(t, []*Task(nil), u.Tasks)
	require.Equal(t, "", u.TaskSchedulerUrl)
	require.Equal(t, map[string][]*TaskSpecComment(nil), u.TaskSpecComments)

	// This will cause the cache to reload from scratch.
	require.NoError(t, cache.Update(ctx, true))
	// Expect the update to contain ALL of the information we've seen so
	// far, even though we're requesting the most recent.
	u, ts = update(t, ctx, repoUrl, cache, ts)
	require.Equal(t, 1, len(u.BranchHeads))
	assertdeep.Equal(t, cc, u.CommitComments[t0.Revision][0].CommitComment)
	require.Equal(t, 2, len(u.Commits))
	require.Equal(t, startOver, u.StartOver)
	require.Equal(t, "https://swarming", u.SwarmingUrl)
	assertdeep.Equal(t, tc, u.TaskComments[t0.Revision][t0.Name][0].TaskComment)
	require.Equal(t, 1, len(u.Tasks))
	require.Equal(t, "https://task-scheduler", u.TaskSchedulerUrl)
	assertdeep.Equal(t, tsc, u.TaskSpecComments[t0.Name][0].TaskSpecComment)
}
