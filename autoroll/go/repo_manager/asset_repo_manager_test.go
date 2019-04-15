package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
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
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    "unused/by/asset/repomanager",
				ParentBranch: "master",
			},
			ParentRepo: "", // Filled in after GitInit().
		},
		Asset:     assetName,
		ChildRepo: "", // Filled in after GitInit().
	}
}

func setupAsset(t *testing.T) (context.Context, RepoManager, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, *vcsinfo.LongCommit, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	cfg := assetCfg()

	lastUpload := new(vcsinfo.LongCommit)
	mockRun := &exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" && cmd.Args[0] == "cl" {
			if cmd.Args[1] == "upload" {
				d, err := git.GitDir(cmd.Dir).Details(ctx, "HEAD")
				if err != nil {
					return err
				}
				*lastUpload = *d
				return nil
			} else if cmd.Args[1] == "issue" {
				json := testutils.MarshalJSON(t, &issueJson{
					Issue:    issueNum,
					IssueUrl: "???",
				})
				f := strings.Split(cmd.Args[2], "=")[1]
				testutils.WriteFile(t, f, json)
				return nil
			}
		}
		return exec.DefaultRun(cmd)
	})

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

	recipesCfg := path.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)
	rm, err := NewAssetRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", urlmock.Client(), gerritCR(t, g), false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, rm, mockParent, parent, mockChild, child, lastUpload, urlmock, cleanup
}

func TestAssetRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, rm, mockParent, parent, mockChild, child, lastUpload, urlmock, cleanup := setupAsset(t)
	defer cleanup()

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
	ci := gerrit.ChangeInfo{
		ChangeId: "12345",
		Id:       "12345",
		Issue:    12345,
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
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/12345/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	from, to, err := autoroll.RollRev(ctx, lastUpload.Subject, nil)
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
