//go:build linux
// +build linux

package repo_manager

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func commandsEqual(a, b *exec.Command) bool {
	if a.Name != b.Name {
		return false
	}
	if !util.SSliceEqual(a.Args, b.Args) {
		return false
	}
	if a.Dir != b.Dir {
		return false
	}
	if a.InheritEnv != b.InheritEnv {
		return false
	}
	if a.InheritPath != b.InheritPath {
		return false
	}
	if !util.SSliceEqual(a.Env, b.Env) {
		return false
	}
	return (a.Name == b.Name &&
		util.SSliceEqual(a.Args, b.Args) &&
		a.Dir == b.Dir &&
		a.InheritEnv == b.InheritEnv &&
		a.InheritPath == b.InheritPath &&
		util.SSliceEqual(a.Env, b.Env))
}

func TestCommandRepoManager(t *testing.T) {
	unittest.MediumTest(t) // Uses the filesystem

	const tipRev0 = "tipRev0"
	const pinnedRev0 = "pinnedRev0"

	// Setup.
	ctx := context.Background()
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	urlmock := mockhttpclient.NewURLMock()
	g := setupFakeGerrit(t, depsCfg(t).GetDepsLocalGerritParent().Gerrit, urlmock)
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "version", "pinnedRev0")
	parent.Commit(ctx)
	baseDir := filepath.Join(tmp, filepath.Base(parent.RepoUrl()))

	// Commands used by this CommandRepoManager.
	vars := &CommandTmplVars{
		RollingFrom: pinnedRev0,
		RollingTo:   tipRev0,
	}
	getTipRev := &config.CommandRepoManagerConfig_CommandConfig{
		Command: []string{"echo", tipRev0},
		Dir:     ".",
		Env: []string{
			"key=val",
		},
	}
	getTipRevCmd, err := makeCommand(getTipRev, baseDir, vars)
	require.NoError(t, err)
	getTipRevCount := 0

	getPinnedRev := &config.CommandRepoManagerConfig_CommandConfig{
		Command: []string{"cat", "version"},
		Dir:     ".",
		Env: []string{
			"key2=val2",
		},
	}
	getPinnedRevCmd, err := makeCommand(getPinnedRev, baseDir, vars)
	require.NoError(t, err)
	getPinnedRevCount := 0

	setPinnedRev := &config.CommandRepoManagerConfig_CommandConfig{
		Command: []string{"bash", "-c", "echo \"{{.RollingTo}}\" > version"},
		Dir:     ".",
		Env: []string{
			"key3=val3",
		},
	}
	setPinnedRevCmd, err := makeCommand(setPinnedRev, baseDir, vars)
	require.NoError(t, err)
	setPinnedRevCount := 0

	cfg := &config.CommandRepoManagerConfig{
		GitCheckout: &config.GitCheckoutConfig{
			Branch:  git.MasterBranch,
			RepoUrl: parent.RepoUrl(),
		},
		GetTipRev:    getTipRev,
		GetPinnedRev: getPinnedRev,
		SetPinnedRev: setPinnedRev,
	}

	// Mock all commands. If the command is one of the three special commands
	// for this repo manager, verify that it matches expectations.
	lastUpload := new(vcsinfo.LongCommit)
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if commandsEqual(cmd, getTipRevCmd) {
			getTipRevCount++
		}
		if commandsEqual(cmd, getPinnedRevCmd) {
			getPinnedRevCount++
		}
		if commandsEqual(cmd, setPinnedRevCmd) {
			setPinnedRevCount++
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
	rm, err := NewCommandRepoManager(ctx, cfg, setupRegistry(t), tmp, "fake.server.com", gerritCR(t, g, urlmock.Client()))
	require.NoError(t, err)
	require.Equal(t, 0, getTipRevCount)
	require.Equal(t, 0, getPinnedRevCount)
	require.Equal(t, 0, setPinnedRevCount)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.NotNil(t, lastRollRev)
	require.NotNil(t, tipRev)
	require.NotNil(t, notRolledRevs)
	require.Equal(t, 1, getTipRevCount)
	require.Equal(t, 1, getPinnedRevCount)
	require.Equal(t, 0, setPinnedRevCount)
	require.Equal(t, pinnedRev0, lastRollRev.Id)
	require.Equal(t, tipRev0, tipRev.Id)
	require.Len(t, notRolledRevs, 1)
	require.Equal(t, tipRev0, notRolledRevs[0].Id)

	// Mock the request to load the change.
	// TODO(borenet): Refactor Gerrit mocks.
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Id:       "123",
		Project:  "test-project",
		Branch:   "test-branch",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS&o=SUBMITTABLE", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody := []byte(`{}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@google.com"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/test-project~test-branch~123/revisions/ps2/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"reviewer@google.com"}, false, "fake-commit-msg")
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}
