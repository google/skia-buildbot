package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	childPath       = "path/to/child"
	cqExtraTrybots  = ""
	issueNum        = int64(12345)
	mockServer      = "https://skia-review.googlesource.com"
	mockUser        = "user@chromium.org"
	numChildCommits = 10
)

var (
	emails = []string{"reviewer@chromium.org"}
)

func depsCfg(t *testing.T) *DEPSRepoManagerConfig {
	return &DEPSRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  masterBranchTmpl(t),
				ChildPath:    childPath,
				ParentBranch: masterBranchTmpl(t),
			},
		},
		Gerrit: &codereview.GerritConfig{
			URL:     "https://fake-skia-review.googlesource.com",
			Project: "fake-gerrit-project",
			Config:  codereview.GERRIT_CONFIG_CHROMIUM,
		},
	}
}

func setupDEPSRepoManager(t *testing.T) (context.Context, *config_vars.Registry, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *vcsinfo.LongCommit, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	ctx := context.Background()
	child := git_testutils.GitInit(t, ctx)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(ctx, f))
	}

	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Add(ctx, ".gitignore", fmt.Sprintf(`
.gclient
.gclient_entries
%s
`, childPath))
	parent.Commit(ctx)

	lastUpload := new(vcsinfo.LongCommit)
	mockRun := &exec.CommandCollector{}
	ctx = exec.NewContext(ctx, mockRun.Run)
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
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

	urlmock := setupFakeGerrit(t, wd)
	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, setupRegistry(t), wd, child, childCommits, parent, mockRun, lastUpload, urlmock, cleanup
}

func setupFakeGerrit(t *testing.T, wd string) *mockhttpclient.URLMock {
	urlMock := mockhttpclient.NewURLMock()

	// Create a dummy commit-msg hook.
	changeId := "123"
	respBody := []byte(fmt.Sprintf(`#!/bin/sh
git interpret-trailers --trailer "Change-Id: %s" >> $1
`, changeId))
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/tools/hooks/commit-msg", mockhttpclient.MockGetDialogue(respBody))

	return urlMock
}

// TestRepoManager tests all aspects of the DEPSRepoManager except for CreateNewRoll.
func TestDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, childRepo, childCommits, parentRepo, _, _, urlmock, cleanup := setupDEPSRepoManager(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	cfg := depsCfg(t)
	cfg.ChildRepo = childRepo.RepoUrl()
	cfg.ParentRepo = parentRepo.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
	require.NoError(t, err)

	// Test update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))
}

func mockGerritGetAndPublishChange(t *testing.T, urlmock *mockhttpclient.URLMock, cfg *DEPSRepoManagerConfig) {
	// Mock the request to load the change.
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
		},
		WorkInProgress: true,
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody := []byte(`{}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	gerritCfg := codereview.GERRIT_CONFIGS[cfg.Gerrit.Config]
	if gerritCfg.HasCq {
		reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	} else {
		reqBody = []byte(`{"labels":{"Code-Review":1},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	}
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))
	if !gerritCfg.HasCq {
		urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/submit", mockhttpclient.MockPostDialogue("application/json", []byte("{}"), []byte("")))
	}
}

func TestCreateNewDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, childRepo, _, parentRepo, _, _, urlmock, cleanup := setupDEPSRepoManager(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	cfg := depsCfg(t)
	cfg.ChildRepo = childRepo.RepoUrl()
	cfg.ParentRepo = parentRepo.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, int64(123), issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsDeps(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, childRepo, _, parentRepo, _, _, urlmock, cleanup := setupDEPSRepoManager(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	// Create a dummy pre-upload step.
	ran := false
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string) error {
		ran = true
		return nil
	})

	cfg := depsCfg(t)
	cfg.ChildRepo = childRepo.RepoUrl()
	cfg.ParentRepo = parentRepo.RepoUrl()
	cfg.PreUploadSteps = []string{stepName}
	rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.True(t, ran)
}

