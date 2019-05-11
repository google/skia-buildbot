package syncer

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	// Use this as an expected error when you don't care about the actual
	// error which is returned.
	ERR_DONT_CARE = fmt.Errorf("DONTCARE")
)

func tempGitRepoSetup(t *testing.T) (context.Context, *git_testutils.GitBuilder, string, string) {
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	gb.Add(ctx, "codereview.settings", `CODE_REVIEW_SERVER: codereview.chromium.org
PROJECT: skia`)
	c1 := gb.CommitMsg(ctx, "initial commit")
	c2 := gb.CommitGen(ctx, "somefile")
	return ctx, gb, c1, c2
}

func tempGitRepoBotUpdateTests(t *testing.T, cases map[types.RepoState]error) {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	ctx := context.Background()
	cacheDir := path.Join(tmp, "cache")
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	for rs, expectErr := range cases {
		c, err := tempGitRepoBotUpdate(ctx, rs, depotTools, cacheDir, tmp)
		if expectErr != nil {
			assert.Error(t, err)
			if expectErr != ERR_DONT_CARE {
				assert.EqualError(t, err, expectErr.Error())
			}
		} else {
			defer c.Delete()
			assert.NoError(t, err)
			output, err := c.Git(ctx, "remote", "-v")
			gotRepo := "COULD NOT FIND REPO"
			for _, s := range strings.Split(output, "\n") {
				if strings.HasPrefix(s, "origin") {
					split := strings.Fields(s)
					assert.Equal(t, 3, len(split))
					gotRepo = split[1]
					break
				}
			}
			assert.Equal(t, rs.Repo, gotRepo)
			gotRevision, err := c.RevParse(ctx, "HEAD")
			assert.NoError(t, err)
			assert.Equal(t, rs.Revision, gotRevision)
			// If not a try job, we expect a clean checkout,
			// otherwise we expect a dirty checkout, from the
			// applied patch.
			_, err = c.Git(ctx, "diff", "--exit-code", "--no-patch", rs.Revision)
			if rs.IsTryJob() {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestTempGitRepo(t *testing.T) {
	unittest.LargeTest(t)
	_, gb, c1, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	cases := map[types.RepoState]error{
		{
			Repo:     gb.RepoUrl(),
			Revision: c1,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: "bogusRev",
		}: ERR_DONT_CARE,
	}
	tempGitRepoBotUpdateTests(t, cases)
}

func TestTempGitRepoPatch(t *testing.T) {
	unittest.LargeTest(t)

	ctx, gb, _, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	issue := "12345"
	patchset := "3"
	gb.CreateFakeGerritCLGen(ctx, issue, patchset)

	cases := map[types.RepoState]error{
		{
			Patch: types.Patch{
				Server:   gb.RepoUrl(),
				Issue:    issue,
				Patchset: patchset,
			},
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
	}
	tempGitRepoBotUpdateTests(t, cases)
}

func TestTempGitRepoParallel(t *testing.T) {
	unittest.LargeTest(t)

	ctx, gb, c1, _ := tcc_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	s := New(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS)
	defer testutils.AssertCloses(t, s)
	rs := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, s.TempGitRepo(ctx, rs, func(g *git.TempCheckout) error {
				return nil
			}))
		}()
	}
	wg.Wait()
}

func TestTempGitRepoErr(t *testing.T) {
	// bot_update uses lots of retries with exponential backoff, which makes
	// this really slow.
	unittest.ManualTest(t)
	unittest.LargeTest(t)

	ctx, gb, c1, _ := tcc_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	s := New(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS)
	defer testutils.AssertCloses(t, s)

	// bot_update will fail to apply the issue if we don't fake it in Git.
	rs := types.RepoState{
		Patch: types.Patch{
			Issue:    "my-issue",
			Patchset: "my-patchset",
			Server:   "my-server",
		},
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	assert.Error(t, s.TempGitRepo(ctx, rs, func(c *git.TempCheckout) error {
		// This may fail with a nil pointer dereference due to a nil
		// git.TempCheckout.
		assert.FailNow(t, "We should not have gotten here.")
		return nil
	}))
}

func TestLazyTempGitRepo(t *testing.T) {
	unittest.LargeTest(t)
	// TODO(borenet): This test only takes ~5 seconds locally, but for some
	// reason it consistently times out after 4 minutes on the bots.
	unittest.ManualTest(t)

	ctx, gb, c1, _ := tcc_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	botUpdateCount := 0
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		for _, arg := range cmd.Args {
			if strings.Contains(arg, "bot_update") {
				botUpdateCount++
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx = exec.NewContext(context.Background(), mockRun.Run)

	s := New(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS)
	defer testutils.AssertCloses(t, s)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	ltgr := s.LazyTempGitRepo(rs1)

	// This isn't a great marker, but it indicates whether the goroutine
	// with the TempGitRepo is running.
	assert.Nil(t, ltgr.queue)

	ran := false
	assert.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran = true
		return nil
	}))
	assert.True(t, ran)
	assert.Equal(t, 1, botUpdateCount)

	// See above comment.
	assert.NotNil(t, ltgr.queue)

	ran2 := false
	assert.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran2 = true
		return nil
	}))
	assert.True(t, ran2)
	assert.Equal(t, 1, botUpdateCount)

	// See above comment.
	assert.NotNil(t, ltgr.queue)

	ltgr.Done()

	// What happens if we hit a sync error?
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
		Patch: types.Patch{
			Issue:    "12345",
			Patchset: "1",
			Server:   "fake.fake/fake",
		},
	}
	ltgr = s.LazyTempGitRepo(rs2)
	notSyncError := errors.New("not a sync error")
	syncErr := ltgr.Do(ctx, func(co *git.TempCheckout) error {
		return notSyncError
	})
	assert.NotNil(t, syncErr)
	assert.NotEqual(t, syncErr, notSyncError)
	assert.Equal(t, 2, botUpdateCount)
	// Subsequent calls should receive the same sync error, without another
	// bot_update invocation.
	err = ltgr.Do(ctx, func(co *git.TempCheckout) error {
		return notSyncError
	})
	assert.NotNil(t, err)
	assert.EqualError(t, syncErr, err.Error())
	assert.Equal(t, 2, botUpdateCount)
	ltgr.Done()

	// Errors returned by passed-in funcs should be forwarded through to
	// the caller.
	ltgr = s.LazyTempGitRepo(rs1)
	err = ltgr.Do(ctx, func(co *git.TempCheckout) error {
		return notSyncError
	})
	assert.EqualError(t, notSyncError, err.Error())
	assert.Equal(t, 3, botUpdateCount)
	// ... but we should still be able to run other funcs.
	ran = false
	assert.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran = true
		return nil
	}))
	assert.True(t, ran)
	ltgr.Done()
}
