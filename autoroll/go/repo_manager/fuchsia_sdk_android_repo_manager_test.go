package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func fuchsiaAndroidCfg(t *testing.T) *FuchsiaSDKAndroidRepoManagerConfig {
	return &FuchsiaSDKAndroidRepoManagerConfig{
		FuchsiaSDKRepoManagerConfig: FuchsiaSDKRepoManagerConfig{
			NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
				CommonRepoManagerConfig: CommonRepoManagerConfig{
					ChildBranch:  masterBranchTmpl(t),
					ChildPath:    "external/fuchsia_sdk",
					ParentBranch: masterBranchTmpl(t),
				},
			},
		},
		GenSdkBpRepo:   "TODO",
		GenSdkBpBranch: "master",
	}
}

func TestFuchsiaSDKAndroidConfig(t *testing.T) {
	unittest.SmallTest(t)

	cfg := fuchsiaAndroidCfg(t)
	cfg.ParentRepo = "todo.git"
	require.NoError(t, cfg.Validate())
}

func setupFuchsiaSDKAndroid(t *testing.T) (context.Context, string, *fuchsiaSDKAndroidRepoManager, *mockhttpclient.URLMock, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	cfg := fuchsiaAndroidCfg(t)

	// Mock out repo commands.
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "repo") {
			return nil
		} else if strings.Contains(cmd.Name, "git") && strings.Contains(cmd.Dir, cfg.ChildPath) {
			var output string
			if cmd.Args[0] == "log" {
				if cmd.Args[1] == "--format=format:%H%x20%ci" {
					output = fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", childCommits[0], childCommits[1])
				}
			} else if cmd.Args[0] == "ls-remote" {
				output = childCommits[0]
			} else if cmd.Args[0] == "merge-base" {
				output = childCommits[1]
			}
			n, err := cmd.CombinedOutput.Write([]byte(output))
			require.NoError(t, err)
			require.Equal(t, len(output), n)
			return nil
		} else if cmd.Name == "python" && strings.Contains(cmd.Args[0], GEN_SDK_BP) {
			androidBuildTop := ""
			for _, env := range cmd.Env {
				if strings.HasPrefix(env, "ANDROID_BUILD_TOP") {
					androidBuildTop = strings.Split(env, "=")[1]
				}
			}
			require.NotEqual(t, "", androidBuildTop)
			androidBp := path.Join(androidBuildTop, cfg.ChildPath, ANDROID_BP)
			require.NoError(t, os.MkdirAll(path.Dir(androidBp), os.ModePerm))
			require.NoError(t, ioutil.WriteFile(androidBp, []byte("hi"), os.ModePerm))
			return nil
		} else {
			return exec.DefaultRun(ctx, cmd)
		}
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	// Create repos.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, FUCHSIA_SDK_ANDROID_VERSION_FILE, fuchsiaSDKRevBase)
	parent.Commit(ctx)
	cfg.ParentRepo = parent.RepoUrl()

	// This is not technically correct, but the call into gen_sdk_bp is
	// mocked and we have to check out something.
	cfg.GenSdkBpRepo = parent.RepoUrl()

	urlmock := mockhttpclient.NewURLMock()
	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	gUrl := "https://fake-skia-review.googlesource.com"
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlmock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerritWithConfig(gerrit.CONFIG_ANDROID, gUrl, urlmock.Client())
	require.NoError(t, err)

	// Initial update, everything up-to-date.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_ANDROID_VERSION_FILE, parentMaster)
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
	})
	mockGetLatestSDK(urlmock, FUCHSIA_SDK_GS_LATEST_PATH_LINUX, FUCHSIA_SDK_GS_LATEST_PATH_MAC, fuchsiaSDKRevBase, "mac-base")
	mockDownloadSDK(t, urlmock, fuchsiaSDKRevBase, wd)

	rm, err := NewFuchsiaSDKAndroidRepoManager(ctx, cfg, setupRegistry(t), wd, g, "fake.server.com", urlmock.Client(), androidGerrit(t, g), false)
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, wd, rm.(*fuchsiaSDKAndroidRepoManager), urlmock, mockParent, parent, cleanup
}

