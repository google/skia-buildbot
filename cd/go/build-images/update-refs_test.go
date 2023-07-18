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
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/docker"
	mock_docker "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/exec"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBazelRegexAndReplaceForImage(t *testing.T) {
	const fileContents = `# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:0ae30b768fb1bdcbea5b6721075b758806c4076a74a8a99a67ff3632df87cf5a",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:17e18164238a4162ce2c30b7328a7e44fbe569e56cab212ada424dc7378c1f5f",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)`
	const expectedContents = `# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:0ae30b768fb1bdcbea5b6721075b758806c4076a74a8a99a67ff3632df87cf5a",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:f5f1c8737cd424ada212bac65e965ebf44e7a8237b03c2ec2614a83246181e71",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)`
	image := &SingleImageInfo{
		Image: "gcr.io/skia-public/cd-base",
		Tag:   "unused",
	}
	regex, replace := bazelRegexAndReplaceForImage(image, "sha256:f5f1c8737cd424ada212bac65e965ebf44e7a8237b03c2ec2614a83246181e71")
	updatedContents := regex.ReplaceAllString(fileContents, replace)
	require.Equal(t, expectedContents, updatedContents)
}

const fakeBuildImageJSON = `{"images":[{"image":"gcr.io/skia-public/envoy_skia_org","tag":"2023-07-01T02_03_04Z-louhi-aabbccd-clean"}]}
`

const fakeDockerfileContents = `
FROM gcr.io/skia-public/envoy_skia_org@sha256:04ec75f15a12ae03ef1436fcd67b8bb918fb6c1e577b12dfd25a501a83c9074d

FROM gcr.io/skia-public/debugger-app-base@sha256:be5f915d20737800528468b421259c283a88db263b6a2e83c200e91d93cf02cd
`

const fakeWorkspaceContents = `
# This is a comment
container_pull(
    name = "envoy_skia_org",
    digest = "sha256:04ec75f15a12ae03ef1436fcd67b8bb918fb6c1e577b12dfd25a501a83c9074d",
    registry = "gcr.io",
    repository = "skia-public/envoy_skia_org",
)

container_pull(
    name = "debugger-app-base",
    digest = "sha256:6820bee4d8f062bfac1a370fa66ea83e8ad67443f603f843c62367ab486b1506",
    registry = "gcr.io",
    repository = "skia-public/debugger-app-base",
)
`

// useFakeCheckout creates the following file system under the directory that git checkout is run:
//
//	nested/Dockerfile
//	WORKSPACE.bazel
//
// The files contain realistic data that may be changed by tests.
func useFakeCheckout(t *testing.T) commandMatcher {
	return gitMatcher(func(cmd *exec.Command) error {
		if len(cmd.Args) == 2 && cmd.Args[0] == "checkout" { // git checkout FETCH_HEAD
			w := filepath.Join(cmd.Dir, "nested")
			require.NoError(t, os.MkdirAll(w, 0777))
			// Make the permissions different to make sure they are preserved
			require.NoError(t, os.WriteFile(filepath.Join(w, "Dockerfile"), []byte(fakeDockerfileContents), 0744))
			require.NoError(t, os.WriteFile(filepath.Join(cmd.Dir, "WORKSPACE.bazel"), []byte(fakeWorkspaceContents), 0644))
			return nil
		}
		return nil
	})
}

func gitHasDiffs() commandMatcher {
	return gitMatcher(func(cmd *exec.Command) error {
		if len(cmd.Args) == 3 && cmd.Args[0] == "diff" { // git diff HEAD --exit-code
			return errors.New("This is an arbitrary error to signal git detected diffs")
		}
		return nil
	})
}

