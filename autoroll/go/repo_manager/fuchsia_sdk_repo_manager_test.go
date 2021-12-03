package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
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

	fuchsiaSDKArchiveUrlTmpl = "https://storage.googleapis.com/fuchsia/development/%s"

	fuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	fuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"
)

var (
	fuchsiaSDKLatestArchiveUrlLinux = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "LATEST_LINUX")
	fuchsiaSDKLatestArchiveUrlMac   = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "LATEST_MAC")
)

func fuchsiaCfg() *config.ParentChildRepoManagerConfig {
	child := &config.ParentChildRepoManagerConfig_FuchsiaSdkChild{
		FuchsiaSdkChild: &config.FuchsiaSDKChildConfig{
			GcsBucket:            "fake-fuchsia-sdk",
			LatestLinuxPath:      "linux/LATEST",
			LatestMacPath:        "mac/LATEST",
			TarballLinuxPathTmpl: "%s.sdk",
		},
	}
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: "todo.git",
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id:   "FuchsiaSDK",
						Path: fuchsiaSDKVersionFilePathLinux,
					},
					Transitive: []*config.TransitiveDepConfig{
						{
							Child: &config.VersionFileConfig{
								Id:   child.FuchsiaSdkChild.LatestMacPath,
								Path: fuchsiaSDKVersionFilePathMac,
							},
							Parent: &config.VersionFileConfig{
								Id:   child.FuchsiaSdkChild.LatestLinuxPath,
								Path: fuchsiaSDKVersionFilePathMac,
							},
						},
					},
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: child,
	}
}

func setupFuchsiaSDK(t *testing.T) (context.Context, *parentChildRepoManager, *mockhttpclient.URLMock, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	ctx := context.Background()

	cfg := fuchsiaCfg()
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, fuchsiaSDKVersionFilePathLinux, fuchsiaSDKRevBase)
	parent.Add(ctx, fuchsiaSDKVersionFilePathMac, fuchsiaSDKRevBase)
	parent.Commit(ctx)
	parentCfg.Gitiles.RepoUrl = parent.RepoUrl()

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
	g, err := gerrit.NewGerrit(gUrl, urlmock.Client())
	require.NoError(t, err)

	// Initial update, everything up-to-date.
	mockParent.MockGetCommit(ctx, git.MainBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathLinux, parentHead)
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathMac, parentHead)
	mockGetLatestSDK(urlmock, fuchsiaSDKRevBase, "mac-base")

	rm, err := newParentChildRepoManager(ctx, cfg, setupRegistry(t), wd, "fake-roller", "fake-recipes-cfg", "fake.server.com", urlmock.Client(), gerritCR(t, g, urlmock.Client()))
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, rm, urlmock, mockParent, parent, cleanup
}

func mockGetLatestSDK(urlmock *mockhttpclient.URLMock, revLinux, revMac string) {
	c := fuchsiaCfg().GetFuchsiaSdkChild()
	urlmock.MockOnce("https://storage.googleapis.com/"+c.GcsBucket+"/"+c.LatestLinuxPath, mockhttpclient.MockGetDialogue([]byte(revLinux)))
	if c.LatestMacPath != "" {
		urlmock.MockOnce("https://storage.googleapis.com/"+c.GcsBucket+"/"+c.LatestMacPath, mockhttpclient.MockGetDialogue([]byte(revMac)))
	}
}

func TestFuchsiaSDKRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, rm, urlmock, mockParent, parent, cleanup := setupFuchsiaSDK(t)
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
	require.Equal(t, 0, len(notRolledRevs))

	// There's a new version.
	mockParent.MockGetCommit(ctx, git.MainBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathLinux, parentHead)
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathMac, parentHead)
	mockGetLatestSDK(urlmock, fuchsiaSDKRevNext, "mac-next")
	lastRollRev, tipRev, notRolledRevs, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevBase, lastRollRev.Id)
	require.Equal(t, fuchsiaSDKRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, fuchsiaSDKRevNext, notRolledRevs[0].Id)

	// Upload a CL.

	// Mock the request for the currently-pinned versions.
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathLinux, parentHead)
	mockParent.MockReadFile(ctx, fuchsiaSDKVersionFilePathMac, parentHead)

	// Mock the initial change creation.
	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, git.MainBranch, parentHead))
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Project:  "test-project",
		Branch:   "test-branch",
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
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(fakeCommitMsgMock, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the version files.
	reqBody = []byte(tipRev.Id + "\n")
	reqUrl := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(fuchsiaSDKVersionFilePathLinux))
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	reqBody = []byte("mac-next\n")
	reqUrl = fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(fuchsiaSDKVersionFilePathMac))
	urlmock.MockOnce(reqUrl, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS&o=SUBMITTABLE", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/test-project~test-branch~123/revisions/ps2/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, ci.Issue, issue)
}
