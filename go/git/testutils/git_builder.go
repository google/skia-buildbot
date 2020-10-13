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

	"github.com/stretchr/testify/require"
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
	git    string
	rng    *rand.Rand
}

// GitInit creates a new git repo in a temporary directory and returns a
// GitBuilder to manage it. Call Cleanup to remove the temporary directory. The
// current branch will be the main branch.
func GitInit(t sktest.TestingT, ctx context.Context) *GitBuilder {
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	return GitInitWithDir(t, ctx, tmp)
}

// GitInit creates a new git repo in the specified directory and returns a
// GitBuilder to manage it. Call Cleanup to remove the temporary directory. The
// current branch will be the main branch.
func GitInitWithDir(t sktest.TestingT, ctx context.Context, dir string) *GitBuilder {
	gitExec, _, _, err := git_common.FindGit(ctx)
	require.NoError(t, err)

	g := &GitBuilder{
		t:      t,
		dir:    dir,
		branch: git_common.DefaultBranch,
		git:    gitExec,
		rng:    rand.New(rand.NewSource(0)),
	}

	g.Git(ctx, "init")
	// Set the initial branch.
	//
	// It is important to set the initial branch explicitly because developer workstations might have
	// a value for Git option init.defaultBranch[1] that differs from that of the CQ bots. This can
	// cause tests that use GitBuilder to fail locally but pass on the CQ (or vice versa).
	//
	// [1] https://git-scm.com/docs/git-config#Documentation/git-config.txt-initdefaultBranch
	//
	// TODO(lovisolo): Replace with "git init --initial-branch <git_common.DefaultBranch>" once all
	//                 GCE instances have been upgraded to Git >= v2.28, which introduces flag
	//                 --initial-branch.
	//                 See https://github.com/git/git/commit/32ba12dab2acf1ad11836a627956d1473f6b851a.
	g.Git(ctx, "symbolic-ref", "HEAD", "refs/heads/"+g.branch)
	g.Git(ctx, "remote", "add", git_common.DefaultRemote, ".")
	g.Git(ctx, "config", "--local", "user.name", "test")
	g.Git(ctx, "config", "--local", "user.email", "test@google.com")
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

func (g *GitBuilder) Git(ctx context.Context, cmd ...string) string {
	output, err := exec.RunCwd(ctx, g.dir, append([]string{g.git}, cmd...)...)
	require.NoError(g.t, err)
	return output
}

func (g *GitBuilder) runCommand(ctx context.Context, cmd *exec.Command) string {
	cmd.InheritEnv = true
	cmd.Dir = g.dir
	output, err := exec.RunCommand(ctx, cmd)
	require.NoError(g.t, err)
	return output
}

func (g *GitBuilder) write(filepath, contents string) {
	fullPath := path.Join(g.dir, filepath)
	dir := path.Dir(fullPath)
	if dir != "" {
		require.NoError(g.t, os.MkdirAll(dir, os.ModePerm))
	}
	require.NoError(g.t, ioutil.WriteFile(fullPath, []byte(contents), os.ModePerm))
}

func (g *GitBuilder) push(ctx context.Context) {
	g.Git(ctx, "push", git_common.DefaultRemote, g.branch)
}

// genString returns a string with arbitrary content.
func (g *GitBuilder) genString() string {
	return fmt.Sprintf("%d", g.rng.Int())
}

// Add writes contents to file and adds it to the index.
func (g *GitBuilder) Add(ctx context.Context, file, contents string) {
	g.write(file, contents)
	g.Git(ctx, "add", file)
}

// AddGen writes arbitrary content to file and adds it to the index.
func (g *GitBuilder) AddGen(ctx context.Context, file string) {
	g.Add(ctx, file, g.genString())
}

func (g *GitBuilder) lastCommitHash(ctx context.Context) string {
	return strings.TrimSpace(g.Git(ctx, "rev-parse", "HEAD"))
}

// CommitMsg commits files in the index with the given commit message using the
// given time as the commit time. The current branch is then pushed.
// Note that the nanosecond component of time will be dropped. Returns the hash
// of the new commit.
func (g *GitBuilder) CommitMsgAt(ctx context.Context, msg string, time time.Time) string {
	g.runCommand(ctx, &exec.Command{
		Name: g.git,
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
	g.Git(ctx, "checkout", "-b", newBranch, "-t", existingBranch)
	g.branch = newBranch
	g.push(ctx)
}

// CreateBranchTrackBranch creates a new branch pointing at the given commit,
// checks out the new branch, and pushes the new branch.
func (g *GitBuilder) CreateBranchAtCommit(ctx context.Context, name, commit string) {
	g.Git(ctx, "checkout", "--no-track", "-b", name, commit)
	g.branch = name
	g.push(ctx)
}

// CreateOrphanBranch creates a new orphan branch.
func (g *GitBuilder) CreateOrphanBranch(ctx context.Context, newBranch string) {
	g.Git(ctx, "checkout", "--orphan", newBranch)
	g.branch = newBranch
	// Can't push, since the branch doesn't currently point to any commit.
}

// CheckoutBranch checks out the given branch.
func (g *GitBuilder) CheckoutBranch(ctx context.Context, name string) {
	g.Git(ctx, "checkout", name)
	g.branch = name
}

// MergeBranchAt merges the given branch into the current branch at the given
// time and pushes the current branch. Returns the hash of the new commit.
func (g *GitBuilder) MergeBranchAt(ctx context.Context, name string, ts time.Time) string {
	require.NotEqual(g.t, g.branch, name, "Can't merge a branch into itself.")
	args := []string{"merge", name}
	_, major, minor, err := git_common.FindGit(ctx)
	require.NoError(g.t, err)
	if (major == 2 && minor >= 9) || major > 2 {
		args = append(args, "--allow-unrelated-histories")
	}
	g.runCommand(ctx, &exec.Command{
		Name: g.git,
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
	cmd := append([]string{"reset"}, args...)
	g.Git(ctx, cmd...)
	g.push(ctx)
}

// UpdateRef runs "git update-ref" in the repo.
func (g *GitBuilder) UpdateRef(ctx context.Context, args ...string) {
	cmd := append([]string{"update-ref"}, args...)
	g.Git(ctx, cmd...)
	g.push(ctx)
}

// CreateFakeGerritCLGen creates a Gerrit-like ref so that it can be applied like
// a CL on a trybot.
func (g *GitBuilder) CreateFakeGerritCLGen(ctx context.Context, issue, patchset string) {
	currentBranch := strings.TrimSpace(g.Git(ctx, "rev-parse", "--abbrev-ref", "HEAD"))
	g.CreateBranchTrackBranch(ctx, "fake-patch", git_common.DefaultBranch)
	patchCommit := g.CommitGen(ctx, "somefile")
	g.UpdateRef(ctx, fmt.Sprintf("refs/changes/%s/%s/%s", issue[len(issue)-2:], issue, patchset), patchCommit)
	g.CheckoutBranch(ctx, currentBranch)
	g.Git(ctx, "branch", "-D", "fake-patch")
}

// AcceptPushes allows pushing changes to the repo.
func (g *GitBuilder) AcceptPushes(ctx context.Context) {
	// TODO(lovisolo): Consider making GitBuilder point to a bare repository (git init --bare).
	// Under this scenario, GitBuilder would push to that bare repository, and GitBuilder.RepoUrl()
	// would return the URL for the bare repository. This would remove the need for this method.

	g.Git(ctx, "config", "receive.denyCurrentBranch", "ignore")
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
	g.CreateBranchTrackBranch(ctx, "branch2", git_common.DefaultRemoteBranch)
	c2 := g.CommitGen(ctx, "anotherfile.txt")
	g.CheckoutBranch(ctx, git_common.DefaultBranch)
	c3 := g.CommitGen(ctx, "myfile.txt")
	c4 := g.MergeBranch(ctx, "branch2")
	return []string{c0, c1, c2, c3, c4}
}
