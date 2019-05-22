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

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	fuchsiaSDKRevPrev = "000633ae6e904f7eaced443d6aa65fb3d24afe8c"
	fuchsiaSDKRevBase = "32a56ad54471732034ba802cbfc3c9ff277b9d1c"
	fuchsiaSDKRevNext = "37417b795289818723990da66dd7a7b38e50fc04"

	fuchsiaSDKTimePrev = "2009-11-10T23:00:01Z"
	fuchsiaSDKTimeBase = "2009-11-10T23:00:02Z"
	fuchsiaSDKTimeNext = "2009-11-10T23:00:03Z"

	fuchsiaSDKArchiveUrlTmpl = "https://storage.googleapis.com/fuchsia/sdk/core/%s/%s"
)

var (
	fuchsiaSDKLatestArchiveUrlLinux = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "linux-amd64", "LATEST_ARCHIVE")
	fuchsiaSDKLatestArchiveUrlMac   = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "mac-amd64", "LATEST_ARCHIVE")
)

func fuchsiaCfg() *FuchsiaSDKRepoManagerConfig {
	return &FuchsiaSDKRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    "unused/by/fuchsiaSDK/repomanager",
				ParentBranch: "master",
			},
		},
		IncludeMacSDK: true,
	}
}

func setupFuchsiaSDK(t *testing.T) (context.Context, RepoManager, *mockhttpclient.URLMock, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	ctx := context.Background()

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_LINUX, fuchsiaSDKRevBase)
	parent.Add(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_MAC, fuchsiaSDKRevBase)
	parent.Commit(ctx)

	urlmock := mockhttpclient.NewURLMock()
	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	gUrl := "https://fake-skia-review.googlesource.com"
	gitcookies := path.Join(wd, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	assert.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlmock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlmock.Client())
	assert.NoError(t, err)

	cfg := fuchsiaCfg()
	cfg.ParentRepo = parent.RepoUrl()

	// Initial update, everything up-to-date.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_LINUX, parentMaster)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_MAC, parentMaster)
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
	})
	mockGetLatestSDK(urlmock, FUCHSIA_SDK_GS_LATEST_PATH_LINUX, FUCHSIA_SDK_GS_LATEST_PATH_MAC, fuchsiaSDKRevBase, "mac-base")

	rm, err := NewFuchsiaSDKRepoManager(ctx, cfg, wd, g, "fake.server.com", "", urlmock.Client(), gerritCR(t, g), false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, rm, urlmock, mockParent, parent, cleanup
}

func mockGetLatestSDK(urlmock *mockhttpclient.URLMock, pathLinux, pathMac, revLinux, revMac string) {
	urlmock.MockOnce("https://storage.googleapis.com/"+FUCHSIA_SDK_GS_BUCKET+"/"+pathLinux, mockhttpclient.MockGetDialogue([]byte(revLinux)))
	urlmock.MockOnce("https://storage.googleapis.com/"+FUCHSIA_SDK_GS_BUCKET+"/"+pathMac, mockhttpclient.MockGetDialogue([]byte(revMac)))
}

func TestFuchsiaSDKRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, rm, urlmock, mockParent, parent, cleanup := setupFuchsiaSDK(t)
	defer cleanup()

	assert.Equal(t, fuchsiaSDKRevBase, rm.LastRollRev())
	assert.Equal(t, fuchsiaSDKRevBase, rm.NextRollRev())
	rolledPast, err := rm.RolledPast(ctx, fuchsiaSDKRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	assert.Empty(t, rm.PreUploadSteps())
	assert.Equal(t, 0, len(rm.NotRolledRevisions()))

	// There's a new version.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_LINUX, parentMaster)
	mockParent.MockReadFile(ctx, FUCHSIA_SDK_VERSION_FILE_PATH_MAC, parentMaster)
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevNext: fuchsiaSDKTimeNext,
	})
	mockGetLatestSDK(urlmock, FUCHSIA_SDK_GS_LATEST_PATH_LINUX, FUCHSIA_SDK_GS_LATEST_PATH_MAC, fuchsiaSDKRevNext, "mac-next")
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, fuchsiaSDKRevBase, rm.LastRollRev())
	assert.Equal(t, fuchsiaSDKRevNext, rm.NextRollRev())
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)
	assert.Equal(t, 1, len(rm.NotRolledRevisions()))
	assert.Equal(t, fuchsiaSDKRevNext, rm.NotRolledRevisions()[0].Id)

	// Upload a CL.

	// Mock the initial change creation.
	from := fuchsiaSDKShortVersion(rm.LastRollRev())
	to := fuchsiaSDKShortVersion(rm.NextRollRev())
	commitMsg := fmt.Sprintf(FUCHSIA_SDK_COMMIT_MSG_TMPL, from, to, "fake.server.com")
	commitMsg += "\nTBR=reviewer@chromium.org"
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.(*fuchsiaSDKRepoManager).noCheckoutRepoManager.gerritConfig.Project, subject, rm.(*fuchsiaSDKRepoManager).parentBranch, parentMaster))
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
	assert.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(commitMsg, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the version files.
	reqBody = []byte(rm.NextRollRev())
	reqUrl := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(FUCHSIA_SDK_VERSION_FILE_PATH_LINUX))
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	reqBody = []byte(rm.(*fuchsiaSDKRepoManager).nextRollRevMac)
	reqUrl = fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(FUCHSIA_SDK_VERSION_FILE_PATH_MAC))
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	assert.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, ci.Issue, issue)
}

func TestFuchsiaSDKConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := fuchsiaCfg()
	cfg.ParentRepo = "dummy" // Not supplied above.
	assert.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &FuchsiaSDKRepoManagerConfig{}
	assert.Error(t, cfg.Validate())
}
