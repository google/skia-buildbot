package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2"
)

type fakeTokenSource struct{}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "fake-token",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

type fakeGerrit struct {
	gerrit.GerritInterface
}

func (f *fakeGerrit) GetChange(ctx context.Context, id string) (*gerrit.ChangeInfo, error) {
	return &gerrit.ChangeInfo{
		ChangeId: "I123456",
		Issue:    123456,
		Status:   gerrit.ChangeStatusMerged,
	}, nil
}

func setupTempDir(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	return tmpDir, func() { _ = os.Chdir(cwd) }
}

func setupMockContext(ctx context.Context) (context.Context, *exec.CommandCollector) {
	mock := &exec.CommandCollector{}
	mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if len(cmd.Args) > 0 && cmd.Args[0] == "push" {
			_, _ = cmd.CombinedOutput.Write([]byte("remote: https://skia-review.googlesource.com/c/buildbot/+/123456 Uploaded CL\n"))
		} else if len(cmd.Args) > 0 && cmd.Args[0] == "--version" {
			_, _ = cmd.CombinedOutput.Write([]byte("git version 2.40.1\n"))
		}
		return nil
	})
	ctx = td.WithExecRunFn(ctx, mock.Run)
	ctx = git_common.WithGitFinder(ctx, func() (string, error) { return "/usr/bin/git", nil })
	ctx = auth_steps.WithTokenSource(ctx, &fakeTokenSource{})
	return ctx, mock
}

func TestRun_InvalidAction_ReturnsError(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		err := run(ctx, "invalid", "perf/FREEZELOCK", "test@example.com", &fakeGerrit{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid action")
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestRun_ActionFreeze_FileExists_ReturnsNilAndDoesNothing(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		tmpDir, cleanup := setupTempDir(t)
		defer cleanup()

		freezeFileName := "perf/FREEZELOCK"
		err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, freezeFileName)), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, freezeFileName), []byte("FREEZE"), 0644)
		require.NoError(t, err)

		err = run(ctx, "freeze", freezeFileName, "test@example.com", &fakeGerrit{})
		require.NoError(t, err)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestRun_ActionUnfreeze_FileDoesNotExist_ReturnsNilAndDoesNothing(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		_, cleanup := setupTempDir(t)
		defer cleanup()

		freezeFileName := "perf/FREEZELOCK"

		err := run(ctx, "unfreeze", freezeFileName, "test@example.com", &fakeGerrit{})
		require.NoError(t, err)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestRun_ActionFreeze_FileDoesNotExist_CreatesFileAndPushesCL(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		_, cleanup := setupTempDir(t)
		defer cleanup()

		freezeFileName := "perf/FREEZELOCK"

		ctx, mock := setupMockContext(ctx)
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		err := run(ctx, "freeze", freezeFileName, "test@example.com", &fakeGerrit{})
		assert.NoError(t, err)

		// Verify file was indeed created locally
		_, err = os.Stat(freezeFileName)
		assert.NoError(t, err)

		executedCommands := mock.Commands()
		for _, cmd := range executedCommands {
			if len(cmd.Args) >= 3 && cmd.Args[0] == "commit" {
				// The commit message has a random Change-Id at the end, so we ignore it
				// by checking its prefix and setting it to a fixed string.
				require.True(t, strings.HasPrefix(cmd.Args[2], "Toggle freeze perf/FREEZELOCK\n\nChange-Id: I"))
				cmd.Args[2] = "Toggle freeze perf/FREEZELOCK"
			}
		}

		testutils.AssertCommandsMatch(t, [][]string{
			{"/usr/bin/git", "--version"},
			{"/usr/bin/git", "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{"/usr/bin/git", "config", "--global", "user.email", "test@example.com"},
			{"/usr/bin/git", "config", "--global", "user.name", "test"},
			{"/usr/bin/git", "config", "--list", "--show-origin"},
			{"/usr/bin/git", "--version"},
			{"/usr/bin/git", "add", "perf/FREEZELOCK"},
			{"/usr/bin/git", "commit", "-m", "Toggle freeze perf/FREEZELOCK"},
			{"/usr/bin/git", "push", "origin", "HEAD:refs/for/main%ready,notify=OWNER_REVIEWERS,l=Auto-Submit+1,r=rubber-stamper@appspot.gserviceaccount.com"},
		}, executedCommands)

		return nil
	})

	// Ensure that no step panicked or caused unexpected framework errors
	require.Empty(t, res.Exceptions)
}

func TestRun_ActionUnfreeze_FileExists_DeletesFileAndPushesCL(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		tmpDir, cleanup := setupTempDir(t)
		defer cleanup()

		freezeFileName := "perf/FREEZELOCK"
		err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, freezeFileName)), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, freezeFileName), []byte("FREEZE"), 0644)
		require.NoError(t, err)

		ctx, mock := setupMockContext(ctx)
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		err = run(ctx, "unfreeze", freezeFileName, "test@example.com", &fakeGerrit{})
		assert.NoError(t, err)

		// Verify file was indeed deleted locally
		_, err = os.Stat(freezeFileName)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		executedCommands := mock.Commands()
		for _, cmd := range executedCommands {
			if len(cmd.Args) >= 3 && cmd.Args[0] == "commit" {
				require.True(t, strings.HasPrefix(cmd.Args[2], "Toggle freeze perf/FREEZELOCK\n\nChange-Id: I"))
				cmd.Args[2] = "Toggle freeze perf/FREEZELOCK"
			}
		}

		testutils.AssertCommandsMatch(t, [][]string{
			{"/usr/bin/git", "--version"},
			{"/usr/bin/git", "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{"/usr/bin/git", "config", "--global", "user.email", "test@example.com"},
			{"/usr/bin/git", "config", "--global", "user.name", "test"},
			{"/usr/bin/git", "config", "--list", "--show-origin"},
			{"/usr/bin/git", "--version"},
			{"/usr/bin/git", "add", "perf/FREEZELOCK"},
			{"/usr/bin/git", "commit", "-m", "Toggle freeze perf/FREEZELOCK"},
			{"/usr/bin/git", "push", "origin", "HEAD:refs/for/main%ready,notify=OWNER_REVIEWERS,l=Auto-Submit+1,r=rubber-stamper@appspot.gserviceaccount.com"},
		}, executedCommands)

		return nil
	})

	// Ensure that no step panicked or caused unexpected framework errors
	require.Empty(t, res.Exceptions)
}
