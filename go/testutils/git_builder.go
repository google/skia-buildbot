package testutils

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/satori/go.uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

// GitBuilder creates commits and branches in a git repo.
type GitBuilder struct {
	t      *testing.T
	dir    string
	branch string
}

// GitInit creates a new git repo in a temporary directory and returns a
// GitBuilder to manage it. Call Cleanup to remove the temporary directory. The
// current branch will be master.
func GitInit(t *testing.T) *GitBuilder {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	g := &GitBuilder{
		t:      t,
		dir:    tmp,
		branch: "master",
	}

	g.run("git", "init")
	g.run("git", "remote", "add", "origin", ".")

	return g
}

// Cleanup removes the directory containing the git repo.
func (g *GitBuilder) Cleanup() {
	RemoveAll(g.t, g.dir)
}

// Dir returns the directory of the git repo, e.g. for cloning.
func (g *GitBuilder) Dir() string {
	return g.dir
}

func (g *GitBuilder) run(cmd ...string) {
	_, err := exec.RunCwd(g.dir, cmd...)
	assert.NoError(g.t, err)
}

func (g *GitBuilder) write(filepath, contents string) {
	assert.NoError(g.t, ioutil.WriteFile(path.Join(g.dir, filepath), []byte(contents), os.ModePerm))
}

func (g *GitBuilder) push() {
	g.run("git", "push", "origin", g.branch)
}

// genString returns a string with arbitrary content.
func genString() string {
	return uuid.NewV1().String()
}

// Add writes contents to file and adds it to the index.
func (g *GitBuilder) Add(file, contents string) {
	g.write(file, contents)
	g.run("git", "add", file)
}

// AddGen writes arbitrary content to file and adds it to the index.
func (g *GitBuilder) AddGen(file string) {
	g.Add(file, genString())
}

// CommitMsg commits files in the index with the given commit message. The
// current branch is then pushed.
// TODO(benjaminwagner): Return commit hash.
func (g *GitBuilder) CommitMsg(msg string) {
	g.run("git", "commit", "-m", msg)
	g.push()
}

// Commit commits files in the index. The current branch is then pushed. Uses an
// arbitrary commit message.
// TODO(benjaminwagner): Return commit hash.
func (g *GitBuilder) Commit() {
	g.CommitMsg(genString())
}

// CommitGen commits arbitrary content to the given file. The current branch is
// then pushed.
// TODO(benjaminwagner): Return commit hash.
func (g *GitBuilder) CommitGen(file string) {
	s := genString()
	g.Add(file, s)
	g.CommitMsg(s)
}

// CreateBranchTrackBranch creates a new branch tracking an existing branch,
// checks out the new branch, and pushes the new branch.
func (g *GitBuilder) CreateBranchTrackBranch(newBranch, existingBranch string) {
	g.run("git", "checkout", "-t", "-b", newBranch, existingBranch)
	g.branch = newBranch
	g.push()
}

// CreateBranchTrackBranch creates a new branch pointing at the given commit,
// checks out the new branch, and pushes the new branch.
func (g *GitBuilder) CreateBranchAtCommit(name, commit string) {
	g.run("git", "checkout", "--no-track", "-b", name, commit)
	g.branch = name
	g.push()
}

// CheckoutBranch checks out the given branch.
func (g *GitBuilder) CheckoutBranch(name string) {
	g.run("git", "checkout", name)
	g.branch = name
}

// MergeBranch merges the given branch into the current branch and pushes the
// current branch.
// TODO(benjaminwagner): Return commit hash.
func (g *GitBuilder) MergeBranch(name string) {
	assert.NotEqual(g.t, g.branch, name, "Can't merge a branch into itself.")
	g.run("git", "merge", name)
	g.push()
}

// GitSetup adds commits to the Git repo managed by g.
//
// The repo layout looks like this:
//
// c1--c2------c4--c5--
//       \-c3-----/
func GitSetup(g *GitBuilder) {
	// c1
	g.CommitGen("myfile.txt")
	// c2
	g.CommitGen("myfile.txt")
	// c3
	g.CreateBranchTrackBranch("branch2", "origin/master")
	g.CommitGen("anotherfile.txt")
	// c4
	g.CheckoutBranch("master")
	g.CommitGen("myfile.txt")
	// c5
	g.MergeBranch("branch2")
}
