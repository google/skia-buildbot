// The presubmit binary runs many checks on the differences between the current commit and its
// parent branch. This is usually origin/main. If there is a chain of CLs (i.e. branches), this
// will only consider the diffs between the current commit and the parent branch.
// If all presubmits pass, this binary will output nothing (by default) and have exit code 0.
// If any presubmits fail, there will be errors logged to stdout and the exit code will be non-zero.
//
// This should be invoked from the root of the repo via Bazel like
//
//	bazel run //cmd/presubmit
//
// See presubmit.sh for a helper that pipes in the correct value for repo_dir.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	git_pkg "go.skia.org/infra/go/git"
)

func main() {
	// TODO(borenet): Share this code with //presubmit.go.
	var (
		// https://bazel.build/docs/user-manual#running-executables
		repoDir  = flag.String("repo_dir", os.Getenv("BUILD_WORKSPACE_DIRECTORY"), "The root directory of the repo. Default set by BUILD_WORKSPACE_DIRECTORY env variable.")
		upstream = flag.String("upstream", "origin/main", "The upstream repo to diff against.")
		verbose  = flag.Bool("verbose", false, "If extra logging is desired")
		upload   = flag.Bool("upload", false, "If true, this will skip any checks that are not suitable for an upload check (may be the empty set).")
		commit   = flag.Bool("commit", false, "If true, this will skip any checks that are not suitable for a commit check (may be the empty set).")
	)
	flag.Parse()

	ctx := withOutputWriter(context.Background(), os.Stdout)
	if *repoDir == "" {
		logf(ctx, "Must set --repo_dir\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *upload && *commit {
		logf(ctx, "Cannot set both --upload and --commit\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := os.Chdir(*repoDir); err != nil {
		logf(ctx, "Could not cd to %s\n", *repoDir)
		os.Exit(1)
	}

	filesWithDiffs, untrackedFiles := findUncommittedChanges(ctx)
	if len(filesWithDiffs) > 0 {
		logf(ctx, "Found uncommitted changes in %d files:\n", len(filesWithDiffs))
		for _, file := range filesWithDiffs {
			logf(ctx, "    %s\n", file)
		}
		logf(ctx, "Aborting.\n")
		os.Exit(1)
	}
	for _, uf := range untrackedFiles {
		if filepath.Base(uf) == "BUILD.bazel" {
			logf(ctx, "Found uncommitted BUILD.bazel files. Please delete these or check them in. Aborting.\n")
			os.Exit(1)
		}
	}

	branchBaseCommit := findBranchBase(ctx, *upstream)
	if branchBaseCommit == "" {
		// This either means the user hasn't committed their changes or is just on the main branch
		// somewhere. Either way, we don't want to run the presubmits. It could mutate those
		// un-committed changes or just be a no-op, since presumably code in the past passed the
		// presubmit checks.
		logf(ctx, "No commits since %s. Presubmit passes by default. Did you commit all new files?\n", *upstream)
		os.Exit(0)
	}
	if *verbose {
		logf(ctx, "Base commit is %s\n", branchBaseCommit)
	}

	changedFiles, deletedFiles := computeDiffFiles(ctx, branchBaseCommit)
	if *verbose {
		logf(ctx, "Changed files:\n%v\n", changedFiles)
		logf(ctx, "Deleted files:\n%s\n", deletedFiles)
	}
	ok := true
	ok = ok && validateConfigs(ctx)
	ok = ok && checkK8sConfigGeneration(ctx)
	if !*commit {
		// Nothing to do here currently.
	}

	// Only mutate the repo if we are good so far.
	if !ok {
		logf(ctx, "Presubmit errors detected!\n")
		os.Exit(1)
	}

	if !checkGenerateConfigs(ctx, changedFiles) {
		os.Exit(1)
	}

	os.Exit(0)
}

func checkK8sConfigGeneration(ctx context.Context) bool {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		logf(ctx, "Failed to create tmp dir: %s", err)
		return false
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			logf(ctx, "Failed to remove tmp dir: %s", err)
		}
	}()

	output, err := run(ctx, "bash", "./gen-k8s-configs.sh", tmp)
	if err != nil {
		logf(ctx, "Failed running gen-k8s-configs.sh:\n%s\nOutput:\n%s", err, string(output))
		return false
	}
	return true
}

func validateConfigs(ctx context.Context) bool {
	output, err := run(ctx, "bash", "./validate-configs.sh")
	if err != nil {
		logf(ctx, "Failed running validate-configs.sh:\n%s\nOutput:\n%s", err, string(output))
		return false
	}
	return true
}