// Verify that we respect the includeLog parameter.
func TestDEPSRepoManagerIncludeLog(t *testing.T) {
	unittest.LargeTest(t)

	test := func(includeLog bool) {
		ctx, reg, wd, childRepo, _, parentRepo, _, lastUpload, urlmock, cleanup := setupDEPSRepoManager(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		cfg := depsCfg(t)
		cfg.ChildRepo = childRepo.RepoUrl()
		cfg.ParentRepo = parentRepo.RepoUrl()
		cfg.IncludeLog = includeLog
		rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
		require.NoError(t, err)
		lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
		require.NoError(t, err)

		// Mock the request to load the change.
		mockGerritGetAndPublishChange(t, urlmock, cfg)

		// Create a roll.
		_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
		require.NoError(t, err)

		// Ensure that we included the log, or not, as appropriate.
		require.NoError(t, err)
		require.Equal(t, includeLog, strings.Contains(lastUpload.Body, "git log"))
	}

	test(true)
	test(false)
}

// Verify that we properly utilize a gclient spec.
func TestDEPSRepoManagerGClientSpec(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, childRepo, _, parentRepo, mockRun, _, urlmock, cleanup := setupDEPSRepoManager(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	gclientSpec := fmt.Sprintf(`
solutions=[{
  "name": "alternate/location/%s",
  "url": "%s",
  "deps_file": "DEPS",
  "managed": False,
  "custom_deps": {},
  "custom_vars": {
    "a": "b",
    "c": "d",
  },
}];
cache_dir=None
`, path.Base(parentRepo.RepoUrl()), parentRepo.RepoUrl())
	// Remove newlines.
	gclientSpec = strings.Replace(gclientSpec, "\n", "", -1)
	cfg := depsCfg(t)
	cfg.ChildRepo = childRepo.RepoUrl()
	cfg.GClientSpec = gclientSpec
	cfg.ParentPath = filepath.Join("alternate", "location", filepath.Base(parentRepo.RepoUrl()))
	cfg.ParentRepo = parentRepo.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)

	// Ensure that we pass the spec into "gclient config".
	found := false
	for _, c := range mockRun.Commands() {
		if c.Name == "python" && strings.Contains(c.Args[0], "gclient.py") && c.Args[1] == "config" {
			for _, arg := range c.Args {
				if arg == "--spec="+gclientSpec {
					found = true
				}
			}
		}
	}
	require.True(t, found)
}

// Verify that we include the correct bug lings.
func TestDEPSRepoManagerBugs(t *testing.T) {
	unittest.LargeTest(t)

	project := "skiatestproject"

	test := func(bugLine, expect string) {
		// Setup.
		ctx, reg, wd, childRepo, _, parentRepo, _, lastUpload, urlmock, cleanup := setupDEPSRepoManager(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		cfg := depsCfg(t)
		cfg.ChildRepo = childRepo.RepoUrl()
		cfg.IncludeBugs = true
		cfg.BugProject = project
		cfg.ParentRepo = parentRepo.RepoUrl()
		rm, err := NewDEPSRepoManager(ctx, cfg, reg, wd, nil, recipesCfg, "fake.server.com", urlmock.Client(), nil, false)
		require.NoError(t, err)

		// Initial update.
		_, _, _, err = rm.Update(ctx)
		require.NoError(t, err)

		// Insert a fake entry into the repo mapping.
		issues.REPO_PROJECT_MAPPING[parentRepo.RepoUrl()] = project

		// Make a commit with the bug entry.
		childRepo.AddGen(ctx, "myfile")
		hash := childRepo.CommitMsg(ctx, fmt.Sprintf(`Some dummy commit

%s
`, bugLine))
		details, err := git.GitDir(childRepo.Dir()).Details(ctx, hash)
		require.NoError(t, err)
		rev := revision.FromLongCommit(fmt.Sprintf(gitiles.COMMIT_URL, cfg.ChildRepo, "%s"), details)
		// Update.
		lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
		require.NoError(t, err)
		require.Equal(t, hash, tipRev.Id)

		// Mock the request to load the change.
		mockGerritGetAndPublishChange(t, urlmock, cfg)

		// Create a roll.
		_, err = rm.CreateNewRoll(ctx, lastRollRev, rev, notRolledRevs, emails, cqExtraTrybots, false)
		require.NoError(t, err)

		// Verify that we passed the correct --bug argument to roll-dep.
		found := false
		for _, line := range strings.Split(lastUpload.Body, "\n") {
			if strings.HasPrefix(line, "BUG=") {
				found = true
				require.Equal(t, expect, line[4:])
			} else if strings.HasPrefix(line, "Bug: ") {
				found = true
				require.Equal(t, expect, line[5:])
			}
		}
		if expect == "" {
			require.False(t, found)
		} else {
			require.True(t, found)
		}
	}

	// Test cases.
	test("", "None")
	test("BUG=skiatestproject:23", "skiatestproject:23")
	test("BUG=skiatestproject:18,skiatestproject:58", "skiatestproject:18,skiatestproject:58")
	// No prefix defaults to "chromium", which we don't include for rolls into "skiatestproject".
	test("BUG=skiatestproject:18,58", "skiatestproject:18")
	test("BUG=456", "None")
	test("BUG=skia:123,chromium:4532,skiatestproject:21", "skiatestproject:21")
	test("Bug: skiatestproject:33", "skiatestproject:33")
}

func TestDEPSConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := depsCfg(t)
	// These are not supplied above.
	cfg.ChildRepo = "dummy"
	cfg.ParentRepo = "dummy"
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &DEPSRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}
