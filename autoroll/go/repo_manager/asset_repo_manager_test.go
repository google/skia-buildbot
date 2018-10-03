package repo_manager

import (
	"bytes"
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
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	assetVersionBase = "7"
	assetVersionPrev = "6"
	assetVersionNext = "8"

	assetName = "test_asset"
)

var (
	assetVersionFile = fmt.Sprintf(ASSET_VERSION_TMPL, "test_asset")
)

func assetCfg() *AssetRepoManagerConfig {
	return &AssetRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    "unused/by/asset/repomanager",
				ParentBranch: "master",
			},
			GerritProject: "fake-gerrit-project",
			ParentRepo:    "", // Filled in after GitInit().
		},
		Asset:     assetName,
		ChildRepo: "", // Filled in after GitInit().
	}
}

func setupAsset(t *testing.T) (context.Context, RepoManager, *mockhttpclient.URLMock, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	ctx := context.Background()
	cfg := assetCfg()

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, assetVersionFile, assetVersionBase)
	parent.Commit(ctx)
	cfg.ParentRepo = parent.RepoUrl()

	child := git_testutils.GitInit(t, ctx)
	child.Add(ctx, assetVersionFile, assetVersionBase)
	child.Commit(ctx)
	cfg.ChildRepo = child.RepoUrl()

	urlmock := mockhttpclient.NewURLMock()
	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)
	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)

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

	// Initial update. Everything up-to-date.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, assetVersionFile, parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	childMaster, err := git.GitDir(child.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockChild.MockReadFile(ctx, assetVersionFile, childMaster)

	rm, err := NewAssetRepoManager(ctx, cfg, wd, g, "fake.server.com", "", urlmock.Client())
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, rm, urlmock, mockParent, parent, mockChild, child, cleanup
}

func TestAssetRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, rm, urlmock, mockParent, parent, mockChild, child, cleanup := setupAsset(t)
	defer cleanup()

	assert.Equal(t, mockUser, rm.User())
	assert.Equal(t, assetVersionBase, rm.LastRollRev())
	assert.Equal(t, assetVersionBase, rm.NextRollRev())
	rolledPast, err := rm.RolledPast(ctx, assetVersionPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, assetVersionNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)

	// There's a new version.
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "origin/master")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, assetVersionFile, parentMaster)
	child.Add(ctx, assetVersionFile, assetVersionNext)
	childMaster := child.Commit(ctx)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockReadFile(ctx, assetVersionFile, childMaster)
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, assetVersionBase, rm.LastRollRev())
	assert.Equal(t, assetVersionNext, rm.NextRollRev())
	rolledPast, err = rm.RolledPast(ctx, assetVersionPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, assetVersionBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, assetVersionNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)
	assert.Equal(t, 1, rm.CommitsNotRolled())

	// Upload a CL.

	// Mock the initial change creation.
	from := rm.LastRollRev()
	to := rm.NextRollRev()
	var buf bytes.Buffer
	data := struct {
		Asset          string
		CqExtraTrybots string
		Emails         string
		From           string
		ServerURL      string
		To             string
	}{
		Asset:          assetName,
		CqExtraTrybots: "",
		Emails:         "reviewer@chromium.org",
		From:           from,
		ServerURL:      "fake.server.com",
		To:             to,
	}
	err = commitMsgTmplAssets.Execute(&buf, data)
	assert.NoError(t, err)
	commitMsg := buf.String()
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.(*assetRepoManager).gerritProject, subject, rm.(*assetRepoManager).parentBranch, parentMaster))
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": &gerrit.Revision{
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

	// Mock the request to modify the version file.
	reqBody = []byte(rm.NextRollRev())
	url := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(assetVersionFile))
	urlmock.MockOnce(url, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

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

	// Ensure that we can parse the commit message.
	from, to, err = autoroll.RollRev(subject, func(h string) (string, error) {
		return rm.FullChildHash(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, assetVersionBase, from)
	assert.Equal(t, assetVersionNext, to)
}

func TestAssetConfigValidation(t *testing.T) {
	testutils.SmallTest(t)

	cfg := assetCfg()
	cfg.ChildRepo = "dummy"  // Not supplied above.
	cfg.ParentRepo = "dummy" // Not supplied above.
	assert.NoError(t, cfg.Validate())

	cfg.Asset = ""
	assert.EqualError(t, cfg.Validate(), "Asset is required.")
	cfg.Asset = assetName

	cfg.ChildRepo = ""
	assert.EqualError(t, cfg.Validate(), "ChildRepo is required.")
}
