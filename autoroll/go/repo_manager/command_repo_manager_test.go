package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

func makeFakeCommand(cfg *CommandConfig, baseDir string) *exec.Command {
	cmd := exec.ParseCommand(cfg.Command)
	cmd.Dir = filepath.Join(baseDir, cfg.Dir)
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.InheritEnv = true
	return &cmd
}

func TestCommandRepoManager(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	ctx := context.Background()
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	baseDir := tmp // TODO
	urlmock := mockhttpclient.NewURLMock()
	g := setupFakeGerrit(t, depsCfg(t).Gerrit, urlmock)
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "dummy", "blahblahblah")
	parent.Commit(ctx)

	// Commands used by this CommandRepoManager.
	getTipRev := &CommandConfig{
		Command: "./get --tip-rev",
		Dir:     ".",
		Env: map[string]string{
			"key": "val",
		},
	}
	getTipRevCmd := makeFakeCommand(getTipRev, baseDir)
	getTipRevCount := 0

	getPinnedRev := &CommandConfig{
		Command: "./get --pinned-rev",
		Dir:     filepath.Join("pinned", "dir"),
		Env: map[string]string{
			"key2": "val2",
		},
	}
	getPinnedRevCmd := makeFakeCommand(getPinnedRev, baseDir)
	getPinnedRevCount := 0

	setPinnedRev := &CommandConfig{
		Command: "./set --pinned-rev",
		Dir:     filepath.Join("pinned", "dir", "set"),
		Env: map[string]string{
			"key3": "val3",
		},
	}
	setPinnedRevCmd := makeFakeCommand(setPinnedRev, baseDir)
	setPinnedRevCount := 0

	cfg := CommandRepoManagerConfig{
		GitCheckoutConfig: git_common.GitCheckoutConfig{
			Branch:  masterBranchTmpl(t),
			RepoURL: parent.RepoUrl(),
		},
		GetTipRev:    getTipRev,
		GetPinnedRev: getPinnedRev,
		SetPinnedRev: setPinnedRev,
	}

	// Mock all commands. If the command is one of the three special commands
	// for this repo manager, verify that it matches expectations.
	getTipRevOutput := "tipRev0"
	getPinnedRevOutput := "pinnedRev0"
	lastUpload := new(vcsinfo.LongCommit)
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if deepequal.DeepEqual(cmd, getTipRevCmd) {
			getTipRevCount++
			_, err := cmd.Stdout.Write([]byte(getTipRevOutput))
			require.NoError(t, err)
			return nil
		} else if deepequal.DeepEqual(cmd, getPinnedRevCmd) {
			getPinnedRevCount++
			_, err := cmd.Stdout.Write([]byte(getPinnedRevOutput))
			require.NoError(t, err)
			return nil
		} else if deepequal.DeepEqual(cmd, setPinnedRevCmd) {
			setPinnedRevCount++
			return nil
		}
		// Don't perform "git push".
		if strings.Contains(cmd.Name, "git") && cmd.Args[0] == "push" {
			d, err := git.GitDir(cmd.Dir).Details(ctx, "HEAD")
			if err != nil {
				return skerr.Wrap(err)
			}
			*lastUpload = *d
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	// Create the repo manager.
	rm, err := NewCommandRepoManager(ctx, cfg, setupRegistry(t), tmp, g, "fake.server.com", gerritCR(t, g))
	require.NoError(t, err)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.NotNil(t, lastRollRev)
	require.NotNil(t, tipRev)
	require.NotNil(t, notRolledRevs)
}
