package git_common

/*
   Common elements used by go/git and go/git/testutils.
*/

import (
	"context"
	"fmt"
	osexec "os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Apps will gradually migrate to using MainBranch instead of
	// MasterBranch as more repositories use MainBranch (skbug.com/11842).
	MasterBranch = "master"
	MainBranch   = "main"
	// DefaultRef is the fully-qualified ref name of the default branch for most
	// repositories.
	DefaultRef = RefsHeadsPrefix + MainBranch
	// DefaultRemote is the name of the default remote repository.
	DefaultRemote = "origin"
	// DefaultRemoteBranch is the name of the default branch in the default
	// remote repository, for most repos.
	DefaultRemoteBranch = DefaultRemote + "/" + MainBranch
	// RefsHeadsPrefix is the "refs/heads/" prefix used for branches.
	RefsHeadsPrefix = "refs/heads/"

	gitPathFinderKey contextKeyType = "GitPathFinder"
)

var (
	gitVersionRegex = regexp.MustCompile("git version (\\d+)\\.(\\d+)\\..*")
	gitVersionMajor = 0
	gitVersionMinor = 0
	git             = ""
	mtx             sync.Mutex // Protects git, gitVersionMajor, and gitVersionMinor.
)

type contextKeyType string

func findGitPath(ctx context.Context) (string, error) {
	gitPath, err := osexec.LookPath("git")
	return gitPath, skerr.Wrap(err)
}

// WithGitFinder overrides how the git_common.FindGit() function locates the git executable.
// By default, it looks on the PATH, but this can allow other behavior. The primary case is tests,
// where we load in a hermetic version of git.
func WithGitFinder(ctx context.Context, finder func() (string, error)) context.Context {
	return context.WithValue(ctx, gitPathFinderKey, finder)
}

// FindGit returns the path to the Git executable and the major and minor
// version numbers, or any error which occurred.
func FindGit(ctx context.Context) (string, int, int, error) {
	mtx.Lock()
	defer mtx.Unlock()
	gitPath, err := findGitPath(ctx)
	if err != nil {
		return "", 0, 0, skerr.Wrapf(err, "Failed to find git")
	}
	maj, min, err := Version(ctx, gitPath)
	if err != nil {
		return "", 0, 0, skerr.Wrapf(err, "Failed to obtain git version")
	}
	sklog.Infof("Git is %s; version %d.%d", gitPath, maj, min)
	git = gitPath
	gitVersionMajor = maj
	gitVersionMinor = min
	return git, gitVersionMajor, gitVersionMinor, nil
}

// Version returns the installed Git version, in the form:
// (major, minor), or an error if it could not be determined.
func Version(ctx context.Context, git string) (int, int, error) {
	out, err := exec.RunCwd(ctx, ".", git, "--version")
	if err != nil {
		return -1, -1, err
	}
	m := gitVersionRegex.FindStringSubmatch(out)
	if m == nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	if len(m) != 3 {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	return major, minor, nil
}

// MocksForFindGit returns a DelegateRun func which can be passed to
// exec.CommandCollector.SetDelegateRun so that FindGit will succeed when calls
// to exec are fully mocked out.
func MocksForFindGit(ctx context.Context, cmd *exec.Command) error {
	if strings.Contains(cmd.Name, "git") && len(cmd.Args) == 1 && cmd.Args[0] == "--version" {
		_, err := cmd.CombinedOutput.Write([]byte("git version 99.99.1"))
		return err
	}
	return nil
}
