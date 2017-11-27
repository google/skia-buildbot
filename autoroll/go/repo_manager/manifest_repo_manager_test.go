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
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

var (
	manifestEmails = []string{"reviewer@chromium.org"}
)

func setupManifest(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), manifestFileName, fmt.Sprintf(`
<manifest>
  <projects>
	<project name="third_party/sbase"
             path="third_party/sbase"
             remote="https://fuchsia.googlesource.com/third_party/sbase"
             gerrithost="https://fuchsia-review.googlesource.com"
			 revision="abc"
             githooks="manifest/git-hooks"/>
    <project name="%s"
             path="%s"
             remote="%s"
             revision="%s"/>
    <project name="third_party/snappy"
             path="third_party/snappy"
             remote="https://fuchsia.googlesource.com/third_party/snappy"
             gerrithost="https://fuchsia-review.googlesource.com"
             githooks="manifest/git-hooks"/>
  </projects>
</manifest>`, childPath, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" {
			if cmd.Args[0] == "cl" {
				if cmd.Args[1] == "upload" {
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
			} else if cmd.Args[0] == "clone" {
				if !util.In(child.RepoUrl(), cmd.Args) {
					return nil
				}
			}
		} else if cmd.Name == "gclient" {
			if cmd.Args[0] == "sync" {
				// This needs to be deferred until the repo manager creates the dir,
				// so we run it at "gclient sync".
				dest := path.Join(wd, "repo_manager", "skia")
				if _, err := os.Stat(dest); os.IsNotExist(err) {
					co, err := git.NewCheckout(ctx, child.RepoUrl(), cmd.Dir)
					assert.NoError(t, err)
					assert.NoError(t, os.Rename(co.Dir(), dest))
				}
			}
		}
		return exec.DefaultRun(cmd)
	})

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, cleanup
}

func setupManifestFakeGerrit(t *testing.T, wd string) *gerrit.Gerrit {
	gUrl := "https://fake-skia-review.googlesource.com"
	urlMock := mockhttpclient.NewURLMock()
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	assert.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	serializedIssue, err := json.Marshal(&gerrit.ChangeInfo{
		Issue:    12345,
		ChangeId: "abc",
		Revisions: map[string]*gerrit.Revision{
			"1": &gerrit.Revision{},
		},
	})
	assert.NoError(t, err)
	serializedIssue = append([]byte("abcd\n"), serializedIssue...)
	urlMock.MockOnce(gUrl+"/a/changes/12345/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(serializedIssue))
	urlMock.MockOnce(gUrl+"/a/changes/abc/revisions/1/review", mockhttpclient.MockPostDialogue("application/json", mockhttpclient.DONT_CARE_REQUEST, []byte{}))
	gitcookies := path.Join(wd, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlMock.Client())
	assert.NoError(t, err)
	return g
}

// TestRepoManager tests all aspects of the ManifestRepoManager except for CreateNewRoll.
func TestManifestRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil, "fake.server.com")
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev())

	// Test update.
	lastCommit := child.CommitGen(ctx, "abc.txt")
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, lastCommit, rm.NextRollRev())

	// User, name only.
	assert.Equal(t, mockUser, rm.User())
}

// TestCreateNewManifestRoll tests that CreateNewRoll returns the expected issueNum by mocking out
// the git cl upload call.
func TestCreateNewManifestRoll(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, _, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil, "fake.server.com")
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), manifestEmails, "", false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsManifest(t *testing.T) {
	testutils.LargeTest(t)

	testutils.LargeTest(t)

	ctx, wd, _, _, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil, "fake.server.com")
	assert.NoError(t, err)
	ran := false
	rm.(*manifestRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), manifestEmails, "", false)
	assert.NoError(t, err)
	assert.True(t, ran)
}