func checkDiffs(ctx context.Context, script string, args []string, suffixes []string) bool {
	output, err := run(ctx, "bash", append([]string{script}, args...)...)
	if err != nil {
		logf(ctx, "Failed running %s:\n%s\nOutput:\n%s\n", script, err, string(output))
		return false
	}

	newDiffs, untrackedFiles := findUncommittedChanges(ctx)
	if len(newDiffs) > 0 {
		logf(ctx, "%s caused changes. Please inspect them (git diff) and commit if ok.\n", script)
		for _, diff := range newDiffs {
			logf(ctx, diff+"\n")
		}
		return false
	}
	for _, uf := range untrackedFiles {
		for _, suffix := range suffixes {
			if strings.HasSuffix(uf, suffix) {
				logf(ctx, "%s created new files. Please inspect these and check them in.\n", script)
				return false
			}
		}
	}
	return true
}

func checkGenerateConfigs(ctx context.Context, changedFiles []fileWithChanges) bool {
	return checkDiffs(ctx, "./template/generate.sh", nil, []string{".cfg"})
}

const (
	gitErrorMessage = `Error running git - is git on your path?

If running with Bazel, you need to invoke this like:
bazel run //cmd/presubmit --run_under="cd $PWD &&"
`
	refSeperator = "$|" // Some string we hope that no users start their branch names with
)

// findUncommittedChanges returns a list of files that git says have changed (compared to HEAD)
// as well as a list of untracked, unignored files.
func findUncommittedChanges(ctx context.Context) (filesWithDiffs []string, untrackedFiles []string) {
	output, err := git(ctx, "diff", "--name-status", "HEAD")
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	filesWithDiffs = extractFilesWithDiffs(string(output))

	// https://stackoverflow.com/a/2659808/1447621
	// This will list all untracked, unignored files on their own lines
	output, err = git(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	return filesWithDiffs, strings.Split(string(output), "\n")
}

func extractFilesWithDiffs(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	var files []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		files = append(files, fields[len(fields)-1])
	}
	return files
}

// findBranchBase returns the git commit of the parent branch. If there is a chain of CLs
// (i.e. branches), this will return the parent branch's most recent commit. If we are on the
// upstream branch, this will return empty string. Otherwise, it will return the commit on the
// upstream branch where branching occurred. It shells out to git, which is presumed to be on
// PATH in order to find this information.
func findBranchBase(ctx context.Context, upstream string) string {
	// rev-list is one of the git "plumbing" commands and the output should be relatively stable.
	// https://mirrors.edge.kernel.org/pub/software/scm/git/docs/git.html#_low_level_commands_plumbing
	output, err := git(ctx, "rev-list", "HEAD", "^"+upstream,
		// %D means "ref names", which is the commit hash and any branch name associated with it
		// %P means the parent hash
		`--format=%D`+refSeperator+`%P`)
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	return extractBranchBase(string(output))
}

type revEntry struct {
	commit  string
	branch  string
	parents []string
	// depth is a monotonically increasing number for each commit we see as we are going back
	// in time.
	depth int
}

// extractBranchBase looks for the most recent branch, as indicated by the "ref name". Failing to find
// that, it will return the last parent commit, which will connect to the upstream branch.
func extractBranchBase(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	// Create a graph of commits. The entries map holds onto all the nodes (using the commit as
	// a key). After we create the graph, we'll look for the key features.
	entries := map[string]*revEntry{}
	var currentEntry *revEntry
	depth := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "commit ") {
			c := strings.TrimPrefix(line, "commit ")
			entry := entries[c]
			if entry == nil {
				entry = &revEntry{commit: c, depth: depth}
				entries[c] = entry
			} else {
				entry.depth = depth
			}
			currentEntry = entry
			depth++
			continue
		}
		parts := strings.Split(line, refSeperator)
		// First part is the possibly empty branch name
		currentEntry.branch = parts[0]
		// second part is the possibly multiple parents, seperated by spaces
		parents := strings.Split(parts[1], " ")
		currentEntry.parents = parents
		for _, parent := range parents {
			// Associate the parents with the depth of the child they apply to.
			entries[parent] = &revEntry{commit: parent, depth: depth}
		}
	}

	// Go through the created graph and find commits of interest
	var shallowestCommitWithNoParents *revEntry
	var shallowestCommitWithBranch *revEntry
	for _, entry := range entries {
		if len(entry.parents) == 0 {
			if shallowestCommitWithNoParents == nil || shallowestCommitWithNoParents.depth > entry.depth {
				shallowestCommitWithNoParents = entry
			}
		}
		if entry.branch != "" && !strings.HasPrefix(entry.branch, "HEAD -> ") {
			if shallowestCommitWithBranch == nil || shallowestCommitWithBranch.depth > entry.depth {
				shallowestCommitWithBranch = entry
			}
		}
	}

	// If we found a branch that HEAD descends from, compare to the shallowest commit belonging
	// to that branch.
	if shallowestCommitWithBranch != nil {
		return shallowestCommitWithBranch.commit
	}
	// Otherwise, go with the shallowest commit that we didn't find parents for. These parent-less
	// commits correspond to commits on the main branch, and the shallowest one will be the newest.
	if shallowestCommitWithNoParents != nil {
		return shallowestCommitWithNoParents.commit
	}
	// This should not happen unless we are parsing things wrong.
	panic("Could not find a branch to compare to")
}

