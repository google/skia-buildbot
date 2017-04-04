package repo_manager

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
)

const (
	androidIssueNum        = int64(12345)
	mockAndroidServer      = "https://mock-server-review.googlesource.com"
	numAndroidChildCommits = 10
)

var (
	androidEmails = []string{"reviewer@chromium.org"}
	childCommits  = []string{
		"5678888888888888888888888888888888888888",
		"1234444444444444444444444444444444444444"}
)

func setupAndroid(t *testing.T) (string, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "repo") {
			return nil
		}
		if cmd.Name == "git" {
			if cmd.Args[0] == "log" {
				var output string
				if cmd.Args[1] == "--pretty=format:%ae %H" {
					output = fmt.Sprintf("%s else 324\nsometbodyelse %s", SERVICE_ACCOUNT, childCommits[1])
				} else if cmd.Args[1] == "--format=format:%H%x20%ci" {
					output = fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", childCommits[0], childCommits[1])
				}
				n, err := cmd.CombinedOutput.Write([]byte(output))
				assert.NoError(t, err)
				assert.Equal(t, len(output), n)
			} else if cmd.Args[0] == "ls-remote" {
				childHead := childCommits[0]
				n, err := cmd.CombinedOutput.Write([]byte(childHead))
				assert.NoError(t, err)
				assert.Equal(t, len(childHead), n)
			}
		}
		return nil
	})
	exec.SetRunForTesting(mockRun.Run)
	cleanup := func() {
		exec.SetRunForTesting(exec.DefaultRun)
		testutils.RemoveAll(t, wd)
	}
	return wd, cleanup
}

// TestAndroidRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestAndroidRepoManager(t *testing.T) {
	testutils.LargeTest(t)
	wd, cleanup := setupAndroid(t)
	defer cleanup()
	g, err := gerrit.NewGerrit(mockAndroidServer, "", nil)
	assert.NoError(t, err)
	rm, err := NewAndroidRepoManager(wd, childPath, 24*time.Hour, g)
	assert.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s/android_repo/%s", wd, childPath), rm.(*androidRepoManager).childDir)
	assert.Equal(t, "https://mock-server.googlesource.com", rm.(*androidRepoManager).repoUrl)
	assert.Equal(t, childCommits[len(childCommits)-1], rm.LastRollRev())
	assert.Equal(t, childCommits[0], rm.ChildHead())
	assert.Equal(t, SERVICE_ACCOUNT, rm.User())
}

// TestCreateNewAndroidRoll tests creating a new roll.
func TestCreateNewAndroidRoll(t *testing.T) {
	testutils.LargeTest(t)
	wd, cleanup := setupAndroid(t)
	defer cleanup()

	g := &gerrit.MockedGerrit{IssueID: androidIssueNum}
	rm, err := NewAndroidRepoManager(wd, childPath, 24*time.Hour, g)
	assert.NoError(t, err)

	issue, err := rm.CreateNewRoll(ROLL_STRATEGY_BATCH, androidEmails, "", false, true)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
}
