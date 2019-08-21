package testutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
)

// GitBuilder creates commits and branches in a git repo.
type GitBuilder struct {
	t      sktest.TestingT
	dir    string
	branch string
	rng    *rand.Rand
}

// GitInit creates a new git repo in a temporary directory and returns a
// GitBuilder to manage it. Call Cleanup to remove the temporary directory. The
// current branch will be master.
func GitInit(t sktest.TestingT, ctx context.Context) *GitBuilder {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	return GitInitWithDir(t, ctx, tmp)
}

// GitInit creates a new git repo in the specified directory and returns a
// GitBuilder to manage it. Call Cleanup to remove the temporary directory. The
// current branch will be master.
func GitInitWithDir(t sktest.TestingT, ctx context.Context, dir string) *GitBuilder {
	g := &GitBuilder{
		t:      t,
		dir:    dir,
		branch: "master",
		rng:    rand.New(rand.NewSource(0)),
	}

	g.run(ctx, "git", "init")
	g.run(ctx, "git", "remote", "add", "origin", ".")
	g.run(ctx, "git", "config", "--local", "user.name", "test")
	g.run(ctx, "git", "config", "--local", "user.email", "test@google.com")
	return g
}

// Cleanup removes the directory containing the git repo.
func (g *GitBuilder) Cleanup() {
	testutils.RemoveAll(g.t, g.dir)
}

// Dir returns the directory of the git repo, e.g. for cloning.
func (g *GitBuilder) Dir() string {
	return g.dir
}

// RepoUrl returns a git-friendly URL for the repo.
func (g *GitBuilder) RepoUrl() string {
	return fmt.Sprintf("file://%s", g.Dir())
}

// Seed replaces the random seed used by the GitBuilder.
func (g *GitBuilder) Seed(seed int64) {
	g.rng.Seed(seed)
}

func (g *GitBuilder) run(ctx context.Context, cmd ...string) string {
	output, err := exec.RunCwd(ctx, g.dir, cmd...)
	assert.NoError(g.t, err)
	return output
}

func (g *GitBuilder) runCommand(ctx context.Context, cmd *exec.Command) string {
	cmd.InheritEnv = true
	cmd.Dir = g.dir
	output, err := exec.RunCommand(ctx, cmd)
	assert.NoError(g.t, err)
	return output
}

func (g *GitBuilder) write(filepath, contents string) {
	fullPath := path.Join(g.dir, filepath)
	dir := path.Dir(fullPath)
	if dir != "" {
		assert.NoError(g.t, os.MkdirAll(dir, os.ModePerm))
	}
	assert.NoError(g.t, ioutil.WriteFile(fullPath, []byte(contents), os.ModePerm))
}

func (g *GitBuilder) push(ctx context.Context) {
	g.run(ctx, "git", "push", "origin", g.branch)
}

// genString returns a string with arbitrary content.
func (g *GitBuilder) genString() string {
	return fmt.Sprintf("%d", g.rng.Int())
}

// Add writes contents to file and adds it to the index.
func (g *GitBuilder) Add(ctx context.Context, file, contents string) {
	g.write(file, contents)
	g.run(ctx, "git", "add", file)
}

// AddGen writes arbitrary content to file and adds it to the index.
func (g *GitBuilder) AddGen(ctx context.Context, file string) {
	g.Add(ctx, file, g.genString())
}

func (g *GitBuilder) lastCommitHash(ctx context.Context) string {
	return strings.TrimSpace(g.run(ctx, "git", "rev-parse", "HEAD"))
}

