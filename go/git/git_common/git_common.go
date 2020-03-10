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

var (
	commitHashRegex = regexp.MustCompile("^[a-z0-9]{7,40}$")
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

// ValidateRef returns an error if the given ref is not valid. Rules are derived
// from git-check-ref-format documentation, but commit hashes at least 7
// characters in length are also valid.
//
// See https://git-scm.com/docs/git-check-ref-format for more information.
func ValidateRef(ref string) error {
	// Support commit hashes, from 7-40 chars.
	if commitHashRegex.MatchString(ref) {
		return nil
	}

	// The below rules come from git-check-ref-format docs.

	// 1. No component can begin with "." or end with ".lock"
	split := strings.Split(ref, "/")
	for _, elem := range split {
		if strings.HasPrefix(elem, ".") || strings.HasSuffix(elem, ".lock") {
			return skerr.Fmt("No component can begin with \".\" or end with \".lock\"")
		}
	}

	// 2. Must contain at least one "/"
	if len(split) == 1 {
		return skerr.Fmt("Must contain at least one \"/\"")
	}

	// 3. Cannot contain ".."
	if strings.Contains(ref, "..") {
		return skerr.Fmt("Cannot contain \"..\"")
	}

	for _, char := range ref {
		// 4. Cannot contain ASCII control characters, space, tilde, caret, or colon.
		if char < 32 || char == 127 || char == ' ' || char == '~' || char == '^' || char == ':' {
			return skerr.Fmt("Cannot contain ASCII control characters, space, tilde, caret, or colon")
		}

		// 5. Cannot contain question mark, asterisk, or open bracket.
		if char == '?' || char == '*' || char == '[' {
			return skerr.Fmt("Cannot contain question mark, asterisk, or open bracket")
		}
	}

	// 6. Cannot begin or end with a slash or contain multiple consecutive slashes.
	if strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") || strings.Contains(ref, "//") {
		return skerr.Fmt("Cannot begin or end with a slash or contain multiple consecutive slashes")
	}

	// 7. Cannot end with a dot.
	if strings.HasSuffix(ref, ".") {
		return skerr.Fmt("Cannot end with a dot")
	}

	// 8. Cannot contain "@{".
	if strings.Contains(ref, "@{") {
		return skerr.Fmt("Cannot contain \"@{\"")
	}

	// 9. Cannot be "@".
	if ref == "@" {
		return skerr.Fmt("Cannot be \"@\"")
	}

	// 10. Cannot contain \.
	if strings.Contains(ref, "\\") {
		return skerr.Fmt("Cannot contain \"\\\"")
	}

	return nil
}
