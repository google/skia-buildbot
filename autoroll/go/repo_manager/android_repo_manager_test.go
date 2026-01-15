package repo_manager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	androidIssueNum        = int64(12345)
	numAndroidChildCommits = 10
)

var (
	androidEmails       = []string{"reviewer@chromium.org"}
	androidChildCommits = []string{
		"5678888888888888888888888888888888888888",
		"1234444444444444444444444444444444444444"}
)

func androidGerrit(t *testing.T, g gerrit.GerritInterface) (codereview.CodeReview, *mockhttpclient.URLMock) {
	urlmock := mockhttpclient.NewURLMock()
	rv, err := codereview.NewGerrit(&config.GerritConfig{
		Url:     "https://fake-android-review.googlesource.com",
		Project: "platform/external/skia",
		Config:  config.GerritConfig_ANDROID,
	}, g, urlmock.Client())
	require.NoError(t, err)
	return rv, urlmock
}

func androidCfg() *config.AndroidRepoManagerConfig {
	return &config.AndroidRepoManagerConfig{
		ChildBranch:   git.MainBranch,
		ChildPath:     childPath,
		ChildRepoUrl:  "https://fake.googlesource.com/skia",
		ParentBranch:  git.MainBranch,
		ParentRepoUrl: "https://my-repo.com",
		Metadata: &config.AndroidRepoManagerConfig_ProjectMetadataFileConfig{
			FilePath:    "METADATA",
			Name:        "skia",
			Description: "Skia Graphics Library",
			HomePage:    "https://www.skia.org/",
			GitUrl:      "https://fake.googlesource.com/skia",
			LicenseType: "RECIPROCAL",
		},
	}
}

func setupAndroid(t *testing.T) (context.Context, string, func()) {
	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	// We do not actually want to shell out to git, as that would require having an actual
	// git checkout. Instead, we intercept the calls to all binaries...
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if err := git_common.MocksForFindGit(ctx, cmd); err != nil {
			return err
		}
		// It's important that we use strings.HasSuffix, rather than strings.Contains, because
		// under Bazel the git binary will be found under the runfiles directory, and said path
		// might include the name of the target under test, such as "repo_manager_test", which
		// includes the word "repo" and would therefore break the below conditionals.
		if strings.HasSuffix(cmd.Name, "/repo") {
			require.Fail(t, "repo should always be called via python3") // See b/308174176.
		}
		if len(cmd.Args) >= 2 && strings.HasSuffix(cmd.Args[1], "/repo") && cmd.Name != "chmod" {
			// See b/308174176.
			require.Equal(t, cmd.Name, "python3", "repo should always be called via python3")
		}
		if strings.HasSuffix(cmd.Name, "/git") {
			var output string
			if cmd.Args[0] == "log" {
				if cmd.Args[1] == "--format=format:%H%x20%ci" {
					output = fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", androidChildCommits[0], androidChildCommits[1])
				}
				if cmd.Args[1] == "-n" && cmd.Args[2] == "1" {
					commit := ""
					for _, c := range androidChildCommits {
						if cmd.Args[len(cmd.Args)-1] == c {
							commit = c
							break
						}
					}
					require.NotEqual(t, "", commit)
					output = fmt.Sprintf("%s\nparent\nMe (me@google.com)\nsome commit\n1558543876\n", commit)
				}
			} else if cmd.Args[0] == "ls-remote" {
				output = androidChildCommits[0]
			} else if cmd.Args[0] == "merge-base" {
				output = androidChildCommits[1]
			} else if cmd.Args[0] == "rev-list" {
				split := strings.Split(cmd.Args[len(cmd.Args)-1], "..")
				require.Equal(t, 2, len(split))
				startCommit := split[0]
				endCommit := split[1]
				start, end := -1, -1
				for i := 0; i < len(androidChildCommits); i++ {
					if androidChildCommits[i] == startCommit {
						start = i
					}
					if androidChildCommits[i] == endCommit {
						end = i
					}
				}
				require.NotEqual(t, -1, start)
				require.NotEqual(t, -1, end)
				output = strings.Join(androidChildCommits[end:start], "\n")
			}
			n, err := cmd.CombinedOutput.Write([]byte(output))
			require.NoError(t, err)
			require.Equal(t, len(output), n)
		}
		return nil
	})
	// ... and use a fake git path instead of needing the git binary brought in from CIPD.
	fakeGitFinder := func() (string, error) {
		return "/fake/path/to/git", nil
	}
	ctx := git_common.WithGitFinder(context.Background(), fakeGitFinder)
	ctx = exec.NewContext(ctx, mockRun.Run)
	cleanup := func() {
		testutils.RemoveAll(t, wd)
	}
	return ctx, wd, cleanup
}

