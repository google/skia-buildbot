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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// DefaultBranch is the name of the default branch for most repositories.
	DefaultBranch = "master"
	// SecondaryDefaultBranch is the name of the default branch for some
	// repositories which don't use DefaultBranch.
	// TODO(rmistry): Swap the default branch name and delete this after
	// http://skbug.com/11842 is resolved.
	SecondaryDefaultBranch = "main"
	// DefaultRef is the fully-qualified ref name of the default branch for most
	// repositories.
	DefaultRef = RefsHeadsPrefix + DefaultBranch
	// DefaultRemote is the name of the default remote repository.
	DefaultRemote = "origin"
	// DefaultRemoteBranch is the name of the default branch in the default
	// remote repository, for most repos.
	DefaultRemoteBranch = DefaultRemote + "/" + DefaultBranch
	// RefsHeadsPrefix is the "refs/heads/" prefix used for branches.
	RefsHeadsPrefix = "refs/heads/"
)

var (
	gitVersionRegex = regexp.MustCompile("git version (\\d+)\\.(\\d+)\\..*")
	gitVersionMajor = 0
	gitVersionMinor = 0
	git             = ""
	mtx             sync.Mutex // Protects git, gitVersionMajor, and gitVersionMinor.
)

// FindGit returns the path to the Git executable and the major and minor
// version numbers, or any error which occurred.
func FindGit(ctx context.Context) (string, int, int, error) {
	mtx.Lock()
	defer mtx.Unlock()
	if git == "" {
		gitPath, err := osexec.LookPath("git")
		if err != nil {
			return "", 0, 0, skerr.Wrapf(err, "Failed to find git")
		}
		maj, min, err := Version(ctx, gitPath)
		if err != nil {
			return "", 0, 0, skerr.Wrapf(err, "Failed to obtain git version")
		}
		sklog.Infof("Git is %s; version %d.%d", gitPath, maj, min)
		isFromCIPD := IsFromCIPD(gitPath)
		isFromCIPDVal := 0
		if isFromCIPD {
			isFromCIPDVal = 1
		}
		metrics2.GetInt64Metric("git_from_cipd").Update(int64(isFromCIPDVal))
		git = gitPath
		gitVersionMajor = maj
		gitVersionMinor = min
	}
	return git, gitVersionMajor, gitVersionMinor, nil
}

// IsFromCIPD returns a bool indicating whether or not the given version of Git
// appears to be obtained via CIPD.
func IsFromCIPD(git string) bool {
	return strings.Contains(git, "cipd_bin_packages")
}

// EnsureGitIsFromCIPD returns an error if the version of Git in PATH does not
// appear to be obtained via CIPD.
func EnsureGitIsFromCIPD(ctx context.Context) error {
	git, _, _, err := FindGit(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !IsFromCIPD(git) {
		return skerr.Fmt("Git does not appear to be obtained via CIPD: %s", git)
	}
	return nil
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
