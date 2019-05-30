package repo_manager

import (
	"context"
	"errors"
	//"time"
	//"encoding/json"
	//"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	//github_api "github.com/google/go-github/github"
	assert "github.com/stretchr/testify/require"
	//"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	//"go.skia.org/infra/go/github"
	//"go.skia.org/infra/go/mockhttpclient"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	GITHUB_CIPD_DEPS_CHILD_PATH = "path/to/child"
)

//func githubCR(t *testing.T, g *github.GitHub) codereview.CodeReview {
//	rv, err := (&codereview.GithubConfig{
//		RepoOwner:     "me",
//		RepoName:      "my-repo",
//		ChecksNum:     3,
//		ChecksWaitFor: []string{"a", "b", "c"},
//	}).Init(nil, g)
//	assert.NoError(t, err)
//	return rv
//}

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
		CipdAssetName: "test/cipd/name",
		CipdAssetTag:  "latest",
	}
}

func setupGithubCipdDEPS(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	childPath := filepath.Join(wd, "github_repos", "earth")
	assert.NoError(t, os.MkdirAll(childPath, 0755))
	child := git_testutils.GitInitWithDir(t, context.Background(), childPath)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	//parentPath := filepath.Join(wd, "github_repos", "krypton")
	//assert.NoError(t, os.MkdirAll(parentPath, 0755))
	//parent := git_testutils.GitInitWithDir(t, context.Background(), parentPath)
	//parent.Add(context.Background(), "dummy-file.txt", fmt.Sprintf(`%s`, childCommits[0]))
	//parent.Commit(context.Background())
	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`
deps = {
  "%s": {
    "packages": [
	  {
	    "package": "%s",
	    "version": "xyz"
	  }
	],
  },
}`, GITHUB_CIPD_DEPS_CHILD_PATH, "test/cipd/name"))
	parent.Commit(context.Background())
	fmt.Println("WROTE THIS!!!!!!!!!")
	fmt.Println(fmt.Sprintf(`
deps = {
  "%s": {
    "packages": [
	  {
	    "package": "%s",
	    "version": "xyz"
	  }
	],
  },
}`, "path/to/child", "test/cipd/name"))

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		fmt.Println("XXXXXXXXXXXXXXXXXXXXXXX")
		fmt.Println(cmd)
		fmt.Println(cmd.Name)
		fmt.Println(strings.Contains(cmd.Name, "gclient"))
		fmt.Println(cmd.Args[0])
		//if cmd.Name == "python" && strings.Contains(cmd.Args[0], "gclient") && cmd.Args[1] == "sync" {
		//	return nil
		//}
		if strings.Contains(cmd.Name, "gclient") && cmd.Args[0] == "runhooks" {
			return nil
		}
		if cmd.Name == "git" {
			if cmd.Args[0] == "clone" || cmd.Args[0] == "fetch" {
				return nil
			}
			if cmd.Args[0] == "checkout" && cmd.Args[1] == "remote/master" {
				// Pretend origin is the remote branch for testing ease.
				cmd.Args[1] = "origin/master"
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, cleanup
}

//func setupFakeGithub(t *testing.T, childCommits []string) (*github.GitHub, *mockhttpclient.URLMock) {
//	urlMock := mockhttpclient.NewURLMock()

//	// Mock /user endpoint.
//	serializedUser, err := json.Marshal(&github_api.User{
//		Login: &mockGithubUser,
//		Email: &mockGithubUserEmail,
//	})
//	assert.NoError(t, err)
//	urlMock.MockOnce(githubApiUrl+"/user", mockhttpclient.MockGetDialogue(serializedUser))

//	// Mock getRawFile.
//	urlMock.MockOnce("https://raw.githubusercontent.com/superman/krypton/master/dummy-file.txt", mockhttpclient.MockGetDialogue([]byte(childCommits[0])))

//	// Mock /issues endpoint for get and patch requests.
//	serializedIssue, err := json.Marshal(&github_api.Issue{
//		Labels: []github_api.Label{},
//	})
//	assert.NoError(t, err)
//	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345", mockhttpclient.MockGetDialogue(serializedIssue))
//	patchRespBody := []byte(testutils.MarshalJSON(t, &github_api.PullRequest{}))
//	patchReqType := "application/json"
//	patchReqBody := []byte(`{"labels":["autoroller: commit"]}
//`)
//	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
//	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345", patchMd)

//	g, err := github.NewGitHub(context.Background(), "superman", "krypton", urlMock.Client())
//	assert.NoError(t, err)
//	return g, urlMock
//}

//func mockGithubRequests(t *testing.T, urlMock *mockhttpclient.URLMock, from, to string, numCommits int) {
//	// Mock /pulls endpoint.
//	serializedPull, err := json.Marshal(&github_api.PullRequest{
//		Number: &testPullNumber,
//	})
//	assert.NoError(t, err)
//	reqType := "application/json"
//	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, mockhttpclient.DONT_CARE_REQUEST, serializedPull, http.StatusCreated)
//	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/pulls", md)

//	// Mock /comments endpoint.
//	reqType = "application/json"
//	reqBody := []byte(`{"body":"@reviewer : New roll has been created by fake.server.com"}
//`)
//	md = mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, nil, http.StatusCreated)
//	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345/comments", md)
//}

func getMockHttpClient() *http.Client {
	urlmock := mockhttpclient.NewURLMock()
	////mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	//gUrl := "https://fake-skia-review.googlesource.com"
	//gitcookies := path.Join(wd, "gitcookies_fake")
	//assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	//serialized, err := json.Marshal(&gerrit.AccountDetails{
	//	AccountId: 101,
	//	Name:      mockUser,
	//	Email:     mockUser,
	//	UserName:  mockUser,
	//})
	//assert.NoError(t, err)
	//serialized = append([]byte("abcd\n"), serialized...)
	//serialized := []byte(`
	//test/cipd/namelatest`)
	post := mockhttpclient.MockPostError("application/prpc; encoding=binary", mockhttpclient.DONT_CARE_REQUEST, "", 0)
	// mockhttpclient.MockPostDialogue("application/prpc; encoding=binary", mockhttpclient.DONT_CARE_REQUEST, []byte(""))
	urlmock.MockOnce("https://chrome-infra-packages.appspot.com/prpc/cipd.Repository/ResolveVersion", post)
	//g, err := gerrit.NewGerrit(gUrl, gitcookies, urlmock.Client())
	//assert.NoError(t, err)
	return urlmock.Client()
}

type instanceEnumeratorImpl struct {
	done bool
}

func (e *instanceEnumeratorImpl) Next(ctx context.Context, limit int) ([]cipd.InstanceInfo, error) {
	if e.done {
		return nil, nil
	}
	instances := []cipd.InstanceInfo{}
	//now := time.Now()
	instance0 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: "test/cipd/name",
			InstanceID:  "xyz",
		},
		RegisteredBy: "superman@krypton.com",
		//RegisteredTs: time.Time{},
	}
	instance1 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: "test/cipd/name",
			InstanceID:  "abc",
		},
		RegisteredBy: "superman@krypton.com",
		//RegisteredTs: time.Time{},
	}
	instance2 := cipd.InstanceInfo{
		Pin: common.Pin{
			PackageName: "test/cipd/name",
			InstanceID:  "def",
		},
		RegisteredBy: "batman@gotham.com",
		//RegisteredTs: time.Time{},
	}
	instances = append(instances, instance0, instance1, instance2)
	e.done = true
	return instances, nil
}

