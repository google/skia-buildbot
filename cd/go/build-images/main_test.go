package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/task_driver/go/td"
)

func TestShallowClone_InputsThreadedThrough(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		dir, err := shallowClone(ctx, gitRepo, gitCommit)
		assert.NoError(t, err)
		if err != nil {
			return err
		}

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
		}, executedCommands)

		// dir should be a temporary directory somewhere.
		assert.NotEmpty(t, dir)
		// Make sure dir is the working directory for all non-version git commands.
		for i := 1; i < len(executedCommands); i++ {
			assert.Equal(t, dir, executedCommands[1].Dir, "command index %d", i)
		}

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestShallowClone_FetchFailsCausesError(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mock, ctx := commandCollectorWithStubbedGit(ctx, gitMatcher(func(command *exec.Command) error {
			if len(command.Args) > 0 && command.Args[0] == "fetch" {
				return errors.New("Stop trying to make fetch happen")
			}
			return nil
		}))
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		_, err := shallowClone(ctx, gitRepo, gitCommit)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fetch happen")

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
		}, executedCommands)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

const fakeGitPath = "/path/to/fake/git"

type commandMatcher func(*exec.Command) error

// gitMatcher is a helper to create a commandMatcher that only listens to calls to the stubbed git.
func gitMatcher(f commandMatcher) commandMatcher {
	return func(cmd *exec.Command) error {
		if cmd.Name != fakeGitPath {
			return nil
		}
		return f(cmd)
	}
}

// commandCollectorWithStubbedGit returns an exec.CommandCollector and a context.Context configured
// to run the former instead of actually shelling out calls to our go/exec.Run*. If
// git_common.FindGit() is called with the returned function, a fake path and --version will be
// returned. If tests want to intercept other calls, they should add one or more extraMatchers.
func commandCollectorWithStubbedGit(ctx context.Context, extraMatchers ...commandMatcher) (*exec.CommandCollector, context.Context) {
	mock := exec.CommandCollector{}

	gitFinder := func() (string, error) {
		return fakeGitPath, nil
	}
	ctx = git_common.WithGitFinder(ctx, gitFinder)
	extraMatchers = append(extraMatchers, gitMatcher(func(cmd *exec.Command) error {
		if len(cmd.Args) == 1 && cmd.Args[0] == "--version" {
			_, _ = cmd.CombinedOutput.Write([]byte("git version 3.14.15.92653"))
			return nil
		}
		if len(cmd.Args) > 1 && cmd.Args[0] == "config" {
			_, _ = cmd.CombinedOutput.Write([]byte(`"stubbed call to git config (e.g. git_auth.New)"`))
			return nil
		}
		return nil
	}))
	mock.SetDelegateRun(func(_ context.Context, command *exec.Command) error {
		for _, matcher := range extraMatchers {
			err := matcher(command)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return &mock, ctx
}
