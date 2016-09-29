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

type GitDir struct {
	t      *testing.T
	dir    string
	branch string
}

func GitInit(t *testing.T) *GitDir {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	g := &GitDir{
		t:      t,
		dir:    tmp,
		branch: "master",
	}

	g.run("git", "init")
	g.run("git", "remote", "add", "origin", ".")

	return g
}

func (g *GitDir) Cleanup() {
	RemoveAll(g.t, g.dir)
}

func (g *GitDir) Dir() string {
	return g.dir
}

func (g *GitDir) run(cmd ...string) {
	_, err := exec.RunCwd(g.dir, cmd...)
	assert.NoError(g.t, err)
}

func (g *GitDir) write(filepath, contents string) {
	assert.NoError(g.t, ioutil.WriteFile(path.Join(g.dir, filepath), []byte(contents), os.ModePerm))
}

func genString() string {
	return uuid.NewV1().String()
}

func (g *GitDir) Add(file, contents string) {
	g.write(file, contents)
	g.run("git", "add", file)
}

func (g *GitDir) AddGen(file string) {
	g.Add(file, genString())
}

// TODO(benjaminwagner): Return commit hash.
func (g *GitDir) CommitMsg(msg string) {
	g.run("git", "commit", "-m", msg)
	g.run("git", "push", "origin", g.branch)
}

// TODO(benjaminwagner): Return commit hash.
func (g *GitDir) Commit() {
	g.CommitMsg(genString())
}

// TODO(benjaminwagner): Return commit hash.
func (g *GitDir) CommitGen(file string) {
	s := genString()
	g.Add(file, s)
	g.CommitMsg(s)
}

func (g *GitDir) CreateBranchTrackCurrent(name string) {
	g.run("git", "checkout", "-t", "-b", name)
	g.branch = name
	g.run("git", "push", "origin", g.branch)
}

func (g *GitDir) CreateBranchAtCommit(name, commit string) {
	g.run("git", "checkout", "--no-track", "-b", name, commit)
	g.branch = name
	g.run("git", "push", "origin", g.branch)
}

func (g *GitDir) CheckoutBranch(name string) {
	g.run("git", "checkout", name)
	g.branch = name
}

// TODO(benjaminwagner): Return commit hash.
func (g *GitDir) MergeBranch(name string) {
	assert.NotEqual(g.t, g.branch, name, "Can't merge a branch into itself.")
	g.run("git", "merge", name)
	g.run("git", "push", "origin", g.branch)
}

// GitSetup adds commits to the Git repo managed by g.
//
// The repo layout looks like this:
//
// c1--c2------c4--c5--
//       \-c3-----/
func GitSetup(g *GitDir) {
	// c1
	g.CommitGen("myfile.txt")
	// c2
	g.CommitGen("myfile.txt")
	// c3
	g.CreateBranchTrackCurrent("branch2")
	g.CommitGen("anotherfile.txt")
	// c4
	g.CheckoutBranch("master")
	g.CommitGen("myfile.txt")
	// c5
	g.MergeBranch("branch2")
}
