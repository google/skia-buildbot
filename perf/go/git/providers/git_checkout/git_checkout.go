// Package git_checkout implements provider.Provider by shelling out to run git commands.
package git_checkout

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/oauth2/google"
)

// Impl implements provider.Provider.
type Impl struct {
	// gitFullPath is the path of the git executable.
	gitFullPath string

	// repoFullPath if the full path of the checked out Git repo.
	repoFullPath string

	// startCommit is the commit in the repo where we start tracking commits. If
	// not supplied then we start with the first commit in the repo as reachable
	// from HEAD.
	startCommit string
}

// New returns a new instance of Impl, which implements provider.Provider.
func New(ctx context.Context, instanceConfig *config.InstanceConfig) (*Impl, error) {

	// Do git authentication if required.
	if instanceConfig.GitRepoConfig.GitAuthType == config.GitAuthGerrit {
		sklog.Info("Authenticating to Gerrit.")
		ts, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to get tokensource perfgit.Git for config %v", *instanceConfig)
		}
		if _, err := gitauth.New(ctx, ts, "/tmp/git-cookie", true, ""); err != nil {
			return nil, skerr.Wrapf(err, "Failed to gitauth perfgit.Git for config %v", *instanceConfig)
		}
	}

	// Find the path to the git executable, which might be relative to working dir.
	gitFullPath, _, _, err := git_common.FindGit(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find git.")
	}

	// Force the path to be absolute.
	gitFullPath, err = filepath.Abs(gitFullPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get absolute path to git.")
	}

	// Clone the git repo if necessary.
	sklog.Infof("Cloning repo.")
	if _, err := os.Stat(instanceConfig.GitRepoConfig.Dir); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, gitFullPath, "clone", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
		if err := cmd.Run(); err != nil {
			exerr := err.(*exec.ExitError)
			return nil, skerr.Wrapf(err, "Failed to clone repo: %s - %s", err, exerr.Stderr)
		}
	}

	return &Impl{
		gitFullPath:  gitFullPath,
		repoFullPath: instanceConfig.GitRepoConfig.Dir,
		startCommit:  instanceConfig.GitRepoConfig.StartCommit,
	}, nil
}

// Used in defers.
func cmdWaitAndLog(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		sklog.Errorf("running git log: %q", err)
	}
}

// CommitsFromMostRecentGitHashToHead implements provider.Provider.
func (i Impl) CommitsFromMostRecentGitHashToHead(ctx context.Context, mostRecentGitHash string, cb provider.CommitProcessor) error {
	var cmd *exec.Cmd
	if mostRecentGitHash == "" {
		mostRecentGitHash = i.startCommit
	}
	if mostRecentGitHash == "" {
		cmd = exec.CommandContext(ctx, i.gitFullPath, "rev-list", "HEAD", `--pretty=%aN <%aE>%n%s%n%ct`, "--reverse")
	} else {
		// Add all the commits from the repo since the last time we looked.
		cmd = exec.CommandContext(ctx, i.gitFullPath, "rev-list", "HEAD", "^"+mostRecentGitHash, `--pretty=%aN <%aE>%n%s%n%ct`, "--reverse")
	}

	cmd.Dir = i.repoFullPath
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}
	defer cmdWaitAndLog(cmd)

	err = parseGitRevLogStream(stdout, func(p provider.Commit) error {
		return cb(p)
	})
	if err != nil {
		return skerr.Wrapf(err, "parsing git stdout")
	}

	return nil
}

// GitHashesInRangeForFile implements provider.Provider.
func (i Impl) GitHashesInRangeForFile(ctx context.Context, begin, end, filename string) ([]string, error) {
	var revisionRange string
	if begin == "" {
		begin = i.startCommit
	}
	if begin == "" {
		// git log revision range queries of the form hash1..hash2 are exclusive
		// of hash1, so we need to always back up begin one commit, except in
		// the case where the commit number is 0, then we change the revision
		// range.
		revisionRange = end
	} else {
		revisionRange = begin + ".." + end
	}

	// Build the git log command to run.
	cmd := exec.CommandContext(ctx, i.gitFullPath, "log", revisionRange, "--reverse", "--format=format:%H", "--", filename)
	cmd.Dir = i.repoFullPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := cmd.Start(); err != nil {
		return nil, skerr.Wrap(err)
	}
	defer cmdWaitAndLog(cmd)

	// Read the git log output.
	scanner := bufio.NewScanner(stdout)
	ret := []string{}
	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}

	if scanner.Err() != nil {
		return nil, skerr.Wrap(err)
	}
	return ret, nil
}

// LogEntry implements provider.Provider.
func (i Impl) LogEntry(ctx context.Context, hash string) (string, error) {
	// Build the git log command to run.
	cmd := exec.CommandContext(ctx, i.gitFullPath, "show", "-s", hash)
	cmd.Dir = i.repoFullPath
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", skerr.Wrapf(err, "Failed running %q: stdout: %q  stderr: %q", cmd.String(), out.String(), stderr.String())
	}

	return out.String(), nil
}

type parseGitRevLogStreamProcessSingleCommit func(commit provider.Commit) error

// parseGitRevLogStream parses the input stream for input of the form:
//
//	commit 6079a7810530025d9877916895dd14eb8bb454c0
//	Joe Gregorio <joe@bitworking.org>
//	Change #9
//	1584837783
//	commit 977e0ef44bec17659faf8c5d4025c5a068354817
//	Joe Gregorio <joe@bitworking.org>
//	Change #8
//	1584837783
//
// And calls the parseGitRevLogStreamProcessSingleCommit function with each
// entry it finds. The passed in Commit has all valid fields except
// CommitNumber, which is set to types.BadCommitNumber.
func parseGitRevLogStream(r io.ReadCloser, f parseGitRevLogStreamProcessSingleCommit) error {
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "commit ") {
			return skerr.Fmt("Invalid format, expected commit at line %d: %q", lineNumber, line)
		}
		lineNumber++
		gitHash := strings.Split(line, " ")[1]

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting an author line: %d", lineNumber)
		}
		lineNumber++
		author := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a subject line: %d", lineNumber)
		}
		lineNumber++
		subject := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a timestamp line: %d", lineNumber)
		}
		lineNumber++
		timestampString := scanner.Text()
		ts, err := strconv.ParseInt(timestampString, 10, 64)
		if err != nil {
			return skerr.Fmt("Failed to parse timestamp %q at line %d", timestampString, lineNumber)
		}
		if err := f(provider.Commit{
			CommitNumber: types.BadCommitNumber,
			GitHash:      gitHash,
			Timestamp:    ts,
			Author:       author,
			Subject:      subject}); err != nil {
			return skerr.Wrap(err)
		}
	}
	return skerr.Wrap(scanner.Err())
}

// Update implements provider.Provider.
func (i Impl) Update(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "perfgit.pull")
	defer span.End()

	cmd := exec.CommandContext(ctx, i.gitFullPath, "pull")
	cmd.Dir = i.repoFullPath
	if err := cmd.Run(); err != nil {
		exerr := err.(*exec.ExitError)
		return skerr.Wrapf(err, "Failed to pull repo %q with git %q: %s", i.repoFullPath, i.gitFullPath, exerr.Stderr)
	}
	return nil
}

var _ provider.Provider = Impl{}
