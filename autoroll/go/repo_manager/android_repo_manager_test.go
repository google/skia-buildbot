package repo_manager

import (
	"fmt"
	assert "github.com/stretchr/testify/require"
	//"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	//"go.skia.org/infra/go/git"
	//git_testutils "go.skia.org/infra/go/git/testutils"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/testutils"
	"io/ioutil"
	//"path"
	"strings"
	"testing"
	"time"
)

const (
	//childPath       = "path/to/child"
	androidIssueNum        = int64(12345)
	mockAndroidServer      = "https://mock-server-review.googlesource.com"
	numAndroidChildCommits = 10
)

var (
	androidEmails = []string{"reviewer@chromium.org"}
	// TODO(rmistry): Add more commits to this list!
	childCommits = []string{
		"5678888888888888888888888888888888888888",
		"1234444444444444444444444444444444444444"}
)

func setupAndroid(t *testing.T) (string, []string, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	//// Create child and parent repos.
	//child := git_testutils.GitInit(t)
	//f := "somefile.txt"
	//childCommits := make([]string, 0, 10)
	//for i := 0; i < numChildCommits; i++ {
	//	childCommits = append(childCommits, child.CommitGen(f))
	//}
	//parent := git_testutils.GitInit(t)
	//parent.Add("DEPS", fmt.Sprintf(`deps = {
	// "%s": "%s@%s",
	//}`, childPath, child.RepoUrl(), childCommits[0]))
	//parent.Commit()
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		fmt.Println("HERE HERE HERE")
		fmt.Println(cmd.Name)
		fmt.Println(cmd.Args)
		if strings.Contains(cmd.Name, "repo") {
			fmt.Println("Skipping REPO commands")
			return nil
		}
		if cmd.Name == "git" {
			if cmd.Args[0] == "log" {
				if cmd.Args[1] == "--pretty=format:%ae %H" {
					fmt.Println("PRETTY FORMAT")
					lastRollRev := fmt.Sprintf("%s else 324\nsometbodyelse %s", SERVICE_ACCOUNT, childCommits[1])
					n, err := cmd.CombinedOutput.Write([]byte(lastRollRev))
					assert.NoError(t, err)
					assert.Equal(t, len(lastRollRev), n)
				} else if cmd.Args[1] == "--format=format:%H%x20%ci" {
					fmt.Println("REGULAR FORMAT")
					childCommits := fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", childCommits[0], childCommits[1])
					n, err := cmd.CombinedOutput.Write([]byte(childCommits))
					assert.NoError(t, err)
					assert.Equal(t, len(childCommits), n)
				}
			} else if cmd.Args[0] == "ls-remote" {
				childHead := childCommits[0]
				n, err := cmd.CombinedOutput.Write([]byte(childHead))
				assert.NoError(t, err)
				assert.Equal(t, len(childHead), n)
			}
		}
		return nil
		if strings.Contains(cmd.Name, "depot-tools-auth") {
			auth := fmt.Sprintf(`Logged in to %s as %s.
		To login with a different email run:
		depot-tools-auth login https://codereview.chromium.org
		To logout and purge the authentication token run:
		depot-tools-auth logout https://codereview.chromium.org
		`, mockServer, mockUser)
			n, err := cmd.CombinedOutput.Write([]byte(auth))
			assert.NoError(t, err)
			assert.Equal(t, len(auth), n)
			return nil
		} else if cmd.Name == "git" && cmd.Args[0] == "cl" {
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
		}
		return exec.DefaultRun(cmd)
	})
	exec.SetRunForTesting(mockRun.Run)
	cleanup := func() {
		exec.SetRunForTesting(exec.DefaultRun)
		testutils.RemoveAll(t, wd)
	}
	return wd, childCommits, cleanup
}

// TestAndroidRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestAndroidRepoManager(t *testing.T) {
	testutils.LargeTest(t)
	wd, childCommits, cleanup := setupAndroid(t)
	defer cleanup()
	g, err := gerrit.NewGerrit(mockAndroidServer, "", nil)
	assert.NoError(t, err)
	rm, err := NewAndroidRepoManager(wd, childPath, 24*time.Hour, g)
	assert.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s/android_repo/%s", wd, childPath), rm.(*androidRepoManager).childDir)
	assert.Equal(t, "https://mock-server.googlesource.com", rm.(*androidRepoManager).repoUrl)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.ChildHead())
	assert.Equal(t, SERVICE_ACCOUNT, rm.User())
}

type MockedGerrit struct {
	mock.Mock
}

func (g *MockedGerrit) TurnOnAuthenticatedGets() {
}
func (g *MockedGerrit) Url(issueID int64) string {
	return ""
}
func (g *MockedGerrit) ExtractIssue(issueURL string) (string, bool) {
	return "", false
}
func (g *MockedGerrit) GetIssueProperties(issue int64) (*ChangeInfo, error) {
	return nil, nil
}
func (g *MockedGerrit) GetPatch(issue int64, revision string) (string, error) {
	return "", nil
}
func (g *MockedGerrit) SetReview(issue *ChangeInfo, message string, labels map[string]interface{}) error {
	return nil
}
func (g *MockedGerrit) AddComment(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SendToDryRun(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SendToCQ(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) RemoveFromCQ(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) Approve(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) NoScore(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) DisApprove(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) Abandon(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SetTopic(topic string, changeNum int64) error {
	return nil
}
func (g *MockedGerrit) Search(limit int, terms ...*SearchTerm) ([]*ChangeInfo, error) {
	return nil, nil
}
func (g *MockedGerrit) GetTrybotResults(issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return nil, nil
}

func testCreateNewAndroidRoll(t *testing.T, strategy string, expectIdx int) {
	testutils.LargeTest(t)
	wd, childCommits, cleanup := setupAndroid(t)
	fmt.Println(childCommits)
	defer cleanup()
	g, err := gerrit.NewGerrit(mockAndroidServer, "", nil)
	assert.NoError(t, err)
	rm, err := NewAndroidRepoManager(wd, childPath, 24*time.Hour, g)
	assert.NoError(t, err)

	// rmistry: LEFT OVER HERE!!!
	// https://github.com/stretchr/testify#mock-package for Gerrit!!

	// Create a roll, assert that it's at tip of tree.
	// (strategy string, emails []string, cqExtraTrybots string, dryRun, gerrit bool)
	issue, err := rm.CreateNewRoll(strategy, androidEmails, "", false, true)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	//msg, err := ioutil.ReadFile(path.Join(rm.parentDir, ".git", "COMMIT_EDITMSG"))
	//assert.NoError(t, err)
	//from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
	//	return git.GitDir(child.Dir()).RevParse(h)
	//})
	//assert.NoError(t, err)
	//assert.Equal(t, childCommits[0], from)
	//assert.Equal(t, childCommits[expectIdx], to)
}

// TestAndroidRepoManagerBatch tests the batch roll strategy.
func TestAndroidRepoManagerBatch(t *testing.T) {
	testCreateNewAndroidRoll(t, ROLL_STRATEGY_BATCH, numChildCommits-1)
}

// TestAndroidRepoManagerSingle tests the single-commit roll strategy.
func _TestAndroidRepoManagerSingle(t *testing.T) {
	testCreateNewAndroidRoll(t, ROLL_STRATEGY_SINGLE, 1)
}
