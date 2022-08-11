package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const (
	githubCIPDDEPSChildPath = "path/to/child"
	githubCIPDAssetName     = "test/cipd/name"
	githubCIPDAssetTag      = "latest"
	githubCIPDUser          = "aquaman@ocean.com"

	githubCIPDLastRolled = "xyz12345678901234567890"
	githubCipdNotRolled1 = "abc12345678901234567890"
	githubCipdNotRolled2 = "def12345678901234567890"
)

var (
	githubCIPDTs = cipd.UnixTime(time.Unix(1592417178, 0))
)

func githubCipdDEPSRmCfg(t *testing.T) *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGithubParent{
			DepsLocalGithubParent: &config.DEPSLocalGitHubParentConfig{
				DepsLocal: &config.DEPSLocalParentConfig{
					GitCheckout: &config.GitCheckoutParentConfig{
						GitCheckout: &config.GitCheckoutConfig{
							Branch:  git.MainBranch,
							RepoUrl: "todo.git",
						},
						Dep: &config.DependencyConfig{
							Primary: &config.VersionFileConfig{
								Id:   githubCIPDAssetName,
								Path: deps_parser.DepsFileName,
							},
						},
					},
					ChildPath: githubCIPDDEPSChildPath,
				},
				Github: &config.GitHubConfig{
					RepoOwner: githubCIPDUser,
					RepoName:  "todo.git",
				},
				ForkRepoUrl: "todo.git",
			},
		},
		Child: &config.ParentChildRepoManagerConfig_CipdChild{
			CipdChild: &config.CIPDChildConfig{
				Name: githubCIPDAssetName,
				Tag:  githubCIPDAssetTag,
			},
		},
	}
}

