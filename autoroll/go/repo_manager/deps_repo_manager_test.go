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
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	childPath       = "path/to/child"
	cqExtraTrybots  = ""
	issueNum        = int64(12345)
	mockServer      = "https://skia-review.googlesource.com"
	mockUser        = "user@chromium.org"
	numChildCommits = 10
	fakeCommitMsg   = `Roll fake-dep oldrev..newrev

blah blah blah
`
	fakeCommitMsgMock = fakeCommitMsg + "\nChange-Id: 123"
)

var (
	emails = []string{"reviewer@chromium.org"}
)

func depsCfg(t *testing.T) *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGerritParent{
			DepsLocalGerritParent: &config.DEPSLocalGerritParentConfig{
				DepsLocal: &config.DEPSLocalParentConfig{
					GitCheckout: &config.GitCheckoutParentConfig{
						GitCheckout: &config.GitCheckoutConfig{
							Branch:  git.MasterBranch,
							RepoUrl: "TODO",
						},
						Dep: &config.DependencyConfig{
							Primary: &config.VersionFileConfig{
								Id:   "TODO",
								Path: deps_parser.DepsFileName,
							},
						},
					},
					ChildPath: childPath,
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutChild{
			GitCheckoutChild: &config.GitCheckoutChildConfig{
				GitCheckout: &config.GitCheckoutConfig{
					Branch:  git.MasterBranch,
					RepoUrl: "TODO",
				},
			},
		},
	}
}

func setupDEPSRepoManager(t *testing.T, cfg *config.ParentChildRepoManagerConfig) (context.Context, *parentChildRepoManager, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *vcsinfo.LongCommit, *mockhttpclient.URLMock, *bool, func()) {
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
	patchRefInSyncCmd := new(bool)
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
		} else if cmd.Name == "python" && strings.Contains(cmd.Args[0], "gclient.py") && cmd.Args[1] == "sync" && util.In("--patch-ref", cmd.Args) && util.In("--no-rebase-patch-ref", cmd.Args) && util.In("--no-reset-patch-ref", cmd.Args) {
			*patchRefInSyncCmd = true
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})

	urlmock := mockhttpclient.NewURLMock()
	g := setupFakeGerrit(t, cfg.GetDepsLocalGerritParent().GetGerrit(), urlmock)

	// We have a chicken-and-egg problem where the config needs to be passed in,
	// but the caller needs the repo URLs as part of the config. Set the child
	// and parent repo URLs directly on the config, and if the ParentPath and
	// GClientSpec entries are set, treat them as text templates.
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent).DepsLocalGerritParent.DepsLocal
	parentCfg.GitCheckout.Dep.Primary.Id = child.RepoUrl()
	parentCfg.GitCheckout.GitCheckout.RepoUrl = parent.RepoUrl()
	childCfg := cfg.Child.(*config.ParentChildRepoManagerConfig_GitCheckoutChild)
	childCfg.GitCheckoutChild.GitCheckout.RepoUrl = child.RepoUrl()

	vars := struct {
		ParentRepo string
		ParentBase string
	}{
		ParentRepo: parentCfg.GitCheckout.GitCheckout.RepoUrl,
		ParentBase: path.Base(parentCfg.GitCheckout.GitCheckout.RepoUrl),
	}
	parentCfg.CheckoutPath = testutils.ExecTemplate(t, parentCfg.CheckoutPath, vars)
	parentCfg.GclientSpec = testutils.ExecTemplate(t, parentCfg.GclientSpec, vars)

	// Create the RepoManager.
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)
	rm, err := newParentChildRepoManager(ctx, cfg, setupRegistry(t), wd, "fake-roller", recipesCfg, "fake.server.com", urlmock.Client(), gerritCR(t, g))
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, rm, wd, child, childCommits, parent, mockRun, lastUpload, urlmock, patchRefInSyncCmd, cleanup
}

