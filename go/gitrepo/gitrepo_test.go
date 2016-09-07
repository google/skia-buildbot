package gitrepo

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

func run(t *testing.T, dir string, cmd ...string) {
	_, err := exec.RunCwd(dir, cmd...)
	assert.NoError(t, err)
}

func write(t *testing.T, dir, filepath, contents string) {
	assert.NoError(t, ioutil.WriteFile(path.Join(dir, filepath), []byte(contents), os.ModePerm))
}

func commit(t *testing.T, workdir, file string) {
	contents := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	write(t, workdir, file, contents)
	run(t, workdir, "git", "add", file)
	run(t, workdir, "git", "commit", "-m", contents)
}

// gitRepo initializes a Git repo in a temporary directory with some commits.
// Returns the path of the temporary directory, the Repo object associated with
// the repo, and a slice of the commits which were added.
//
// The repo layout looks like this:
//
// c1--c2------c4--c5--
//       \-c3-----/
func gitSetup(t *testing.T) (string, *Repo, []*Commit) {
	testutils.SkipIfShort(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Set up a git repo.
	run(t, tmp, "git", "init")
	run(t, tmp, "git", "remote", "add", "origin", ".")
	commit(t, tmp, "myfile.txt")
	run(t, tmp, "git", "push", "origin", "master")

	repo, err := NewRepo(".", tmp)
	assert.NoError(t, err)

	c1 := repo.Get("master")
	assert.NotNil(t, c1)
	assert.Equal(t, 0, len(c1.GetParents()))

	commit(t, tmp, "myfile.txt")
	run(t, tmp, "git", "push", "origin", "master")
	assert.NoError(t, repo.Update())
	c2 := repo.Get("master")
	assert.NotNil(t, c2)
	assert.Equal(t, 1, len(c2.GetParents()))
	assert.Equal(t, c1, c2.GetParents()[0])
	assert.Equal(t, []string{"master"}, repo.Branches())

	// Create a second branch.
	run(t, tmp, "git", "checkout", "-b", "branch2", "-t", "origin/master")
	commit(t, tmp, "anotherfile.txt")
	run(t, tmp, "git", "push", "origin", "branch2")
	assert.NoError(t, repo.Update())
	c3 := repo.Get("branch2")
	assert.NotNil(t, c3)
	assert.Equal(t, c2, repo.Get("master"))
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())

	// Commit again to master.
	run(t, tmp, "git", "checkout", "master")
	commit(t, tmp, "myfile.txt")
	assert.NoError(t, repo.Update())
	assert.Equal(t, c3, repo.Get("branch2"))
	c4 := repo.Get("master")
	assert.NotNil(t, c4)

	// Merge branch1 into master.
	run(t, tmp, "git", "merge", "branch2")
	assert.NoError(t, repo.Update())
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())
	c5 := repo.Get("master")
	assert.NotNil(t, c5)
	assert.Equal(t, c3, repo.Get("branch2"))

	return tmp, repo, []*Commit{c1, c2, c3, c4, c5}
}

func TestGitRepo(t *testing.T) {
	tmp, repo, commits := gitSetup(t)
	defer testutils.RemoveAll(t, tmp)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	// Trace commits back to the beginning of time.
	assert.Equal(t, []*Commit{c4, c3}, c5.GetParents())
	assert.Equal(t, []*Commit{c2}, c4.GetParents())
	assert.Equal(t, []*Commit{c1}, c2.GetParents())
	assert.Equal(t, []*Commit{}, c1.GetParents())
	assert.Equal(t, []*Commit{c2}, c3.GetParents())

	// Ensure that we can start in an empty dir and check out from scratch properly.
	tmp2, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	repo2, err := NewRepo(tmp, tmp2)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, repo.Branches(), repo2.Branches())
	m1 := repo.Get("master")
	m2 := repo2.Get("master")
	// These will confuse AssertDeepEqual.
	m1.repo = nil
	m2.repo = nil
	testutils.AssertDeepEqual(t, m1, m2)
}

func TestSerialize(t *testing.T) {
	tmp, repo, _ := gitSetup(t)
	defer testutils.RemoveAll(t, tmp)

	glog.Infof("New repo.")
	repo2, err := NewRepo(".", tmp)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, repo, repo2)
}
