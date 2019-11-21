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

	cipd_api "go.chromium.org/luci/cipd/client/cipd"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	GITHUB_CIPD_DEPS_CHILD_PATH = "path/to/child"
	GITHUB_CIPD_ASSET_NAME      = "test/cipd/name"
	GITHUB_CIPD_ASSET_TAG       = "latest"

	GITHUB_CIPD_LAST_ROLLED  = "xyz12345"
	GITHUB_CIPD_NOT_ROLLED_1 = "abc12345"
	GITHUB_CIPD_NOT_ROLLED_2 = "def12345"
)

func githubCipdDEPSRmCfg() *GithubCipdDEPSRepoManagerConfig {
	return &GithubCipdDEPSRepoManagerConfig{
		GithubDEPSRepoManagerConfig: GithubDEPSRepoManagerConfig{
			DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
				CommonRepoManagerConfig: CommonRepoManagerConfig{
					ChildBranch:  "master",
					ChildPath:    GITHUB_CIPD_DEPS_CHILD_PATH,
					ParentBranch: "master",
				},
			},
		},
		CipdAssetName: GITHUB_CIPD_ASSET_NAME,
		CipdAssetTag:  "latest",
	}
}

func setupGithubCipdDEPS(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	childPath := filepath.Join(wd, "github_repos", "earth")
	require.NoError(t, os.MkdirAll(childPath, 0755))

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`
deps = {
  "%s": {
    "packages": [
	  {
	    "package": "%s",
	    "version": "%s"
	  }
	],
  },
}`, GITHUB_CIPD_DEPS_CHILD_PATH, GITHUB_CIPD_ASSET_NAME, GITHUB_CIPD_LAST_ROLLED))
	parent.Commit(context.Background())

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") {
			if cmd.Args[0] == "clone" || cmd.Args[0] == "fetch" || cmd.Args[0] == "reset" {
				return nil
			}
			if cmd.Args[0] == "checkout" && cmd.Args[1] == "remote/master" {
				// Pretend origin is the remote branch for testing ease.
				cmd.Args[1] = "origin/master"
			}
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, wd, parent, mockRun, cleanup
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
			PackageName: GITHUB_CIPD_ASSET_NAME,
			InstanceID:  GITHUB_CIPD_LAST_ROLLED,
		},
		RegisteredBy: "aquaman@ocean.com",
	}
	instance1 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: GITHUB_CIPD_ASSET_NAME,
			InstanceID:  GITHUB_CIPD_NOT_ROLLED_1,
		},
		RegisteredBy: "superman@krypton.com",
	}
	instance2 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: GITHUB_CIPD_ASSET_NAME,
			InstanceID:  GITHUB_CIPD_NOT_ROLLED_2,
		},
		RegisteredBy: "batman@gotham.com",
	}
	e.done = true
	return []cipd.InstanceInfo{instance2, instance1, instance0}, nil
}

func cipdMockDescribe(ctx context.Context, cipdClient *mocks.CIPDClient, ver string) {
	cipdClient.On("Describe", ctx, GITHUB_CIPD_ASSET_NAME, ver).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: GITHUB_CIPD_ASSET_NAME,
				InstanceID:  ver,
			},
			RegisteredBy: "aquaman@ocean.com",
		},
	}, nil).Once()
}

func getCipdMock(ctx context.Context) *mocks.CIPDClient {
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: "test/cipd/name",
		InstanceID:  GITHUB_CIPD_NOT_ROLLED_1,
	}
	cipdClient.On("ResolveVersion", ctx, GITHUB_CIPD_ASSET_NAME, GITHUB_CIPD_ASSET_TAG).Return(head, nil).Once()
	cipdClient.On("ListInstances", ctx, GITHUB_CIPD_ASSET_NAME).Return(&instanceEnumeratorImpl{}, nil).Once()
	cipdMockDescribe(ctx, cipdClient, GITHUB_CIPD_LAST_ROLLED)
	return cipdClient
}

// TestGithubRepoManager tests all aspects of the GithubRepoManager except for CreateNewRoll.
func TestGithubCipdDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, _ := setupFakeGithub(t, nil)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	mockCipd := getCipdMock(ctx)
	rm.(*githubCipdDEPSRepoManager).CipdClient = mockCipd
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Assert last roll, next roll and not rolled yet.
	require.Equal(t, GITHUB_CIPD_LAST_ROLLED, lastRollRev.Id)
	require.Equal(t, GITHUB_CIPD_NOT_ROLLED_1, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, GITHUB_CIPD_NOT_ROLLED_1, notRolledRevs[0].Id)
	require.Equal(t, GITHUB_CIPD_NOT_ROLLED_1[:5]+"...", notRolledRevs[0].Display)
}

func TestCreateNewGithubCipdDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, nil)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	rm.(*githubCipdDEPSRepoManager).CipdClient = getCipdMock(ctx)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll.
	mockGithubRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsGithubCipdDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, nil)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	rm.(*githubCipdDEPSRepoManager).CipdClient = getCipdMock(ctx)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	ran := false
	rm.(*githubCipdDEPSRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, createErr)
	require.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestErrorPreUploadStepsGithubCipdDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, nil)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	rm.(*githubCipdDEPSRepoManager).CipdClient = getCipdMock(ctx)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	ran := false
	expectedErr := errors.New("Expected error")
	rm.(*githubCipdDEPSRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return expectedErr
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}
