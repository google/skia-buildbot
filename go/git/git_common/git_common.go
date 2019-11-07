package git_common

/*
   Common elements used by go/git and go/git/testutils.
*/

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	gitVersionRegex = regexp.MustCompile("git version (\\d+)\\.(\\d+)\\..*")
	gitVersionMajor = 0
	gitVersionMinor = 0
	git             = ""
	isFromCIPD      = false
)

// FindGit returns the path to the Git executable and the major and minor
// version numbers, or any error which occurred.
func FindGit(ctx context.Context) (string, int, int, error) {
	if git == "" {
		out, err := exec.RunCwd(ctx, ".", exec.WHICH, "git")
		if err != nil {
			return "", 0, 0, skerr.Wrapf(err, "Failed to find git")
		}
		gitPath := strings.TrimSpace(out)
		maj, min, err := Version(ctx, gitPath)
		if err != nil {
			return "", 0, 0, skerr.Wrapf(err, "Failed to obtain git version")
		}
		sklog.Infof("Git is %s; version %d.%d", gitPath, maj, min)
		if !IsFromCIPD(gitPath) {
			sklog.Errorf("Git at %s does not appear to be obtained via CIPD; this will be a fatal error soon.", gitPath)
		}
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
