package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func copyCfg(t *testing.T) *CopyRepoManagerConfig {
	return &CopyRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  masterBranchTmpl(t),
				ParentBranch: masterBranchTmpl(t),
			},
		},
		VersionFile: filepath.Join(childPath, "version.sha1"),
		Copies: []parent.CopyEntry{
			// TODO(borenet): Test a directory.
			{
				SrcRelPath: path.Join("child-dir", "child-file.txt"),
				DstRelPath: path.Join(childPath, "parent-file.txt"),
			},
			{
				SrcRelPath: path.Join("child-dir", "child-subdir"),
				DstRelPath: path.Join(childPath, "parent-dir"),
			},
		},
		Gerrit: &codereview.GerritConfig{
			URL:     "https://fake-skia-review.googlesource.com",
			Project: "fake-gerrit-project",
			Config:  codereview.GERRIT_CONFIG_CHROMIUM,
		},
	}
}

func setupCopy(t *testing.T) (context.Context, *CopyRepoManagerConfig, string, *parentChildRepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	ctx := context.Background()
	cfg := copyCfg(t)
	child := git_testutils.GitInit(t, ctx)
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits-1; i++ {
		child.AddGen(ctx, cfg.Copies[0].SrcRelPath)
		child.AddGen(ctx, path.Join(cfg.Copies[1].SrcRelPath, "a"))
		child.AddGen(ctx, path.Join(cfg.Copies[1].SrcRelPath, "b"))
		child.AddGen(ctx, path.Join(cfg.Copies[1].SrcRelPath, "c"))
		childCommits = append(childCommits, child.Commit(ctx))
	}

	parent := git_testutils.GitInit(t, ctx)
	parent.AddGen(ctx, cfg.Copies[0].DstRelPath)
	parent.AddGen(ctx, path.Join(cfg.Copies[1].DstRelPath, "a"))
	parent.AddGen(ctx, path.Join(cfg.Copies[1].DstRelPath, "b"))
	parent.AddGen(ctx, path.Join(cfg.Copies[1].DstRelPath, "c"))
	parent.Add(ctx, filepath.Join(childPath, "version.sha1"), childCommits[0])
	parentMaster := parent.Commit(ctx)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") && cmd.Args[0] == "push" {
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	urlmock := mockhttpclient.NewURLMock()
	g := setupFakeGerrit(t, cfg.Gerrit, urlmock)

	// Mock requests for Update.
	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, cfg.VersionFile, parentMaster)

	// Create the RepoManager.
	rm, err := NewCopyRepoManager(ctx, cfg, setupRegistry(t), wd, g, "fake.server.com", urlmock.Client(), gerritCR(t, g), false)
	require.NoError(t, err)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, cfg, wd, rm, child, parent, mockChild, mockParent, childCommits, urlmock, cleanup
}

// TestCopyRepoManager tests all aspects of the CopyRepoManager.
func TestCopyRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg, _, rm, child, parent, mockChild, mockParent, childCommits, _, cleanup := setupCopy(t)
	defer cleanup()

	// New commit landed.
	lastCommit := child.CommitGen(ctx, "abc.txt")

	// Mock requests for Update.
	mockChild.MockGetCommit(ctx, lastCommit)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], lastCommit))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	parentMaster := strings.TrimSpace(parent.Git(ctx, "rev-parse", "master"))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, cfg.VersionFile, parentMaster)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, lastCommit, tipRev.Id)
	require.Equal(t, len(childCommits), len(notRolledRevs))
}

func TestCopyRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlMock, cleanup := setupCopy(t)
	defer cleanup()

	// Mock requests for Update.
	mockChild.MockGetCommit(ctx, childCommits[len(childCommits)-1])
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	parentMaster := strings.TrimSpace(parentRepo.Git(ctx, "rev-parse", "master"))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, cfg.VersionFile, parentMaster)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Mock requests for CreateNewRoll.
	mockChild.MockGetCommit(ctx, childCommits[0])
	mockChild.MockReadFile(ctx, cfg.Copies[0].SrcRelPath, lastRollRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[0].SrcRelPath, lastRollRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[1].SrcRelPath, lastRollRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[1].SrcRelPath, lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "a"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "a"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "b"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "b"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "c"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "c"), lastRollRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[0].SrcRelPath, tipRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[0].SrcRelPath, tipRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[1].SrcRelPath, tipRev.Id)
	mockChild.MockReadFile(ctx, cfg.Copies[1].SrcRelPath, tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "a"), tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "a"), tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "b"), tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "b"), tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "c"), tipRev.Id)
	mockChild.MockReadFile(ctx, path.Join(cfg.Copies[1].SrcRelPath, "c"), tipRev.Id)

	mockParent.MockReadFile(ctx, cfg.Copies[0].DstRelPath, parentMaster)
	mockParent.MockReadFile(ctx, cfg.Copies[1].DstRelPath, parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "a"), parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "a"), parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "b"), parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "b"), parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "c"), parentMaster)
	mockParent.MockReadFile(ctx, path.Join(cfg.Copies[1].DstRelPath, "c"), parentMaster)

	// Mock the initial change creation.
	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, "master", parentMaster))
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
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(fakeCommitMsg, "\n", "\\n", -1)))
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the requests to modify the copied files.
	mockChild.MockGetCommit(ctx, childCommits[0])
	mockUpdateFile := func(src, dst string) {
		contents, err := git.GitDir(childRepo.Dir()).GetFile(ctx, src, tipRev.Id)
		require.NoError(t, err)
		reqBody := []byte(contents)
		url := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(dst))
		urlMock.MockOnce(url, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	}
	mockUpdateFile(cfg.Copies[0].SrcRelPath, cfg.Copies[0].DstRelPath)
	mockUpdateFile(path.Join(cfg.Copies[1].SrcRelPath, "a"), path.Join(cfg.Copies[1].DstRelPath, "a"))
	mockUpdateFile(path.Join(cfg.Copies[1].SrcRelPath, "b"), path.Join(cfg.Copies[1].DstRelPath, "b"))
	mockUpdateFile(path.Join(cfg.Copies[1].SrcRelPath, "c"), path.Join(cfg.Copies[1].DstRelPath, "c"))

	// Mock the request to update the version file.
	reqBody = []byte(tipRev.Id)
	url := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", cfg.VersionFile)
	urlMock.MockOnce(url, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody = []byte(`{}`)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	gerritCfg := codereview.GERRIT_CONFIGS[cfg.Gerrit.Config]
	if gerritCfg.HasCq {
		reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	} else {
		reqBody = []byte(`{"labels":{"Code-Review":1},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	}
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))
	if !gerritCfg.HasCq {
		urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/submit", mockhttpclient.MockPostDialogue("application/json", []byte("{}"), []byte("")))
	}
	// Upload the CL.
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, int64(123), issue)
}
