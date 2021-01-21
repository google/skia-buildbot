package syncer

import (
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

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