func mockDownloadSDK(t *testing.T, urlmock *mockhttpclient.URLMock, rev, wd string) {
	archive := path.Join(wd, "archive.tgz")
	sdkDir := path.Join(wd, "sdk")
	require.NoError(t, os.MkdirAll(sdkDir, os.ModePerm))
	sdkFile := path.Join(sdkDir, "file1")
	testutils.WriteFile(t, sdkFile, "contents")
	_, err := exec.RunCwd(context.Background(), sdkDir, "tar", "-czf", archive, "file1")
	require.NoError(t, err)
	contents, err := ioutil.ReadFile(archive)
	require.NoError(t, err)
	url := fmt.Sprintf("https://storage.googleapis.com/%s/%s/linux-amd64/%s", FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, rev)
	urlmock.MockOnce(url, mockhttpclient.MockGetDialogue(contents))
}

func TestFuchsiaSDKAndroidRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, rm, urlmock, mockParent, parent, cleanup := setupFuchsiaSDKAndroid(t)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevBase, lastRollRev.Id)
	require.Equal(t, fuchsiaSDKRevBase, tipRev.Id)
	prev, err := rm.GetRevision(ctx, fuchsiaSDKRevPrev)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevPrev, prev.Id)
	base, err := rm.GetRevision(ctx, fuchsiaSDKRevBase)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevBase, base.Id)
	next, err := rm.GetRevision(ctx, fuchsiaSDKRevNext)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevNext, next.Id)
	require.Empty(t, rm.preUploadSteps)
	require.Equal(t, 0, len(notRolledRevs))

	// There's a new version.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_ANDROID_VERSION_FILE, parentMaster)
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevNext: fuchsiaSDKTimeNext,
	})
	mockGetLatestSDK(urlmock, FUCHSIA_SDK_GS_LATEST_PATH_LINUX, FUCHSIA_SDK_GS_LATEST_PATH_MAC, fuchsiaSDKRevNext, "mac-next")
	mockDownloadSDK(t, urlmock, fuchsiaSDKRevNext, wd)

	lastRollRev, tipRev, notRolledRevs, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevBase, lastRollRev.Id)
	require.Equal(t, fuchsiaSDKRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, fuchsiaSDKRevNext, notRolledRevs[0].Id)

	// Upload a CL.

	// Mock the initial change creation.
	from := lastRollRev
	to := tipRev
	commitMsg := fmt.Sprintf(`Roll Fuchsia SDK from %s to %s

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
fake.server.com
Please CC reviewer@chromium.org on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

Tbr: reviewer@chromium.org
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.`, from, to)
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.noCheckoutRepoManager.gerritConfig.Project, subject, rm.parentBranch, parentMaster))
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
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(commitMsg, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the version files.
	reqBody = []byte(tipRev.Id)
	reqUrl := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(FUCHSIA_SDK_ANDROID_VERSION_FILE))
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	reqBody = []byte("hi")
	reqUrl = "https://fake-skia-review.googlesource.com/a/changes/123/edit/Android.bp"
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	reqBody = []byte("contents")
	reqUrl = "https://fake-skia-review.googlesource.com/a/changes/123/edit/file1"
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Autosubmit":1,"Code-Review":2,"Presubmit-Ready":1},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, ci.Issue, issue)
}

func TestFuchsiaSDKAndroidConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := fuchsiaAndroidCfg(t)
	cfg.ParentRepo = "dummy" // Not supplied above.
	require.NoError(t, cfg.Validate())

	cfg.GenSdkBpRepo = ""
	require.EqualError(t, cfg.Validate(), "GenSdkBpRepo is required.")

	// The remaining fields come from the nested Configs, so exclude them
	// and verify that we fail validation.
	cfg = fuchsiaAndroidCfg(t)
	cfg.FuchsiaSDKRepoManagerConfig = FuchsiaSDKRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}