// TestGithubRepoManager tests all aspects of the GithubRepoManager except for CreateNewRoll.
func TestGithubCipdDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, _ := setupFakeGithub(t, childCommits)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	// (ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool)
	//client := getMockHttpClient()
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: "test/cipd/name",
		InstanceID:  "abc",
	}
	cipdClient.On("ResolveVersion", ctx, "test/cipd/name", "latest").Return(head, nil).Once()
	cipdClient.On("ListInstances", ctx, "test/cipd/name").Return(&instanceEnumeratorImpl{}, nil).Once()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil /*httpClient*/, cipdClient, githubCR(t, g), false)

	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, "xyz", rm.LastRollRev())
	assert.Equal(t, "abc", rm.NextRollRev()) // change this
	assert.Equal(t, 2, len(rm.NotRolledRevisions()))
	assert.Equal(t, "abc", rm.NotRolledRevisions()[0].Id)
	assert.Equal(t, "test/cipd/name:abc", rm.NotRolledRevisions()[0].Display)
	assert.Equal(t, "def", rm.NotRolledRevisions()[1].Id)
	assert.Equal(t, "test/cipd/name:def", rm.NotRolledRevisions()[1].Display)
	for _, r := range rm.NotRolledRevisions() {
		fmt.Println(r)
	}
	fmt.Println(rm.NotRolledRevisions())

	//// RolledPast.
	//rp, err := rm.RolledPast(ctx, "xyz")
	//assert.NoError(t, err)
	//assert.True(t, rp)
	//for _, c := range childCommits[1:] {
	//	rp, err := rm.RolledPast(ctx, c)
	//	assert.NoError(t, err)
	//	assert.False(t, rp)
	//}
}

func TestCreateNewGithubCipdDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, cleanup := setupGithub(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, childCommits)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	// (ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool)
	//client := getMockHttpClient()
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: "test/cipd/name",
		InstanceID:  "abc",
	}
	cipdClient.On("ResolveVersion", ctx, "test/cipd/name", "latest").Return(head, nil).Once()
	cipdClient.On("ListInstances", ctx, "test/cipd/name").Return(&instanceEnumeratorImpl{}, nil).Once()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil /*httpClient*/, cipdClient, githubCR(t, g), false)

	// Create a roll.
	mockGithubRequests(t, urlMock, rm.LastRollRev(), rm.NextRollRev(), len(rm.NotRolledRevisions()))
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), githubEmails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsGithubCipdDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, childCommits)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	// (ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool)
	//client := getMockHttpClient()
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: "test/cipd/name",
		InstanceID:  "abc",
	}
	cipdClient.On("ResolveVersion", ctx, "test/cipd/name", "latest").Return(head, nil).Once()
	cipdClient.On("ListInstances", ctx, "test/cipd/name").Return(&instanceEnumeratorImpl{}, nil).Once()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil /*httpClient*/, cipdClient, githubCR(t, g), false)

	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
	ran := false
	rm.(*githubRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock, rm.LastRollRev(), rm.NextRollRev(), len(rm.NotRolledRevisions()))
	_, createErr := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), githubEmails, cqExtraTrybots, false)
	assert.NoError(t, createErr)
	assert.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestErrorPreUploadStepsGithubCipdDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, cleanup := setupGithubCipdDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, childCommits)
	cfg := githubCipdDEPSRmCfg()
	cfg.ParentRepo = parent.RepoUrl()
	// (ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool)
	//client := getMockHttpClient()
	cipdClient := &mocks.CIPDClient{}
	head := common.Pin{
		PackageName: "test/cipd/name",
		InstanceID:  "abc",
	}
	cipdClient.On("ResolveVersion", ctx, "test/cipd/name", "latest").Return(head, nil).Once()
	cipdClient.On("ListInstances", ctx, "test/cipd/name").Return(&instanceEnumeratorImpl{}, nil).Once()
	rm, err := NewGithubCipdDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil /*httpClient*/, cipdClient, githubCR(t, g), false)

	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
	ran := false
	expectedErr := errors.New("Expected error")
	rm.(*githubRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return expectedErr
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock, rm.LastRollRev(), rm.NextRollRev(), len(rm.NotRolledRevisions()))
	_, createErr := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), githubEmails, cqExtraTrybots, false)
	assert.Error(t, expectedErr, createErr)
	assert.True(t, ran)
}