func setupGithubCipdDEPS(t *testing.T, cfg *config.ParentChildRepoManagerConfig) (context.Context, *parentChildRepoManager, string, *git_testutils.GitBuilder, *exec.CommandCollector, *mocks.CIPDClient, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	ctx := context.Background()

	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	// Under Bazel and RBE, there is no pre-existing depot_tools repository checkout for tests to use,
	// so depot_tools.Get() will try to clone the depot_tools repository. However, the delegate "run"
	// function of the exec.CommandCollector below skips any "git clone" commands. For this reason, we
	// clone said repository here before setting up the aforementioned delegate "run" function, and
	// make the checkout available to the caller test case via the corresponding environment variable.
	originalDepotToolsTestEnvVar := os.Getenv(depot_tools.DEPOT_TOOLS_TEST_ENV_VAR)
	if bazel.InBazelTestOnRBE() {
		depotToolsDir, err := depot_tools.Sync(ctx, filepath.Join(wd, "depot_tools"), recipesCfg)
		require.NoError(t, err)
		require.NoError(t, os.Setenv(depot_tools.DEPOT_TOOLS_TEST_ENV_VAR, depotToolsDir))
	}

	// Create parent repo.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "DEPS", fmt.Sprintf(`
deps = {
  "%s": {
    "packages": [
	  {
	    "package": "%s",
	    "version": "%s"
	  }
	],
  },
}`, githubCIPDDEPSChildPath, githubCIPDAssetName, githubCIPDLastRolled))
	parent.Commit(ctx)

	fork := git_testutils.GitInit(t, ctx)
	fork.Git(ctx, "remote", "set-url", git.DefaultRemote, parent.RepoUrl())
	fork.Git(ctx, "fetch", git.DefaultRemote)
	fork.Git(ctx, "checkout", git.MainBranch)
	fork.Git(ctx, "reset", "--hard", git.DefaultRemoteBranch)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") {
			if cmd.Args[0] == "clone" || cmd.Args[0] == "fetch" || cmd.Args[0] == "reset" {
				sklog.Infof("Skipping command: %s %s", cmd.Name, strings.Join(cmd.Args, " "))
				return nil
			}
			if cmd.Args[0] == "checkout" && cmd.Args[1] == "remote/"+git.MainBranch {
				// Pretend origin is the remote branch for testing ease.
				cmd.Args[1] = git.DefaultRemoteBranch
			}
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	g, urlMock := setupFakeGithub(ctx, t, nil)

	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl = parent.RepoUrl()
	parentCfg.ForkRepoUrl = fork.RepoUrl()
	rm, err := newParentChildRepoManager(ctx, cfg, setupRegistry(t), wd, "test_roller_name", recipesCfg, "fake.server.com", nil, githubCR(t, g))
	require.NoError(t, err)
	mockCipd := getCipdMock(ctx)
	rm.Child.(*child.CIPDChild).SetClientForTesting(mockCipd)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		require.NoError(t, os.Setenv(depot_tools.DEPOT_TOOLS_TEST_ENV_VAR, originalDepotToolsTestEnvVar))
		parent.Cleanup()
	}

	return ctx, rm, wd, parent, mockRun, mockCipd, urlMock, cleanup
}

type instanceEnumeratorImpl struct {
	done bool
}

func (e *instanceEnumeratorImpl) Next(ctx context.Context, limit int) ([]cipd.InstanceInfo, error) {
	if e.done {
		return nil, nil
	}
	instance0 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: githubCIPDAssetName,
			InstanceID:  githubCIPDLastRolled,
		},
		RegisteredBy: "aquaman@ocean.com",
	}
	instance1 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: githubCIPDAssetName,
			InstanceID:  githubCipdNotRolled1,
		},
		RegisteredBy: "superman@krypton.com",
	}
	instance2 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: githubCIPDAssetName,
			InstanceID:  githubCipdNotRolled2,
		},
		RegisteredBy: "batman@gotham.com",
	}
	e.done = true
	return []cipd.InstanceInfo{instance2, instance1, instance0}, nil
}

func cipdMockDescribe(ctx context.Context, cipdClient *mocks.CIPDClient, ver string, tags []string) {
	tagInfos := make([]cipd.TagInfo, len(tags))
	for idx, tag := range tags {
		tagInfos[idx].Tag = tag
	}
	cipdClient.On("Describe", ctx, githubCIPDAssetName, ver).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: githubCIPDAssetName,
				InstanceID:  ver,
			},
			RegisteredBy: githubCIPDUser,
			RegisteredTs: githubCIPDTs,
		},
		Tags: tagInfos,
	}, nil).Once()
}

func getCipdMock(ctx context.Context) *mocks.CIPDClient {
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: githubCIPDAssetName,
		InstanceID:  githubCipdNotRolled1,
	}
	cipdClient.On("ResolveVersion", ctx, githubCIPDAssetName, githubCIPDAssetTag).Return(head, nil).Once()
	cipdMockDescribe(ctx, cipdClient, githubCipdNotRolled1, nil)
	cipdMockDescribe(ctx, cipdClient, githubCipdNotRolled1, nil)
	cipdClient.On("ListInstances", ctx, githubCIPDAssetName).Return(&instanceEnumeratorImpl{}, nil).Once()
	cipdMockDescribe(ctx, cipdClient, githubCIPDLastRolled, nil)
	return cipdClient
}

// TestGithubRepoManager tests all aspects of the GithubRepoManager except for CreateNewRoll.
func TestGithubCipdDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubCipdDEPSRmCfg(t)
	ctx, rm, _, _, _, _, _, cleanup := setupGithubCipdDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Assert last roll, next roll and not rolled yet.
	require.Equal(t, githubCIPDLastRolled, lastRollRev.Id)
	require.Equal(t, githubCipdNotRolled1, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, githubCipdNotRolled1, notRolledRevs[0].Id)
	require.Equal(t, githubCipdNotRolled1[:17]+"...", notRolledRevs[0].Display)
}

func TestGithubCipdDEPSRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubCipdDEPSRmCfg(t)
	ctx, rm, _, _, _, _, urlMock, cleanup := setupGithubCipdDEPS(t, cfg)
	defer cleanup()
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll.
	mockGithubRequests(t, urlMock, cfg.GetDepsLocalGithubParent().ForkRepoUrl)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestGithubCipdDEPSRepoManagerPreUploadSteps(t *testing.T) {
	unittest.LargeTest(t)

	// Create a fake pre-upload step.
	ran := false
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string, *revision.Revision, *revision.Revision) error {
		ran = true
		return nil
	})
	cfg := githubCipdDEPSRmCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.PreUploadSteps = []config.PreUploadStep{stepName}

	ctx, rm, _, _, _, _, urlMock, cleanup := setupGithubCipdDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock, parentCfg.ForkRepoUrl)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, createErr)
	require.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestGithubCipdDEPSRepoManagerPreUploadStepsError(t *testing.T) {
	unittest.LargeTest(t)

	ran := false
	expectedErr := errors.New("Expected error")
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string, *revision.Revision, *revision.Revision) error {
		ran = true
		return expectedErr
	})
	cfg := githubCipdDEPSRmCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.PreUploadSteps = []config.PreUploadStep{stepName}

	ctx, rm, _, _, _, _, urlMock, cleanup := setupGithubCipdDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock, parentCfg.ForkRepoUrl)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}

// TestGithubRepoManagerGetRevision tests GithubCipdDEPSRepoManager.GetRevision().
func TestGithubCipdDEPSRepoManagerGetRevision(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubCipdDEPSRmCfg(t)
	ctx, rm, _, _, _, mockCipd, _, cleanup := setupGithubCipdDEPS(t, cfg)
	defer cleanup()

	// Clear out the mocks.
	_, _, _, err := rm.Update(ctx)
	require.NoError(t, err)

	// Basic.
	test := func(id string, tags []string, expect *revision.Revision) {
		cipdMockDescribe(ctx, mockCipd, id, tags)
		rev, err := rm.GetRevision(ctx, id)
		require.NoError(t, err)
		assertdeep.Equal(t, expect, rev)
	}

	getExpect := func(id string) *revision.Revision {
		return &revision.Revision{
			Id:          id,
			Author:      githubCIPDUser,
			Description: fmt.Sprintf("%s:%s", githubCIPDAssetName, id),
			Display:     id[:17] + "...",
			Timestamp:   time.Time(githubCIPDTs),
			URL:         fmt.Sprintf("https://chrome-infra-packages.appspot.com/p/%s/+/%s", githubCIPDAssetName, id),
		}
	}
	expect := getExpect(githubCipdNotRolled1)
	test(githubCipdNotRolled1, []string{"key:value"}, expect)

	// Bugs.
	expect = getExpect(githubCipdNotRolled2)
	expect.Bugs = map[string][]string{
		util.BUG_PROJECT_BUGANIZER: {"1234"},
		"chromium":                 {"456", "789"},
	}
	test(githubCipdNotRolled2, []string{"bug:b/1234", "bug:chromium:456", "bug:chromium:789"}, expect)

	// Details.
	expect = getExpect(githubCIPDLastRolled)
	expect.Details = `line 0
duplicates OK
line 1
 line 3
ordering doesnt matter`
	test(githubCIPDLastRolled, []string{
		"details4:ordering doesnt matter",
		"details0:line 0",
		"details1:line 1",
		"details3: line 3",
		"details1:duplicates OK",
	}, expect)
}