func setupFakeGerrit(t *testing.T, cfg *config.GerritConfig, urlMock *mockhttpclient.URLMock) *gerrit.Gerrit {
	// Create a dummy commit-msg hook.
	changeId := "123"
	respBody := []byte(fmt.Sprintf(`#!/bin/sh
git interpret-trailers --trailer "Change-Id: %s" >> $1
`, changeId))
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/tools/hooks/commit-msg", mockhttpclient.MockGetDialogue(respBody))

	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(cfg.Url+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerritWithConfig(codereview.GerritConfigs[cfg.Config], cfg.Url, urlMock.Client())
	require.NoError(t, err)

	return g
}

// TestRepoManager tests all aspects of the DEPSRepoManager except for CreateNewRoll.
func TestDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	cfg := depsCfg(t)
	ctx, rm, _, _, childCommits, _, _, _, _, _, cleanup := setupDEPSRepoManager(t, cfg)
	defer cleanup()

	// Test update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))
}

func mockGerritGetAndPublishChange(t *testing.T, urlmock *mockhttpclient.URLMock, cfg *config.ParentChildRepoManagerConfig) {
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
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent).DepsLocalGerritParent
	gerritCfg := codereview.GerritConfigs[parentCfg.Gerrit.Config]
	if gerritCfg.HasCq {
		reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	} else {
		reqBody = []byte(`{"labels":{"Code-Review":1},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	}
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps2/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))
	if !gerritCfg.HasCq {
		urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/submit", mockhttpclient.MockPostDialogue("application/json", []byte("{}"), []byte("")))
	}
}

func TestDEPSRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	cfg := depsCfg(t)
	ctx, rm, _, _, _, _, _, _, urlmock, _, cleanup := setupDEPSRepoManager(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, int64(123), issue)
}

func TestDEPSRepoManagerCreateNewRollWithPatchRef(t *testing.T) {
	unittest.LargeTest(t)

	cfg := depsCfg(t)
	ctx, rm, _, _, _, _, _, _, urlmock, patchRefInSyncCmd, cleanup := setupDEPSRepoManager(t, cfg)
	defer cleanup()

	lastRollRev, _, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, false, *patchRefInSyncCmd)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Use a patch ref and create a roll.
	unsubmittedRev := &revision.Revision{
		Id: "refs/changes/11/1111/1",
	}
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, unsubmittedRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, true, *patchRefInSyncCmd)
	require.Equal(t, int64(123), issue)
}

// Verify that we ran the PreUploadSteps.
func TestDEPSRepoManagerPreUploadSteps(t *testing.T) {
	unittest.LargeTest(t)

	// Create a dummy pre-upload step.
	ran := false
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string, *revision.Revision, *revision.Revision) error {
		ran = true
		return nil
	})

	cfg := depsCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent).DepsLocalGerritParent.DepsLocal
	parentCfg.PreUploadSteps = []config.PreUploadStep{stepName}

	ctx, rm, _, _, _, _, _, _, urlmock, _, cleanup := setupDEPSRepoManager(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.True(t, ran)
}

// Verify that we properly utilize a gclient spec.
func TestDEPSRepoManagerGClientSpec(t *testing.T) {
	unittest.LargeTest(t)

	gclientSpec := `
solutions=[{
  "name": "alternate/location/{{.ParentBase}}",
  "url": "{{.ParentRepo}}",
  "deps_file": "DEPS",
  "managed": False,
  "custom_deps": {},
  "custom_vars": {
    "a": "b",
    "c": "d",
  },
}];
cache_dir=None
`
	// Remove newlines.
	gclientSpec = strings.Replace(gclientSpec, "\n", "", -1)
	cfg := depsCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent).DepsLocalGerritParent.DepsLocal
	parentCfg.GclientSpec = gclientSpec
	parentCfg.CheckoutPath = filepath.Join("alternate", "location", "{{.ParentBase}}")

	ctx, rm, _, _, _, _, mockRun, _, urlmock, _, cleanup := setupDEPSRepoManager(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mock the request to load the change.
	mockGerritGetAndPublishChange(t, urlmock, cfg)

	// Create a roll.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)

	// Ensure that we pass the spec into "gclient config".
	found := false
	for _, c := range mockRun.Commands() {
		if c.Name == "python" && strings.Contains(c.Args[0], parent.GClient) && c.Args[1] == "config" {
			for _, arg := range c.Args {
				if strings.HasPrefix(arg, "--spec=") && strings.Contains(arg, `"a": "b",`) {
					found = true
				}
			}
		}
	}
	require.True(t, found)
}
