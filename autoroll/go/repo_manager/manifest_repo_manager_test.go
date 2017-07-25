package repo_manager

import (
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
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

var (
	manifestEmails = []string{"reviewer@chromium.org"}
)

func setupManifest(t *testing.T) (string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(f))
	}

	parent := git_testutils.GitInit(t)
	parent.Add(MANIFEST_FILE_NAME, fmt.Sprintf(`
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
	parent.Commit()

	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" {
			var output string
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
			} else if cmd.Args[0] == "rev-parse" {
				output = childCommits[len(childCommits)-1]
				n, err := cmd.CombinedOutput.Write([]byte(output))
				assert.NoError(t, err)
				assert.Equal(t, len(output), n)
			}
			return nil
		}
		return exec.DefaultRun(cmd)
	})
	exec.SetRunForTesting(mockRun.Run)

	cleanup := func() {
		exec.SetRunForTesting(exec.DefaultRun)
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return wd, child, childCommits, parent, cleanup
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

	wd, child, childCommits, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil)
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev())

	// Test update.
	lastCommit := child.CommitGen("abc.txt")
	assert.NoError(t, rm.Update())
	assert.Equal(t, lastCommit, rm.NextRollRev())

	// User, name only.
	assert.Equal(t, mockUser, rm.User())
}

// TestCreateNewManifestRoll tests that CreateNewRoll returns the expected issueNum by mocking out
// the git cl upload call.
func TestCreateNewManifestRoll(t *testing.T) {
	testutils.LargeTest(t)

	wd, _, _, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil)
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(rm.LastRollRev(), rm.NextRollRev(), manifestEmails, "", false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsManifest(t *testing.T) {
	testutils.LargeTest(t)

	testutils.LargeTest(t)

	wd, _, _, parent, cleanup := setupManifest(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupManifestFakeGerrit(t, wd)
	rm, err := NewManifestRepoManager(wd, parent.RepoUrl(), "master", childPath, "master", depotTools, g, s, nil)
	assert.NoError(t, err)
	ran := false
	rm.(*manifestRepoManager).preUploadSteps = []PreUploadStep{
		func(string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(rm.LastRollRev(), rm.NextRollRev(), manifestEmails, "", false)
	assert.NoError(t, err)
	assert.True(t, ran)
}