// TestAndroidRepoManager_Update tests AndroidRepoManager.Update.
func TestAndroidRepoManager_Update(t *testing.T) {
	ctx, wd, cleanup := setupAndroid(t)
	defer cleanup()
	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	mockGerrit, _ := androidGerrit(t, g)
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, _, err := rm.Update(ctx)
	require.NoError(t, err)

	require.Equal(t, fmt.Sprintf("%s/%s", wd, childPath), rm.childDir)
	require.Equal(t, androidChildCommits[len(androidChildCommits)-1], lastRollRev.Id)
	require.Equal(t, androidChildCommits[0], tipRev.Id)
}

// TestAndroidRepoManager_CreateNewRoll tests creating a new roll.
func TestAndroidRepoManager_CreateNewRoll(t *testing.T) {
	ctx, wd, cleanup := setupAndroid(t)
	defer cleanup()

	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	g.On("Search", testutils.AnyContext, 1, false, gerrit.SearchCommit("")).Return([]*gerrit.ChangeInfo{{Issue: androidIssueNum}}, nil)
	g.On("GetIssueProperties", testutils.AnyContext, androidIssueNum).Return(&gerrit.ChangeInfo{Issue: androidIssueNum}, nil)
	g.On("SetTopic", testutils.AnyContext, "child_merge_12345", androidIssueNum).Return(nil)
	g.On("SetReview", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockGerrit, _ := androidGerrit(t, g)
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, androidEmails, false, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// TestAndroidRepoManager_CreateNewRollWithExternalChangeId tests creating a new
// roll with an external change ID specified in the target revision.
func TestAndroidRepoManager_CreateNewRollWithExternalChangeId(t *testing.T) {
	testTopicName := "test_topic_name"
	ctx, wd, cleanup := setupAndroid(t)
	defer cleanup()

	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	g.On("Search", testutils.AnyContext, 1, false, gerrit.SearchCommit("")).Return([]*gerrit.ChangeInfo{{Issue: androidIssueNum}}, nil)
	g.On("GetIssueProperties", testutils.AnyContext, androidIssueNum).Return(&gerrit.ChangeInfo{Issue: androidIssueNum}, nil)
	g.On("SetTopic", testutils.AnyContext, testTopicName, androidIssueNum).Return(nil)
	g.On("SetReview", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockGerrit, _ := androidGerrit(t, g)
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Add ExternalChangeId to the revision.
	tipRev.ExternalChangeId = testTopicName

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, androidEmails, false, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

func TestAndroidRepoManager_ConfigValidation(t *testing.T) {
	cfg := androidCfg()
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &config.AndroidRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}

func TestAndroidRepoManager_GetNotSubmittedReason(t *testing.T) {
	ctx, wd, cleanup := setupAndroid(t)
	defer cleanup()
	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	mockGerrit, _ := androidGerrit(t, g)
	urlMock := mockhttpclient.NewURLMock()
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), wd, "fake.server.com", "fake-service-account", urlMock.Client(), mockGerrit, true, true)
	require.NoError(t, err)

	// The revision was submitted.
	fakeRev := &revision.Revision{Id: androidChildCommits[0]}
	gitiles_testutils.MockGetCommit(t, urlMock, rm.childRepoURL, rm.childBranch, &gitiles.Commit{
		Commit: androidChildCommits[0],
		Author: &gitiles.Author{
			Time: time.Now().Format(gitiles.DateFormatNoTZ),
		},
		Committer: &gitiles.Author{
			Time: time.Now().Format(gitiles.DateFormatNoTZ),
		},
	})
	notSubmittedReason, err := rm.GetNotSubmittedReason(ctx, fakeRev)
	require.NoError(t, err)
	require.Empty(t, notSubmittedReason)

	// The revision is not submitted.
	fakeRev.Id = "bogus"
	notSubmittedReason, err = rm.GetNotSubmittedReason(ctx, fakeRev)
	require.NoError(t, err)
	require.NotEmpty(t, notSubmittedReason)
}
