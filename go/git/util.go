package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
)

const (
	// MasterBranch is the name of the default branch for most repositories.
	MasterBranch = git_common.MasterBranch
	// MainBranch is the name of the default branch for some
	// repositories which don't use MasterBranch.
	// TODO(rmistry): Delete this after http://skbug.com/11842 is resolved.
	MainBranch = git_common.MainBranch
	// DefaultRef is the fully-qualified ref name of the default branch for most
	// repositories.
	DefaultRef = git_common.DefaultRef
	// DefaultRemote is the name of the default remote repository.
	DefaultRemote = git_common.DefaultRemote
	// DefaultRemoteBranch is the name of the default branch in the default
	// remote repository, for most repos.
	DefaultRemoteBranch = git_common.DefaultRemoteBranch
)

// This regex is taken from:
// https://source.chromium.org/chromium/infra/infra/+/master:go/src/go.chromium.org/luci/common/git/footer/footer.go?q=%22%5E%5Cs*(%5B%5Cw-%5D%2B):%20*(.*)$%22&ss=chromium
var trailerRegex = regexp.MustCompile(`^\s*([\w-]+): *(.*)$`)

// Types of git objects.
const (
	ObjectTypeBlob   ObjectType = "blob"
	ObjectTypeCommit ObjectType = "commit"
	ObjectTypeTree   ObjectType = "tree"
)

// ObjectType represents a Git object type.
type ObjectType string

// Clone runs "git clone" into the given destination directory. Most callers
// should use NewRepo or NewCheckout instead.
func Clone(ctx context.Context, repoUrl, dest string, mirror bool) error {
	git, err := Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	if mirror {
		// We don't use a "real" mirror, since that syncs ALL refs,
		// including every patchset of every CL that gets uploaded. Instead,
		// we use a bare clone and then add the "mirror" config after
		// cloning. It would be equivalent to use --mirror and then update
		// the refspec to only sync the branches, but that would force the
		// initial clone step to sync every ref.
		if _, err := exec.RunCwd(ctx, ".", git, "clone", "--bare", repoUrl, dest); err != nil {
			return fmt.Errorf("Failed to clone repo: %s", err)
		}
		if _, err := exec.RunCwd(ctx, dest, git, "config", "remote.origin.mirror", "true"); err != nil {
			return fmt.Errorf("Failed to set git mirror config: %s", err)
		}
		if _, err := exec.RunCwd(ctx, dest, git, "config", "remote.origin.fetch", "refs/heads/*:refs/heads/*"); err != nil {
			return fmt.Errorf("Failed to set git mirror config: %s", err)
		}
		if _, err := exec.RunCwd(ctx, dest, git, "fetch", "--force", "--all"); err != nil {
			return fmt.Errorf("Failed to set git mirror config: %s", err)
		}
	} else {
		if _, err := exec.RunCwd(ctx, ".", git, "clone", repoUrl, dest); err != nil {
			return fmt.Errorf("Failed to clone repo: %s", err)
		}
	}
	return nil
}

// LogFromTo returns a string which is used to log from one commit to another.
// It is important to note that:
// - The results may include the second commit but will not include the first.
// - The results include all commits reachable from the first commit which are
//   not reachable from the second, ie. if there is a merge in the given
//   range, the results will include that line of history and not just the
//   commits which are descendants of the first commit. If you want only commits
//   which are ancestors of the second commit AND descendants of the first, you
//   should use LogLinear, but note that the results will be empty if the first
//   commit is not an ancestor of the second, ie. they're on different branches.
func LogFromTo(from, to string) string {
	return fmt.Sprintf("%s..%s", from, to)
}

