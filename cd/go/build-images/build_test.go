package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBuild_SingleTarget_OutputJSONFileCreated(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		extraArgs := []string{"--config=extra"}
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		assert.NoError(t, err)
		if err != nil {
			return err
		}

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "--config=extra", "//skfe:skfe_container"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:2023-07-01T02_03_04Z-louhi-aabbccd-clean"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:git-aabbccddeeff00112233445566778899aabbccdd"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:latest"},
		}, executedCommands)

		contents, err := os.ReadFile(filepath.Join(workspace, "build-images.json"))
		require.NoError(t, err)
		assert.Equal(t, `{"images":[{"image":"gcr.io/skia-public/envoy_skia_org","tag":"2023-07-01T02_03_04Z-louhi-aabbccd-clean"}]}
`, string(contents))

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_SingleTarget_InvalidTargetCausesFailure(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "This is an invalid target"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid target")

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
		}, executedCommands)

		assertFileDoesNotExist(t, filepath.Join(workspace, "build-images.json"))
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_SingleTarget_GitFetchErrorCausesFailure(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx, gitMatcher(func(cmd *exec.Command) error {
			if len(cmd.Args) > 0 && cmd.Args[0] == "fetch" {
				return errors.New("Host unreachable")
			}
			return nil
		}))
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Host unreachable")

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
		}, executedCommands)

		assertFileDoesNotExist(t, filepath.Join(workspace, "build-images.json"))
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_SingleTarget_DockerErrorCausesFailure(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx, func(cmd *exec.Command) error {
			if cmd.Name == "docker" {
				return errors.New("fail whale")
			}
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fail whale")

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "//skfe:skfe_container"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:2023-07-01T02_03_04Z-louhi-aabbccd-clean"},
		}, executedCommands)

		assertFileDoesNotExist(t, filepath.Join(workspace, "build-images.json"))
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_SingleTarget_BazelErrorCausesFailure(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx, func(cmd *exec.Command) error {
			if cmd.Name == "bazelisk" {
				return errors.New("A mirror can reflect thy fatal glare")
			}
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reflect thy")

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "//skfe:skfe_container"},
		}, executedCommands)

		assertFileDoesNotExist(t, filepath.Join(workspace, "build-images.json"))
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_MultipleTarget_OutputJSONFileCreated(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 7, 8, 9, 10, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 7, 8, 19, 1, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "00112233445566778899aaabbbcccdddeeefff00"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target1 = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		const target2 = "//prober:proberk_container:gcr.io/skia-public/proberk"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target1, target2}, extraArgs)
		assert.NoError(t, err)
		if err != nil {
			return err
		}

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "//skfe:skfe_container"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:2023-07-07T08_09_10Z-louhi-0011223-clean"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:git-00112233445566778899aaabbbcccdddeeefff00"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:latest"},

			{"bazelisk", "run", "//prober:proberk_container"},
			{"docker", "tag", "gcr.io/skia-public/proberk:latest",
				"louhi_ws/gcr.io/skia-public/proberk:2023-07-07T08_09_10Z-louhi-0011223-clean"},
			{"docker", "tag", "gcr.io/skia-public/proberk:latest",
				"louhi_ws/gcr.io/skia-public/proberk:git-00112233445566778899aaabbbcccdddeeefff00"},
			{"docker", "tag", "gcr.io/skia-public/proberk:latest",
				"louhi_ws/gcr.io/skia-public/proberk:latest"},
		}, executedCommands)

		contents, err := os.ReadFile(filepath.Join(workspace, "build-images.json"))
		require.NoError(t, err)
		assert.Equal(t, `{"images":[{"image":"gcr.io/skia-public/envoy_skia_org","tag":"2023-07-07T08_09_10Z-louhi-0011223-clean"},{"image":"gcr.io/skia-public/proberk","tag":"2023-07-07T08_09_10Z-louhi-0011223-clean"}]}
`, string(contents))

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBuild_SingleTargetMultipleTimes_Deduplicated(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		var extraArgs []string
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target, target, target}, extraArgs)
		assert.NoError(t, err)
		if err != nil {
			return err
		}

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "//skfe:skfe_container"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:2023-07-01T02_03_04Z-louhi-aabbccd-clean"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:git-aabbccddeeff00112233445566778899aabbccdd"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:latest"},
		}, executedCommands)

		contents, err := os.ReadFile(filepath.Join(workspace, "build-images.json"))
		require.NoError(t, err)
		assert.Equal(t, `{"images":[{"image":"gcr.io/skia-public/envoy_skia_org","tag":"2023-07-01T02_03_04Z-louhi-aabbccd-clean"}]}
`, string(contents))

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func assertFileDoesNotExist(t *testing.T, f string) {
	_, err := os.Stat(f)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestBuild_MultipleExtraArgs_PreservedInOrder(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Use a fixed, arbitrary time for one of the Docker tags
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))
		mock, ctx := commandCollectorWithStubbedGit(ctx)
		ctx = td.WithExecRunFn(ctx, mock.Run)

		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const gitCommit = "aabbccddeeff00112233445566778899aabbccdd"
		workspace := t.TempDir()
		const username = "louhi"
		const email = "louhi-service-account@example.com"
		const target = "//skfe:skfe_container:gcr.io/skia-public/envoy_skia_org"
		extraArgs := []string{"--config=one", "--config=two", "--verbose"}
		err := build(ctx, gitCommit, gitRepo, workspace, username, email, []string{target}, extraArgs)
		assert.NoError(t, err)
		if err != nil {
			return err
		}

		executedCommands := mock.Commands()
		testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", gitCommit},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{"bazelisk", "run", "--config=one", "--config=two", "--verbose", "//skfe:skfe_container"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:2023-07-01T02_03_04Z-louhi-aabbccd-clean"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:git-aabbccddeeff00112233445566778899aabbccdd"},
			{"docker", "tag", "gcr.io/skia-public/envoy_skia_org:latest",
				"louhi_ws/gcr.io/skia-public/envoy_skia_org:latest"},
		}, executedCommands)

		contents, err := os.ReadFile(filepath.Join(workspace, "build-images.json"))
		require.NoError(t, err)
		assert.Equal(t, `{"images":[{"image":"gcr.io/skia-public/envoy_skia_org","tag":"2023-07-01T02_03_04Z-louhi-aabbccd-clean"}]}
`, string(contents))

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}
