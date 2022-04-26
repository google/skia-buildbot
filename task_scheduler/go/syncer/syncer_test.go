package syncer

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
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
	// Skip download-topics in gclient calls to avoid that network call.
	ctx = context.WithValue(ctx, SkipDownloadTopicsKey, true)
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

	s := New(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DefaultNumWorkers)
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