// CommitMsg commits files in the index with the given commit message using the
// given time as the commit time. The current branch is then pushed.
// Note that the nanosecond component of time will be dropped. Returns the hash
// of the new commit.
func (g *GitBuilder) CommitMsgAt(ctx context.Context, msg string, time time.Time) string {
	g.runCommand(ctx, &exec.Command{
		Name: "git",
		Args: []string{"commit", "-m", msg},
		Env:  []string{fmt.Sprintf("GIT_AUTHOR_DATE=%d +0000", time.Unix()), fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", time.Unix())},
	})
	g.push(ctx)
	return g.lastCommitHash(ctx)
}

// CommitMsg commits files in the index with the given commit message. The
// current branch is then pushed. Returns the hash of the new commit.
func (g *GitBuilder) CommitMsg(ctx context.Context, msg string) string {
	return g.CommitMsgAt(ctx, msg, time.Now())
}

// Commit commits files in the index. The current branch is then pushed. Uses an
// arbitrary commit message. Returns the hash of the new commit.
func (g *GitBuilder) Commit(ctx context.Context) string {
	return g.CommitMsg(ctx, g.genString())
}

// CommitGen commits arbitrary content to the given file. The current branch is
// then pushed. Returns the hash of the new commit.
func (g *GitBuilder) CommitGen(ctx context.Context, file string) string {
	s := g.genString()
	g.Add(ctx, file, s)
	return g.CommitMsg(ctx, s)
}

// CommitGenAt commits arbitrary content to the given file using the given time
// as the commit time. Note that the nanosecond component of time will be
// dropped. Returns the hash of the new commit.
func (g *GitBuilder) CommitGenAt(ctx context.Context, file string, ts time.Time) string {
	g.AddGen(ctx, file)
	return g.CommitMsgAt(ctx, g.genString(), ts)
}

// CommitGenMsg commits arbitrary content to the given file and uses the given
// commit message. The current branch is then pushed. Returns the hash of the
// new commit.
func (g *GitBuilder) CommitGenMsg(ctx context.Context, file, msg string) string {
	g.AddGen(ctx, file)
	return g.CommitMsg(ctx, msg)
}

// CreateBranchTrackBranch creates a new branch tracking an existing branch,
// checks out the new branch, and pushes the new branch.
func (g *GitBuilder) CreateBranchTrackBranch(ctx context.Context, newBranch, existingBranch string) {
	g.run(ctx, "git", "checkout", "-b", newBranch, "-t", existingBranch)
	g.branch = newBranch
	g.push(ctx)
}

// CreateBranchTrackBranch creates a new branch pointing at the given commit,
// checks out the new branch, and pushes the new branch.
func (g *GitBuilder) CreateBranchAtCommit(ctx context.Context, name, commit string) {
	g.run(ctx, "git", "checkout", "--no-track", "-b", name, commit)
	g.branch = name
	g.push(ctx)
}

// CreateOrphanBranch creates a new orphan branch.
func (g *GitBuilder) CreateOrphanBranch(ctx context.Context, newBranch string) {
	g.run(ctx, "git", "checkout", "--orphan", newBranch)
	g.branch = newBranch
	// Can't push, since the branch doesn't currently point to any commit.
}

// CheckoutBranch checks out the given branch.
func (g *GitBuilder) CheckoutBranch(ctx context.Context, name string) {
	g.run(ctx, "git", "checkout", name)
	g.branch = name
}

// MergeBranchAt merges the given branch into the current branch at the given
// time and pushes the current branch. Returns the hash of the new commit.
func (g *GitBuilder) MergeBranchAt(ctx context.Context, name string, ts time.Time) string {
	assert.NotEqual(g.t, g.branch, name, "Can't merge a branch into itself.")
	args := []string{"merge", name}
	major, minor, err := git_common.Version(ctx)
	assert.NoError(g.t, err)
	if (major == 2 && minor >= 9) || major > 2 {
		args = append(args, "--allow-unrelated-histories")
	}
	g.runCommand(ctx, &exec.Command{
		Name: "git",
		Args: args,
		Env:  []string{fmt.Sprintf("GIT_AUTHOR_DATE=%d +0000", ts.Unix()), fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", ts.Unix())},
	})
	g.push(ctx)
	return g.lastCommitHash(ctx)
}

// MergeBranch merges the given branch into the current branch and pushes the
// current branch. Returns the hash of the new commit.
func (g *GitBuilder) MergeBranch(ctx context.Context, name string) string {
	return g.MergeBranchAt(ctx, name, time.Now())
}

// Reset runs "git reset" in the repo.
func (g *GitBuilder) Reset(ctx context.Context, args ...string) {
	cmd := append([]string{"git", "reset"}, args...)
	g.run(ctx, cmd...)
	g.push(ctx)
}

// UpdateRef runs "git update-ref" in the repo.
func (g *GitBuilder) UpdateRef(ctx context.Context, args ...string) {
	cmd := append([]string{"git", "update-ref"}, args...)
	g.run(ctx, cmd...)
	g.push(ctx)
}

// CreateFakeGerritCLGen creates a Gerrit-like ref so that it can be applied like
// a CL on a trybot.
func (g *GitBuilder) CreateFakeGerritCLGen(ctx context.Context, issue, patchset string) {
	currentBranch := strings.TrimSpace(g.run(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD"))
	g.CreateBranchTrackBranch(ctx, "fake-patch", "master")
	patchCommit := g.CommitGen(ctx, "somefile")
	g.UpdateRef(ctx, fmt.Sprintf("refs/changes/%s/%s/%s", issue[len(issue)-2:], issue, patchset), patchCommit)
	g.CheckoutBranch(ctx, currentBranch)
	g.run(ctx, "git", "branch", "-D", "fake-patch")
}

// GitSetup adds commits to the Git repo managed by g.
//
// The repo layout looks like this:
//
// older           newer
// c0--c1------c3--c4--
//       \-c2-----/
//
// Returns the commit hashes in order from c0-c4.
func GitSetup(ctx context.Context, g *GitBuilder) []string {
	c0 := g.CommitGen(ctx, "myfile.txt")
	c1 := g.CommitGen(ctx, "myfile.txt")
	g.CreateBranchTrackBranch(ctx, "branch2", "origin/master")
	c2 := g.CommitGen(ctx, "anotherfile.txt")
	g.CheckoutBranch(ctx, "master")
	c3 := g.CommitGen(ctx, "myfile.txt")
	c4 := g.MergeBranch(ctx, "branch2")
	return []string{c0, c1, c2, c3, c4}
}
