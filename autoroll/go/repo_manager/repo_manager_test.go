package repo_manager

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

// mockCmd is a struct used for faking RepoManager commands.
type mockCmd struct {
	*exec_testutils.MockCmd
}

// newMockCmd returns a mockCmd instance.
func newMockCmd(t *testing.T) *mockCmd {
	return &mockCmd{
		exec_testutils.NewMockCmd(t),
	}
}

// mockDepotToolsAuth adds a mock for the depot-tools-auth command.
func (c *mockCmd) mockDepotToolsAuth(depotTools string) {
	mockServer := "https://codereview.chromium.org"
	mockUser := "user@chromium.org"
	c.Mock(&exec.Command{
		Name:           "depot-tools-auth",
		Args:           []string{"info", mockServer},
		Env:            getEnv(depotTools),
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, fmt.Sprintf(`Logged in to %s as %s.

To login with a different email run:
  depot-tools-auth login https://codereview.chromium.org
To logout and purge the authentication token run:
  depot-tools-auth logout https://codereview.chromium.org`, mockServer, mockUser), nil)
}

// mockGclientConfig adds a mock for the "gclient config" command.
func (c *mockCmd) mockGclientConfig(repo, cwd, depotTools string) {
	c.Mock(&exec.Command{
		Name:           "gclient",
		Args:           []string{"config", repo},
		Env:            getEnv(depotTools),
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, "", nil)
}

// mockGclientRevInfo adds a mock for the "gclient revinfo" command.
func (c *mockCmd) mockGclientRevInfo(cwd string, revs map[string]string) {
	output := ""
	for k, v := range revs {
		output += fmt.Sprintf("%s: %s\n", k, v)
	}
	c.Mock(&exec.Command{
		Name:           "gclient",
		Args:           []string{"revinfo"},
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, output, nil)
}

// mockGclientSync adds a mock for the "gclient sync" command.
func (c *mockCmd) mockGclientSync(cwd, depotTools string) {
	c.Mock(&exec.Command{
		Name:           "gclient",
		Args:           []string{"sync", "--nohooks"},
		Env:            getEnv(depotTools),
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, "", nil)
}

// mockGitShowRef adds a mock for the "git show-ref" command.
func (c *mockCmd) mockGitShowRef(cwd string) {
	c.Mock(&exec.Command{
		Name:           "git",
		Args:           []string{"show-ref"},
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, "", nil)
}

// mockGitRevList adds a mock for the "git rev-list" command.
func (c *mockCmd) mockGitRevList(cwd string, revs []string) {
	c.Mock(&exec.Command{
		Name:           "git",
		Args:           []string{"rev-list", "--max-parents=0", "HEAD"},
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, strings.Join(revs, "\n"), nil)
}

// mockGitRevParse adds a mock for the "git rev-parse" command.
func (c *mockCmd) mockGitRevParse(cwd, result string) {
	c.Mock(&exec.Command{
		Name:           "git",
		Args:           []string{"rev-parse", "origin/master^{commit}"},
		Dir:            cwd,
		CombinedOutput: &bytes.Buffer{},
		Verbose:        2,
	}, result, nil)
}

// mockUpdate adds mocks for all of the commands run by RepoManager.update()
func (c *mockCmd) mockUpdate(workdir, depotTools, parent, child, childHead, lastRollRev string) {
	repoManagerDir := path.Join(workdir, "repo_manager")
	childDir := path.Join(repoManagerDir, child)
	parentDir := path.Join(repoManagerDir, strings.TrimSuffix(parent, ".git"))

	c.mockGclientConfig(parent, repoManagerDir, depotTools)
	c.mockGclientSync(repoManagerDir, depotTools)
	//c.mockGitShowRef(childDir)
	//c.mockGitRevList(childDir)
	c.mockGclientRevInfo(parentDir, map[string]string{child: fmt.Sprintf("child.git@%s", lastRollRev)})
	c.mockGitRevParse(childDir, childHead)
}

// mockNewRepoManager adds mocks for all of the commands run by
// NewDefaultRepoManager.
func (c *mockCmd) mockNewRepoManager(workdir, depotTools, parent, child, childHead, lastRollRev string) {
	repoManagerDir := path.Join(workdir, "repo_manager")
	childDir := path.Join(repoManagerDir, child)
	parentDir := path.Join(repoManagerDir, strings.TrimSuffix(parent, ".git"))

	c.mockDepotToolsAuth(depotTools)
	c.mockGclientConfig(parent, repoManagerDir, depotTools)
	c.mockGclientSync(repoManagerDir, depotTools)
	c.mockGitShowRef(childDir)
	c.mockGitRevList(childDir, []string{})
	c.mockGclientRevInfo(parentDir, map[string]string{child: fmt.Sprintf("child.git@%s", lastRollRev)})
	c.mockGitRevParse(childDir, childHead)
}

func setup(t *testing.T) (string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		childCommits = append(childCommits, child.CommitGen(f))
	}

	parent := git_testutils.GitInit(t)
	parent.Add("DEPS", fmt.Sprintf(`deps = {
  "path/to/child": "%s@%s",
}`, child.RepoUrl(), childCommits[0]))
	parent.Commit()

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return wd, child, childCommits, parent, cleanup
}

// TestRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	wd, child, childCommits, parent, cleanup := setup(t)
	defer cleanup()

	depotTools := ""
	childPath := "path/to/child"

	rm, err := NewDefaultRepoManager(wd, parent.RepoUrl(), childPath, 24*time.Hour, depotTools)
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.ChildHead())

	// Test FullChildHash.
	for _, c := range childCommits {
		h, err := rm.FullChildHash(c[:12])
		assert.NoError(t, err)
		assert.Equal(t, c, h)
	}

	// Test update.
	lastCommit := child.CommitGen("abc.txt")
	assert.NoError(t, rm.ForceUpdate())
	assert.Equal(t, lastCommit, rm.ChildHead())

	// RolledPast.
	rp, err := rm.RolledPast(childCommits[0])
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		rp, err := rm.RolledPast(c)
		assert.NoError(t, err)
		assert.False(t, rp)
	}
}