func TestUpdateRefs_NoStageFileNoGitilesNoPubsub_ReplacementsMadeCLUploaded(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const email = "louhi-service-account@example.com"
		const newDockerHash = "sha256:0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff"
		const fakeChangeID = "Change-Id: Ib5e0a9a6f10910d8514b800252f106edd314dec3"
		workspace := t.TempDir()
		// Mock git such that we actually create some files on disk (needed for find and replace)
		// and then have git indicate there were "diffs".
		var gitCheckoutDir string
		mockExec, ctx := commandCollectorWithStubbedGit(ctx, useFakeCheckout(t), gitHasDiffs(), gitMatcher(func(cmd *exec.Command) error {
			if len(cmd.Args) > 0 && cmd.Args[0] == "checkout" {
				gitCheckoutDir = cmd.Dir // capture the temporary directory made to clone the git repo.
			}
			return nil
		}))
		ctx = td.WithExecRunFn(ctx, mockExec.Run)
		ctx = rubberstamper.WithDeterministicChangeID(ctx, fakeChangeID)
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))

		mDocker := mock_docker.NewClient(t)
		mDocker.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/envoy_skia_org", "2023-07-01T02_03_04Z-louhi-aabbccd-clean").
			Return(&docker.Manifest{
				Digest: newDockerHash,
				// other fields not used
			}, nil)

		require.NoError(t, os.WriteFile(filepath.Join(workspace, "build-images.json"), []byte(fakeBuildImageJSON), 0666))

		err := updateRefs(ctx, mDocker, gitRepo, workspace, email, "", "", "", "")
		assert.NoError(t, err)

		executedCommands := mockExec.Commands()
		exec_testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", "refs/heads/main"},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "checkout", "-b", "update", "-t", "origin/main"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "diff", "HEAD", "--exit-code"},
			{fakeGitPath, "commit", "-a", "-m", `Update envoy_skia_org

Change-Id: Ib5e0a9a6f10910d8514b800252f106edd314dec3`}, // fakeChangeID
			{fakeGitPath, "push", "origin", "HEAD:refs/for/main%notify=OWNER_REVIEWERS,l=Auto-Submit+1,r=rubber-stamper@appspot.gserviceaccount.com"},
		}, executedCommands)

		require.NotEmpty(t, gitCheckoutDir)
		// filemodes should match what they were created with in useFakeCheckout
		assertFileMatches(t, filepath.Join(gitCheckoutDir, "nested", "Dockerfile"), 0744, `
FROM gcr.io/skia-public/envoy_skia_org@sha256:0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff

FROM gcr.io/skia-public/debugger-app-base@sha256:be5f915d20737800528468b421259c283a88db263b6a2e83c200e91d93cf02cd
`)
		assertFileMatches(t, filepath.Join(gitCheckoutDir, "WORKSPACE.bazel"), 0644, `
# This is a comment
container_pull(
    name = "envoy_skia_org",
    digest = "sha256:0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff",
    registry = "gcr.io",
    repository = "skia-public/envoy_skia_org",
)

container_pull(
    name = "debugger-app-base",
    digest = "sha256:6820bee4d8f062bfac1a370fa66ea83e8ad67443f603f843c62367ab486b1506",
    registry = "gcr.io",
    repository = "skia-public/debugger-app-base",
)
`)

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestUpdateRefs_NoDiffs_NoCLUploaded(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Arbitrary data that closely mirrors reality
		const gitRepo = "https://skia.googlesource.com/buildbot.git"
		const email = "louhi-service-account@example.com"
		const existingDockerHash = "sha256:04ec75f15a12ae03ef1436fcd67b8bb918fb6c1e577b12dfd25a501a83c9074d"
		workspace := t.TempDir()
		mockExec, ctx := commandCollectorWithStubbedGit(ctx, useFakeCheckout(t))
		ctx = td.WithExecRunFn(ctx, mockExec.Run)
		ctx = context.WithValue(ctx, now.ContextKey, time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC))
		ctx = auth_steps.WithTokenSource(ctx, FakeTokenSource(time.Date(2023, time.July, 1, 2, 33, 4, 0, time.UTC)))

		mDocker := mock_docker.NewClient(t)
		mDocker.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/envoy_skia_org", "2023-07-01T02_03_04Z-louhi-aabbccd-clean").
			Return(&docker.Manifest{
				Digest: existingDockerHash,
				// other fields not used
			}, nil)

		require.NoError(t, os.WriteFile(filepath.Join(workspace, "build-images.json"), []byte(fakeBuildImageJSON), 0666))

		err := updateRefs(ctx, mDocker, gitRepo, workspace, email, "", "", "", "")
		assert.NoError(t, err)

		executedCommands := mockExec.Commands()
		exec_testutils.AssertCommandsMatch(t, [][]string{
			{fakeGitPath, "--version"},
			{fakeGitPath, "config", "--global", "http.cookiefile", "/tmp/.gitcookies"},
			{fakeGitPath, "config", "--global", "user.email", email},
			{fakeGitPath, "config", "--global", "user.name", "louhi-service-account"},
			{fakeGitPath, "config", "--list", "--show-origin"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "init"},
			{fakeGitPath, "remote", "add", "origin", gitRepo},
			{fakeGitPath, "fetch", "--depth=1", "origin", "refs/heads/main"},
			{fakeGitPath, "checkout", "FETCH_HEAD"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "checkout", "-b", "update", "-t", "origin/main"},
			{fakeGitPath, "--version"},
			{fakeGitPath, "diff", "HEAD", "--exit-code"},
		}, executedCommands)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func assertFileMatches(t *testing.T, fpath string, expectedMode os.FileMode, expectedContents string) {
	stat, err := os.Stat(fpath)
	require.NoError(t, err)
	assert.Equalf(t, expectedMode, stat.Mode(), "%o != %o", expectedMode, stat.Mode())
	b, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, expectedContents, string(b))
}

type fakeTokenSource struct {
	expires time.Time
}

func (n *fakeTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken:  "fake_access_token",
		TokenType:    "fake_token_type",
		RefreshToken: "fake_refresh_token",
		Expiry:       n.expires,
	}, nil
}

func FakeTokenSource(expires time.Time) oauth2.TokenSource {
	return &fakeTokenSource{
		expires: expires,
	}
}
