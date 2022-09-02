package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	androidIssueNum        = int64(12345)
	numAndroidChildCommits = 10
)

var (
	androidEmails = []string{"reviewer@chromium.org"}
	childCommits  = []string{
		"5678888888888888888888888888888888888888",
		"1234444444444444444444444444444444444444"}
)

func androidGerrit(t *testing.T, g gerrit.GerritInterface) (codereview.CodeReview, *mockhttpclient.URLMock) {
	urlmock := mockhttpclient.NewURLMock()
	rv, err := codereview.NewGerrit(&config.GerritConfig{
		Url:     "https://googleplex-android-review.googlesource.com",
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
		ChildRepoUrl:  common.REPO_SKIA,
		ParentBranch:  git.MainBranch,
		ParentRepoUrl: "https://my-repo.com",
		Metadata: &config.AndroidRepoManagerConfig_ProjectMetadataFileConfig{
			FilePath:    "METADATA",
			Name:        "skia",
			Description: "Skia Graphics Library",
			HomePage:    "https://www.skia.org/",
			GitUrl:      "https://skia.googlesource.com/skia",
			LicenseType: "RECIPROCAL",
		},
	}
}

func setupAndroid(t *testing.T) (context.Context, *config_vars.Registry, string, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
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
			return nil
		}
		if strings.HasSuffix(cmd.Name, "/git") {
			var output string
			if cmd.Args[0] == "log" {
				if cmd.Args[1] == "--format=format:%H%x20%ci" {
					output = fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", childCommits[0], childCommits[1])
				}
				if cmd.Args[1] == "-n" && cmd.Args[2] == "1" {
					commit := ""
					for _, c := range childCommits {
						if cmd.Args[len(cmd.Args)-1] == c {
							commit = c
							break
						}
					}
					require.NotEqual(t, "", commit)
					output = fmt.Sprintf("%s\nparent\nMe (me@google.com)\nsome commit\n1558543876\n", commit)
				}
			} else if cmd.Args[0] == "ls-remote" {
				output = childCommits[0]
			} else if cmd.Args[0] == "merge-base" {
				output = childCommits[1]
			} else if cmd.Args[0] == "rev-list" {
				split := strings.Split(cmd.Args[len(cmd.Args)-1], "..")
				require.Equal(t, 2, len(split))
				startCommit := split[0]
				endCommit := split[1]
				start, end := -1, -1
				for i := 0; i < len(childCommits); i++ {
					if childCommits[i] == startCommit {
						start = i
					}
					if childCommits[i] == endCommit {
						end = i
					}
				}
				require.NotEqual(t, -1, start)
				require.NotEqual(t, -1, end)
				output = strings.Join(childCommits[end:start], "\n")
			}
			n, err := cmd.CombinedOutput.Write([]byte(output))
			require.NoError(t, err)
			require.Equal(t, len(output), n)
		}
		return nil
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	cleanup := func() {
		testutils.RemoveAll(t, wd)
	}
	return ctx, setupRegistry(t), wd, cleanup
}

// TestAndroidRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestAndroidRepoManager(t *testing.T) {
	ctx, reg, wd, cleanup := setupAndroid(t)
	defer cleanup()
	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	mockGerrit, _ := androidGerrit(t, g)
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), reg, wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, _, err := rm.Update(ctx)
	require.NoError(t, err)

	require.Equal(t, fmt.Sprintf("%s/%s", wd, childPath), rm.(*androidRepoManager).childDir)
	require.Equal(t, childCommits[len(childCommits)-1], lastRollRev.Id)
	require.Equal(t, childCommits[0], tipRev.Id)
}

// TestCreateNewAndroidRoll tests creating a new roll.
func TestCreateNewAndroidRoll(t *testing.T) {
	ctx, reg, wd, cleanup := setupAndroid(t)
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
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), reg, wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, androidEmails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// TestCreateNewAndroidRollWithExternalChangeId tests creating a new roll
// with an external change ID specified in the target revision.
func TestCreateNewAndroidRollWithExternalChangeId(t *testing.T) {
	testTopicName := "test_topic_name"
	ctx, reg, wd, cleanup := setupAndroid(t)
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
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), reg, wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Add ExternalChangeId to the revision.
	tipRev.ExternalChangeId = testTopicName

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, androidEmails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsAndroid(t *testing.T) {
	ctx, reg, wd, cleanup := setupAndroid(t)
	defer cleanup()

	g := &mocks.GerritInterface{}
	g.On("GetUserEmail", testutils.AnyContext).Return("fake-service-account", nil)
	g.On("GetRepoUrl").Return(androidCfg().ParentRepoUrl)
	g.On("Config").Return(gerrit.ConfigAndroid)
	g.On("Search", testutils.AnyContext, 1, false, gerrit.SearchCommit("")).Return([]*gerrit.ChangeInfo{{Issue: androidIssueNum}}, nil)
	g.On("GetIssueProperties", testutils.AnyContext, androidIssueNum).Return(&gerrit.ChangeInfo{Issue: androidIssueNum}, nil)
	g.On("SetTopic", testutils.AnyContext, mock.AnythingOfType("string"), androidIssueNum).Return(nil)
	g.On("SetReview", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockGerrit, _ := androidGerrit(t, g)
	rm, err := NewAndroidRepoManager(ctx, androidCfg(), reg, wd, "fake.server.com", "fake-service-account", nil, mockGerrit, true, true)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	ran := false
	rm.(*androidRepoManager).preUploadSteps = []parent.PreUploadStep{
		func(context.Context, []string, *http.Client, string, *revision.Revision, *revision.Revision) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, androidEmails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.True(t, ran)
}

func TestAndroidConfigValidation(t *testing.T) {

	cfg := androidCfg()
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &config.AndroidRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}