// computeDiffFiles returns a slice of changed (modified or added) files with the lines touched
// and a slice of deleted files. It shells out to git, which is presumed to be on PATH in order
// to find this information.
func computeDiffFiles(ctx context.Context, branchBase string) ([]fileWithChanges, []string) {
	// git diff-index is considered to be a "git plumbing" API, so its output should be pretty
	// stable across git version (unlike ordinary git diff, which can do things like show
	// tabs as multiple spaces).
	output, err := git(ctx, "diff-index", branchBase,
		// Don't show any surrounding context (i.e. lines which did not change)
		"--unified=0",
		"--patch-with-raw",
		"--no-color")
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	return extractChangedAndDeletedFiles(string(output))
}

var (
	gitDiffLine = regexp.MustCompile(`^diff --git (?P<fileA>.*) (?P<fileB>.*)$`)
	lineAnchor  = regexp.MustCompile(`^@@ -(?P<deleted>\d+)(?P<delLines>,\d+)? \+(?P<added>\d+)(?P<addLines>,\d+)? @@.*$`)
)

const (
	deletedFilePrefix = "deleted file mode"
	addedLinePrefix   = "+"
)

// fileWithChanges represents an added or modified files along with the lines changed (touched).
// Lines that were deleted are not tracked.
type fileWithChanges struct {
	fileName     string
	touchedLines []lineOfCode
}

func (f fileWithChanges) String() string {
	rv := f.fileName + "\n"
	for _, line := range f.touchedLines {
		rv += "  " + line.String() + "\n"
	}
	return rv
}

type lineOfCode struct {
	contents string
	num      int
}

func (c lineOfCode) String() string {
	return fmt.Sprintf("% 4d:%s", c.num, c.contents)
}

// extractChangedAndDeletedFiles looks through the provided `git diff` output and finds the files
// that were added or modified, as well as the new version of any lines touched. It also returns
// a slice of deleted files.
func extractChangedAndDeletedFiles(diffOutput string) ([]fileWithChanges, []string) {
	var changed []fileWithChanges
	var deleted []string
	lines := strings.Split(diffOutput, "\n")
	currFileDeleted := false
	lastLineIndex := -1
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if match := gitDiffLine.FindStringSubmatch(line); len(match) > 0 {
			// A new file was changed, reset our counters, trackers, and add an entry for it.
			var newFile fileWithChanges
			newFile.fileName = strings.TrimPrefix(match[2], "b/")
			changed = append(changed, newFile)
			currFileDeleted = false
			lastLineIndex = -1
		}
		if currFileDeleted || len(changed) == 0 {
			continue
		}
		currFile := &changed[len(changed)-1]
		if strings.HasPrefix(line, deletedFilePrefix) {
			deleted = append(deleted, currFile.fileName)
			changed = changed[:len(changed)-1] // trim off changed file
			currFileDeleted = true
			continue
		}
		if match := lineAnchor.FindStringSubmatch(line); len(match) > 0 {
			// The added group has the line number corresponding to the next line starting with a +
			var err error
			lastLineIndex, err = strconv.Atoi(match[3])
			if err != nil {
				panic("Got an integer where none was expected: " + line + "\n" + err.Error())
			}
			continue
		}
		if lastLineIndex < 0 {
			// Have not found a line index yet, ignore the lines like:
			// ++ b/PRESUBMIT.py
			continue
		}
		if strings.HasPrefix(line, addedLinePrefix) {
			currFile.touchedLines = append(currFile.touchedLines, lineOfCode{
				contents: strings.TrimPrefix(line, addedLinePrefix),
				num:      lastLineIndex,
			})
			lastLineIndex++
		}
	}

	return changed, deleted
}

type contextKeyType string

const outputWriterKey contextKeyType = "outputWriter"

// withOutputWriter registers the given writer to the context. See also logf.
func withOutputWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, outputWriterKey, w)
}

// logf takes the writer on the context and writes the given format and arguments to it. This allows
// unit tests to intercept logged output if necessary. It panics if a writer was not registered
// using withOutputWriter.
func logf(ctx context.Context, format string, args ...interface{}) {
	w, ok := ctx.Value(outputWriterKey).(io.Writer)
	if !ok {
		panic("Must set outputWriter on ctx")
	}
	_, err := fmt.Fprintf(w, format, args...)
	if err != nil {
		panic("Error while logging " + err.Error())
	}
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func git(ctx context.Context, args ...string) ([]byte, error) {
	gitExec, err := git_pkg.Executable(ctx)
	if err != nil {
		return nil, err
	}
	return run(ctx, gitExec, args...)
}
