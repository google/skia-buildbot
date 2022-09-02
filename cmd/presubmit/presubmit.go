// The presubmit binary runs many checks on the differences between the current commit and its
// parent branch. This is usually origin/main. If there is a chain of CLs (i.e. branches), this
// will only consider the diffs between the current commit and the parent branch.
// If all presubmits pass, this binary will output nothing (by default) and have exit code 0.
// If any presubmits fail, there will be errors logged to stdout and the exit code will be non-zero.
//
// This should be invoked from the root of the repo via Bazel like
//   bazel run //cmd/presubmit
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

	"go.skia.org/infra/bazel/external/buildifier"
)

func main() {
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

	// The pre-built binaries we need are relative to the path that Bazel starts us in.
	// We need to get those paths before we re-locate to the git repo we are testing.
	buildifierPath := buildifier.MustFindBuildifier()

	if err := os.Chdir(*repoDir); err != nil {
		logf(ctx, "Could not cd to %s\n", *repoDir)
		os.Exit(1)
	}

	filesWithDiffs, untrackedFiles := findUncommittedChanges(ctx)
	if len(filesWithDiffs) > 0 {
		logf(ctx, "Found uncommitted changes in %d files. Aborting.\n", len(filesWithDiffs))
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
	ok = ok && checkTODOHasOwner(ctx, changedFiles)
	ok = ok && checkForStrayWhitespace(ctx, changedFiles)
	ok = ok && checkPythonFilesHaveNoTabs(ctx, changedFiles)
	ok = ok && checkBannedGoAPIs(ctx, changedFiles)
	ok = ok && checkJSDebugging(ctx, changedFiles)
	if !*commit {
		// Long lines are sometimes inevitable. Ideally we would add these long line files to
		// the excluded list, but sometimes that is hard to do precisely.
		ok = ok && checkLongLines(ctx, changedFiles)
		// Give warnings for non-ASCII characters on upload but not commit, since they may
		// be intentional.
		ok = ok && checkNonASCII(ctx, changedFiles)
	}

	// Only mutate the repo if we are good so far.
	if !ok {
		logf(ctx, "Presubmit errors detected!\n")
		os.Exit(1)
	}

	if !runBuildifier(ctx, buildifierPath, changedFiles) {
		os.Exit(1)
	}
	if !runGofmt(ctx, changedFiles) {
		os.Exit(1)
	}
	if !runGazelle(ctx, changedFiles, deletedFiles) {
		os.Exit(1)
	}

	os.Exit(0)
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
	// diff-index is one of the git "plumbing" commands and the output should be relatively stable.
	// https://mirrors.edge.kernel.org/pub/software/scm/git/docs/git.html#_low_level_commands_plumbing
	cmd := exec.CommandContext(ctx, "git", "diff-index", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	filesWithDiffs = extractFilesWithDiffs(string(output))

	// https://stackoverflow.com/a/2659808/1447621
	// This will list all untracked, unignored files on their own lines
	cmd = exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	output, err = cmd.CombinedOutput()
	if err != nil {
		logf(ctx, string(output)+"\n")
		logf(ctx, err.Error()+"\n")
		panic(gitErrorMessage)
	}
	return filesWithDiffs, strings.Split(string(output), "\n")
}

var fileDiff = regexp.MustCompile(`^:.+\t(?P<file>[^\t]+)$`)

func extractFilesWithDiffs(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	var files []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if match := fileDiff.FindStringSubmatch(line); len(match) > 0 {
			files = append(files, match[1])
		}
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
	cmd := exec.CommandContext(ctx, "git", "rev-list", "HEAD", "^"+upstream,
		// %D means "ref names", which is the commit hash and any branch name associated with it
		// %P means the parent hash
		`--format=%D`+refSeperator+`%P`)
	output, err := cmd.CombinedOutput()
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
	cmd := exec.CommandContext(ctx, "git", "diff-index", branchBase,
		// Don't show any surrounding context (i.e. lines which did not change)
		"--unified=0",
		"--patch-with-raw",
		"--no-color")
	output, err := cmd.CombinedOutput()
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

// checkLongLines looks through all touched lines and returns false if any of them (not covered by
// exceptions) have lines longer than 100 lines, an arbitrary measurement.
// Based on https://chromium.googlesource.com/chromium/tools/depot_tools.git/+/19b3eb5adbe00e9da40375cb5dc47380a46f3041/presubmit_canned_checks.py#488
func checkLongLines(ctx context.Context, files []fileWithChanges) bool {
	const maxLineLength = 100
	ignoreFileExts := []string{".go", ".html", ".py"}
	ignoreFiles := []string{"package-lock.json", "go.sum", "infra/bots/tasks.json", "WORKSPACE",
		"golden/k8s-config-templates/gold-common.json5"}
	ok := true
	for _, f := range files {
		if contains(ignoreFiles, f.fileName) {
			continue
		}
		if contains(ignoreFileExts, filepath.Ext(f.fileName)) {
			continue
		}
		for _, line := range f.touchedLines {
			if len(line.contents) > maxLineLength {
				logf(ctx, "%s:%d Line too long (%d/%d)\n", f.fileName, line.num, len(line.contents), maxLineLength)
				ok = false
			}
		}
	}
	return ok
}

var todoWithoutOwner = regexp.MustCompile(`TODO[^(]`)

// checkTODOHasOwner looks through all touched lines and returns false if any of them (not covered
// by exceptions) have a TODO without an owner or a bug.
// Based on https://chromium.googlesource.com/chromium/tools/depot_tools.git/+/19b3eb5adbe00e9da40375cb5dc47380a46f3041/presubmit_canned_checks.py#464
func checkTODOHasOwner(ctx context.Context, files []fileWithChanges) bool {
	ignoreFiles := []string{
		// These files have TODO in their function names and test data.
		"cmd/presubmit/presubmit.go", "cmd/presubmit/presubmit_test.go",
	}
	ok := true
	for _, f := range files {
		if contains(ignoreFiles, f.fileName) {
			continue
		}
		for _, line := range f.touchedLines {
			if todoWithoutOwner.MatchString(line.contents) || strings.HasSuffix(line.contents, "TODO") {
				logf(ctx, "%s:%d TODO without owner or bug\n", f.fileName, line.num)
				ok = false
			}
		}
	}
	return ok
}

var trailingWhitespace = regexp.MustCompile(`\s+$`)

// checkForStrayWhitespace goes through all touched lines and returns false if any of them end with
// a whitespace character (e.g. tabs, spaces). newlines should have been stripped out of
// the content of a line earlier.
// Based on https://chromium.googlesource.com/chromium/tools/depot_tools.git/+/19b3eb5adbe00e9da40375cb5dc47380a46f3041/presubmit_canned_checks.py#476
func checkForStrayWhitespace(ctx context.Context, files []fileWithChanges) bool {
	ok := true
	for _, f := range files {
		for _, line := range f.touchedLines {
			if trailingWhitespace.MatchString(line.contents) {
				logf(ctx, "%s:%d Trailing whitespace\n", f.fileName, line.num)
				ok = false
			}
		}
	}
	return ok
}

// checkPythonFilesHaveNoTabs goes through the touched lines of all Python files and returns false
// if any of them have tabs anywhere.
// Based on https://chromium.googlesource.com/chromium/tools/depot_tools.git/+/19b3eb5adbe00e9da40375cb5dc47380a46f3041/presubmit_canned_checks.py#441
func checkPythonFilesHaveNoTabs(ctx context.Context, files []fileWithChanges) bool {
	ok := true
	for _, f := range files {
		if filepath.Ext(f.fileName) != ".py" {
			continue
		}
		for _, line := range f.touchedLines {
			if strings.Contains(line.contents, "\t") {
				logf(ctx, "%s:%d Tab character not allowed\n", f.fileName, line.num)
				ok = false
			}
		}
	}
	return ok
}

type bannedGoAPI struct {
	regex      *regexp.Regexp
	suggestion string
	exceptions []*regexp.Regexp
}

// checkBannedGoAPIs goes through all touched lines in go files and returns false if any of them
// have APIs that we wish not to use. It logs suggested replacements in that case.
func checkBannedGoAPIs(ctx context.Context, files []fileWithChanges) bool {
	bannedAPIs := []bannedGoAPI{
		{regex: regexp.MustCompile(`reflect\.DeepEqual`), suggestion: "DeepEqual in go.skia.org/infra/go/testutils"},
		{regex: regexp.MustCompile(`github\.com/golang/glog`), suggestion: "go.skia.org/infra/go/sklog"},
		{regex: regexp.MustCompile(`github\.com/skia-dev/glog`), suggestion: "go.skia.org/infra/go/sklog"},
		{regex: regexp.MustCompile(`http\.Get`), suggestion: "NewTimeoutClient in go.skia.org/infra/go/httputils"},
		{regex: regexp.MustCompile(`http\.Head`), suggestion: "NewTimeoutClient in go.skia.org/infra/go/httputils"},
		{regex: regexp.MustCompile(`http\.Post`), suggestion: "NewTimeoutClient in go.skia.org/infra/go/httputils"},
		{regex: regexp.MustCompile(`http\.PostForm`), suggestion: "NewTimeoutClient in go.skia.org/infra/go/httputils"},
		{regex: regexp.MustCompile(`os\.Interrupt`), suggestion: "AtExit in go.skia.org/go/cleanup"},
		{regex: regexp.MustCompile(`signal\.Notify`), suggestion: "AtExit in go.skia.org/go/cleanup"},
		{regex: regexp.MustCompile(`syscall\.SIGINT`), suggestion: "AtExit in go.skia.org/go/cleanup"},
		{regex: regexp.MustCompile(`syscall\.SIGTERM`), suggestion: "AtExit in go.skia.org/go/cleanup"},
		{regex: regexp.MustCompile(`syncmap\.Map`), suggestion: "sync.Map, added in go 1.9"},
		{regex: regexp.MustCompile(`assert\s+"github\.com/stretchr/testify/require"`), suggestion: `non-aliased import; this can be confused with package "github.com/stretchr/testify/assert"`},
		{
			regex:      regexp.MustCompile(`"git"`),
			suggestion: `Executable in go.skia.org/infra/go/git`,
			exceptions: []*regexp.Regexp{
				// These don't actually shell out to git; the tests look for "git" in the
				// command line and mock stdout accordingly.
				regexp.MustCompile(`autoroll/go/repo_manager/.*_test.go`),
				// This doesn't shell out to git; it's referring to a CIPD package with
				// the same name.
				regexp.MustCompile(`infra/bots/gen_tasks.go`),
				// This doesn't shell out to git; it retrieves the path to the Git binary
				// in the corresponding Bazel-downloaded CIPD packages.
				regexp.MustCompile(`bazel/external/cipd/git/git.go`),
				// Our presubmits invoke git directly because git is a necessary
				// executable for all devs, and we do not want our presubmit code to
				// depend on the code it is checking.
				regexp.MustCompile(`cmd/presubmit/.*`),
				// This is the one place where we are allowed to shell out to git; all
				// others should go through here.
				regexp.MustCompile(`go/git/git_common/.*.go`),
			},
		},
	}
	ok := true
	for _, f := range files {
		if filepath.Ext(f.fileName) != ".go" {
			continue
		}
		if f.fileName == "cmd/presubmit/presubmit_test.go" {
			// We don't want our own test cases to trigger any of these.
			continue
		}
		for _, line := range f.touchedLines {
		bannedAPILoop:
			for _, bannedAPI := range bannedAPIs {
				for _, exception := range bannedAPI.exceptions {
					if exception.MatchString(f.fileName) {
						continue bannedAPILoop
					}
				}
				if match := bannedAPI.regex.FindStringSubmatch(line.contents); len(match) > 0 {
					logf(ctx, "%s:%d Instead of %s, please use %s\n", f.fileName, line.num, match[0], bannedAPI.suggestion)
					ok = false
				}
			}
		}
	}
	return ok
}

// checkJSDebugging goes through all touched lines and returns false if any TS or JS files contain
// refinements of debugging that we don't want to check in.
func checkJSDebugging(ctx context.Context, files []fileWithChanges) bool {
	debuggingCalls := []string{"debugger;", "it.only(", "describe.only("}
	targetFileExts := []string{".ts", ".js"}
	ok := true
	for _, f := range files {
		if !contains(targetFileExts, filepath.Ext(f.fileName)) {
			continue
		}
		for _, line := range f.touchedLines {
			for _, call := range debuggingCalls {
				if strings.Contains(line.contents, call) {
					logf(ctx, "%s:%d debugging code found (%s)\n", f.fileName, line.num, call)
					ok = false
				}
			}
		}
	}
	return ok
}

// checkNonASCII goes through all touched lines and returns false if any of them contain non-ASCII
// characters (except for file formats that support things like UTF-8).
func checkNonASCII(ctx context.Context, files []fileWithChanges) bool {
	// This list can grow if other file extensions are OK with non-ascii (UTF-8) characters
	ignoreFileExts := []string{".go"}
	ok := true
	for _, f := range files {
		if contains(ignoreFileExts, filepath.Ext(f.fileName)) {
			continue
		}
		for _, line := range f.touchedLines {
			// https://stackoverflow.com/a/53069799/1447621
			for i := 0; i < len(line.contents); i++ {
				if line.contents[i] > '\u007F' { // unicode.MaxASCII
					// Report both line number and (1-indexed) byte offset
					logf(ctx, "%s:%d:%d Non ASCII character found\n", f.fileName, line.num, i+1)
					ok = false
					break
				}
			}
		}
	}
	return ok
}

// runBuildifier uses a provided buildifier path to reformat our BUILD.bazel and .bzl files as
// well as check them for linting errors. If buildifier has no output and a non-zero exit code,
// that is interpreted as "all good" and we return true. If there are issues, we print the
// buildifier output (which has files and line numbers) and return false.
func runBuildifier(ctx context.Context, buildifierPath string, files []fileWithChanges) bool {
	args := []string{"-lint=warn", "-mode=fix"}
	for _, f := range files {
		if filepath.Base(f.fileName) == "BUILD.bazel" || filepath.Ext(f.fileName) == ".bzl" {
			args = append(args, f.fileName)
		}
	}
	if len(args) == 2 { // no additional arguments (files) added to check
		return true
	}
	cmd := exec.CommandContext(ctx, buildifierPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logf(ctx, string(output))
		logf(ctx, "Buildifier linting errors detected!\n")
		return false
	}

	if xf, _ := findUncommittedChanges(ctx); len(xf) > 0 {
		logf(ctx, "Buildifier caused changes. Please inspect them (git diff) and commit if ok.\n")
		return false
	}
	return true
}

// runGofmt runs gofmt on any changed golang files. It returns false if gofmt fails or
// produces any diffs.
func runGofmt(ctx context.Context, files []fileWithChanges) bool {
	// -s means "simplify"
	// -w means "write", as in, modify the files that need formatting.
	args := []string{"run", "//:gofmt", "--", "-s", "-w"}
	for _, f := range files {
		if filepath.Ext(f.fileName) == ".go" {
			args = append(args, f.fileName)
		}
	}
	if len(args) == 5 { // no additional arguments (files) added to check
		return true
	}
	cmd := exec.CommandContext(ctx, "bazelisk", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logf(ctx, string(output))
		logf(ctx, "gofmt failed!\n")
		return false
	}

	if xf, _ := findUncommittedChanges(ctx); len(xf) > 0 {
		logf(ctx, "gofmt caused changes. Please inspect them (git diff) and commit if ok.\n")
		return false
	}
	return true
}

// runGazelle uses gazelle (and our custom gazelle plugin) to regenerate BUILD.bazel files for our
// go files as well as our Typescript and SCSS rules. Gazelle is idempotent, so if a user has
// already generated BUILD.bazel files, there should be no diffs. If the user forgot to do so,
// there could be diffs or new files added. This function returns false if that is the case or true
// if Gazelle made no modifications.
func runGazelle(ctx context.Context, changedFiles []fileWithChanges, deletedFiles []string) bool {
	// If these change, we should regenerate everything (slower, but more sound)
	globalFilesToCheck := []string{"WORKSPACE", "WORKSPACE.bazel", "go.mod", "go.sum"}
	globalExtensionsToCheck := []string{".bzl"}
	// If these change, then we should only need to update their containing folders.
	localFilesToCheck := []string{"BUILD.bazel"}
	localExtensionsToCheck := []string{".go", ".ts", ".scss"}
	var foldersToCheck []string
	regenEverything := len(deletedFiles) > 0
	if !regenEverything {
		for _, f := range changedFiles {
			if contains(globalFilesToCheck, filepath.Base(f.fileName)) || contains(globalExtensionsToCheck, filepath.Ext(f.fileName)) {
				regenEverything = true
				break
			}
			if contains(localFilesToCheck, filepath.Base(f.fileName)) || contains(localExtensionsToCheck, filepath.Ext(f.fileName)) {
				folder := filepath.Dir(f.fileName)
				if !contains(foldersToCheck, folder) {
					foldersToCheck = append(foldersToCheck, folder)
				}
			}
		}
	}

	// No need to run gazelle
	if !regenEverything && len(foldersToCheck) == 0 {
		return true
	}
	if regenEverything {
		cmd := exec.CommandContext(ctx, "bazelisk", "run", "//:gazelle", "--", "update-repos",
			"-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories")
		output, err := cmd.CombinedOutput()
		if err != nil {
			logf(ctx, string(output))
			logf(ctx, "Could not regenerate go_repositories.bzl!\n")
			return false
		}
	}

	args := []string{"run", "//:gazelle", "--", "update"}
	if regenEverything {
		// Reminder: we have changed directory into the repo root
		args = append(args, "./")
	} else {
		args = append(args, foldersToCheck...)
	}

	cmd := exec.CommandContext(ctx, "bazelisk", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logf(ctx, string(output))
		logf(ctx, "Could not regenerate BUILD.bazel changedFiles using gazelle!\n")
		return false
	}

	newDiffs, untrackedFiles := findUncommittedChanges(ctx)
	if len(newDiffs) > 0 {
		logf(ctx, "Gazelle caused changes. Please inspect them (git diff) and commit if ok.\n")
		return false
	}
	for _, uf := range untrackedFiles {
		if filepath.Base(uf) == "BUILD.bazel" {
			logf(ctx, "Gazelle created new BUILD.bazel files. Please inspect these and check them in.\n")
			return false
		}
	}
	return true
}

// contains returns true if the given slice has the provided element in it.
func contains[T string](a []T, s T) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
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