// NormalizeURL strips everything from the URL except for the host and the path.
// A trailing ".git" is also stripped. The purpose is to allow for small
// variations in repo URL to be recognized as the same repo. The URL needs to
// contain a valid transport protocol, e.g. https, ssh.
// These URLs will all return 'github.com/skia-dev/textfiles':
//
//    "https://github.com/skia-dev/textfiles.git"
//    "ssh://git@github.com/skia-dev/textfiles"
//    "ssh://git@github.com:skia-dev/textfiles.git"
//
func NormalizeURL(inputURL string) (string, error) {
	// If the scheme is ssh we have to account for the scp-like syntax with a ':'
	const ssh = "ssh://"
	if strings.HasPrefix(inputURL, ssh) {
		inputURL = ssh + strings.Replace(inputURL[len(ssh):], ":", "/", 1)
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", skerr.Wrapf(err, "parsing inputURL")
	}

	host := parsedURL.Host
	// Trim trailing slashes and the ".git" extension.
	path := strings.TrimRight(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
	path = "/" + strings.TrimLeft(path, "/:")
	return host + path, nil
}

// DeleteLockFiles finds and deletes Git lock files within the given workdir.
func DeleteLockFiles(ctx context.Context, workdir string) error {
	sklog.Infof("Looking for git lockfiles in %s", workdir)
	output, err := exec.RunCwd(ctx, workdir, "find", ".", "-name", "index.lock")
	if err != nil {
		return err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		sklog.Info("No lockfiles found.")
		return nil
	}
	lockfiles := strings.Split(output, "\n")
	for _, f := range lockfiles {
		fp := filepath.Join(workdir, f)
		sklog.Warningf("Removing git lockfile: %s", fp)
		if err := os.Remove(fp); err != nil {
			return err
		}
	}
	return nil
}

// ParseDir parses the contents of a directory. Expects the contents to be in
// the format used by git, ie. lines taking the form:
//
// mode tree|blob hash name
//
func ParseDir(contents []byte) ([]os.FileInfo, error) {
	rv := []os.FileInfo{}
	for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if line == "" {
			continue
		}
		// Lines are formatted as follows:
		// mode tree|blob hash name
		fields := strings.Fields(line)
		if len(fields) != 4 {
			return nil, skerr.Fmt("Expected format \"mode tree|blob hash name\" but got:\n %s", contents)
		}
		// We can't know the size of the directory contents with the information
		// we're given.
		size := 0
		fi, err := MakeFileInfo(fields[3], fields[0], ObjectType(fields[1]), size)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, fi)
	}
	return rv, nil
}

// MakeFileInfo returns an os.FileInfo with the given information.
func MakeFileInfo(name, mode string, typ ObjectType, size int) (os.FileInfo, error) {
	modeInt, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid file mode %q", mode)
	}
	fileMode := os.FileMode(modeInt)
	isDir := false
	if typ == ObjectTypeTree {
		isDir = true
		fileMode = fileMode | os.ModeDir
	}
	if typ != ObjectTypeTree && typ != ObjectTypeBlob {
		return nil, skerr.Fmt("Invalid file type %q", typ)
	}
	return vfs.FileInfo{
		Name: path.Base(name),
		Size: int64(size),
		Mode: fileMode,
		// Gitiles doesn't give us the modification timestamp. We could make
		// one up using the timestamp of the commit which last touched the
		// file, but that would require an extra request.
		ModTime: time.Time{},
		IsDir:   isDir,
		Sys:     nil,
	}.Get(), nil
}

