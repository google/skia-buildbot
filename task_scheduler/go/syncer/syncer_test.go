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

	"github.com/stretchr/testify/require"
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

func tempGitRepoGclientTests(t *testing.T, cases map[types.RepoState]error) {
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	ctx := context.Background()
	cacheDir := path.Join(tmp, "cache")
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	for rs, expectErr := range cases {
		c, err := tempGitRepoGclient(ctx, rs, depotTools, cacheDir, tmp)
		if expectErr != nil {
			require.Error(t, err)
			if expectErr != ERR_DONT_CARE {
				require.EqualError(t, err, expectErr.Error())
			}
		} else {
			defer c.Delete()
			require.NoError(t, err)
			output, err := c.Git(ctx, "remote", "-v")
			gotRepo := "COULD NOT FIND REPO"
			for _, s := range strings.Split(output, "\n") {
				if strings.HasPrefix(s, git.DefaultRemote) {
					split := strings.Fields(s)
					require.Equal(t, 3, len(split))
					gotRepo = split[1]
					break
				}
			}
			require.Equal(t, rs.Repo, gotRepo)
			gotRevision, err := c.RevParse(ctx, "HEAD")
			require.NoError(t, err)
			require.Equal(t, rs.Revision, gotRevision)
			// If not a try job, we expect a clean checkout,
			// otherwise we expect a dirty checkout, from the
			// applied patch.
			_, err = c.Git(ctx, "diff", "--exit-code", "--no-patch", rs.Revision)
			if rs.IsTryJob() {
				require.NotNil(t, err)
			} else {
				require.NoError(t, err)
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
	}
	tempGitRepoGclientTests(t, cases)
}

func TestTempGitRepoBadRev(t *testing.T) {
	// TODO(borenet): Git wrapper automatically retries commands when it
	// encounters "transient" errors. I'm not sure why it thinks "fatal:
	// couldn't find remote ref" is transient, but these retries cause the
	// test to time out.
	unittest.ManualTest(t)
	unittest.LargeTest(t)
	_, gb, _, _ := tempGitRepoSetup(t)
	defer gb.Cleanup()

	cases := map[types.RepoState]error{
		{
			Repo:     gb.RepoUrl(),
			Revision: "bogusRev",
		}: ERR_DONT_CARE,
	}
	tempGitRepoGclientTests(t, cases)
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
	tempGitRepoGclientTests(t, cases)
}

func TestTempGitRepoParallel(t *testing.T) {
	unittest.LargeTest(t)

	ctx, gb, c1, _ := tcc_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)

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
			require.NoError(t, s.TempGitRepo(ctx, rs, func(g *git.TempCheckout) error {
				return nil
			}))
		}()
	}
	wg.Wait()
}

func TestTempGitRepoErr(t *testing.T) {
	// TODO(borenet): Git wrapper automatically retries commands when it
	// encounters "transient" errors. I'm not sure why it thinks this error
	// is transient, but these retries cause the test to time out.
	unittest.ManualTest(t)
	unittest.LargeTest(t)

	ctx, gb, c1, _ := tcc_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)

	s := New(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS)
	defer testutils.AssertCloses(t, s)

	// gclient will fail to apply the issue if we don't fake it in Git.
	rs := types.RepoState{
		Patch: types.Patch{
			Issue:    "my-issue",
			Patchset: "my-patchset",
			Server:   "my-server",
		},
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	require.Error(t, s.TempGitRepo(ctx, rs, func(c *git.TempCheckout) error {
		// This may fail with a nil pointer dereference due to a nil
		// git.TempCheckout.
		require.FailNow(t, "We should not have gotten here.")
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
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)

	syncCount := 0
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		gclient := false
		sync := false
		for _, arg := range cmd.Args {
			if strings.Contains(arg, "gclient") {
				gclient = true
			}
			if strings.Contains(arg, "sync") {
				sync = true
			}
		}
		if gclient && sync {
			syncCount++
		}
		return exec.DefaultRun(ctx, cmd)
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
	require.Nil(t, ltgr.queue)

	ran := false
	require.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran = true
		return nil
	}))
	require.True(t, ran)
	require.Equal(t, 1, syncCount)

	// See above comment.
	require.NotNil(t, ltgr.queue)

	ran2 := false
	require.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran2 = true
		return nil
	}))
	require.True(t, ran2)
	require.Equal(t, 1, syncCount)

	// See above comment.
	require.NotNil(t, ltgr.queue)

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
	require.NotNil(t, syncErr)
	require.NotEqual(t, syncErr, notSyncError)
	require.Equal(t, 2, syncCount)
	// Subsequent calls should receive the same sync error, without another
	// "gclient sync" invocation.
	err = ltgr.Do(ctx, func(co *git.TempCheckout) error {
		return notSyncError
	})
	require.NotNil(t, err)
	require.EqualError(t, syncErr, err.Error())
	require.Equal(t, 2, syncCount)
	ltgr.Done()

	// Errors returned by passed-in funcs should be forwarded through to
	// the caller.
	ltgr = s.LazyTempGitRepo(rs1)
	err = ltgr.Do(ctx, func(co *git.TempCheckout) error {
		return notSyncError
	})
	require.EqualError(t, notSyncError, err.Error())
	require.Equal(t, 3, syncCount)
	// ... but we should still be able to run other funcs.
	ran = false
	require.NoError(t, ltgr.Do(ctx, func(co *git.TempCheckout) error {
		ran = true
		return nil
	}))
	require.True(t, ran)
	ltgr.Done()
}