// SplitTrailers splits a commit message into a main commit message body and
// trailers footer.  Assumes that the commit message is already well-formed with
// respect to trailers, ie. there is an empty line between the last body
// paragraph and the single trailers paragraph, which contains only lines in
// "key: value" format.
func SplitTrailers(commitMsg string) ([]string, []string) {
	// Split the commit message into paragraphs.
	if commitMsg == "" {
		return []string{}, []string{}
	}
	lines := strings.Split(strings.TrimSpace(commitMsg), "\n")
	paragraphs := [][]string{}
	var paragraph []string
	for _, line := range lines {
		paragraph = append(paragraph, line)
		if line == "" {
			if len(paragraph) > 0 {
				paragraphs = append(paragraphs, paragraph)
			}
			paragraph = []string{}
		}
	}
	if len(paragraph) > 0 {
		paragraphs = append(paragraphs, paragraph)
	}

	// If the last paragraph consists of valid trailers, split off those lines,
	// otherwise include them as part of the main commit message body.
	if len(paragraphs) < 1 {
		return lines, []string{}
	}
	trailerLines := paragraphs[len(paragraphs)-1]
	for _, line := range trailerLines {
		if !trailerRegex.MatchString(line) {
			// At least one line in the last paragraph does not fit the trailer
			// format; assume there are no trailers.
			return lines, []string{}
		}
	}
	bodyLines := make([]string, 0, len(paragraphs[0]))
	for _, paragraph := range paragraphs[:len(paragraphs)-1] {
		bodyLines = append(bodyLines, paragraph...)
	}
	return bodyLines, trailerLines
}

// JoinTrailers joins a main commit message body with a trailers footer.
func JoinTrailers(bodyLines, trailers []string) string {
	commitMsg := make([]string, 0, len(bodyLines)+len(trailers)+1)
	commitMsg = append(commitMsg, bodyLines...)
	if len(commitMsg) > 0 && commitMsg[len(commitMsg)-1] != "" {
		commitMsg = append(commitMsg, "")
	}
	commitMsg = append(commitMsg, trailers...)
	return strings.Join(commitMsg, "\n")
}

// AddTrailer adds a trailer to the given commit message.
func AddTrailer(commitMsg, trailer string) (string, error) {
	if !trailerRegex.MatchString(trailer) {
		return "", skerr.Fmt("%q is not a valid git trailer", trailer)
	}
	body, trailers := SplitTrailers(commitMsg)
	trailers = append(trailers, trailer)
	return JoinTrailers(body, trailers), nil
}

// FullyQualifiedBranchName ensures that the branch has the refs/heads/ prefix.
func FullyQualifiedBranchName(branch string) string {
	if strings.HasPrefix(branch, git_common.RefsHeadsPrefix) {
		return branch
	}
	return git_common.RefsHeadsPrefix + branch
}

// BranchBaseName ensures that the branch does not have the refs/heads/ prefix.
func BranchBaseName(branch string) string {
	return strings.TrimPrefix(branch, git_common.RefsHeadsPrefix)
}

// GetFootersMap parses the specified commit msg and returns it's footers.
// Invalid footer formats are logged.
// Eg: commit msg: "test test\n\nBug: skia:123\nTested: true" will return
// {"skia": "123", "Tested": "true"}.
func GetFootersMap(commitMsg string) map[string]string {
	footersMap := map[string]string{}
	_, footers := SplitTrailers(commitMsg)
	for _, f := range footers {
		rs := trailerRegex.FindStringSubmatch(f)
		if len(rs) != 3 {
			sklog.Errorf("Could not parse footer %s from the commitMsg %s", f, commitMsg)
			continue
		}
		footersMap[rs[1]] = rs[2]
	}

	return footersMap
}

// GetBoolFooterVal looks for the specified footer in the footersMap and returns
// it's boolean value. If the footer is not found then false is returned.
// If the value is not boolean then false is returned and an error is logged.
func GetBoolFooterVal(footersMap map[string]string, footer string, issue int64) bool {
	if val, ok := footersMap[string(footer)]; ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			sklog.Errorf("Could not parse bool value out of \"%s: %s\" in %d", footer, val, issue)
			return false
		} else {
			if b {
				return b
			}
		}
	}
	return false
}

// GetStringFooterVal looks for the specified footer in the footersMap and returns
// it's strings value. If the footer is not found then an empty string is
// returned.
func GetStringFooterVal(footersMap map[string]string, footer string) string {
	if val, ok := footersMap[string(footer)]; ok {
		return val
	}
	return ""
}
