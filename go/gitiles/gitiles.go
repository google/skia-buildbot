package gitiles

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/time/rate"
)

/*
	Utilities for working with Gitiles.
*/

const (
	// CommitURL is the format of the URL used to retrieve a commit.
	CommitURL = "%s/+show/%s"
	// CommitURLJSON is the format of the URL used to retrieve a commit as JSON.
	CommitURLJSON = CommitURL + "?format=JSON"

	// DownloadURL is the format of the URL used to download a file.
	DownloadURL = "%s/+show/%s/%s?format=TEXT"
	// LogURL is the format of the URL used to view the git log.
	LogURL = "%s/+log/%s?format=JSON"
	// RefsURL is the format of the URL used to retrieve refs.
	RefsURL = "%s/+refs%%2Fheads?format=JSON"
	// TagsURL is the format of the URL used to retrieve tags.
	TagsURL = "%s/+refs%%2Ftags?format=JSON"

	dateFormatNoTZ = "Mon Jan 02 15:04:05 2006"
	dateFormatTZ   = "Mon Jan 02 15:04:05 2006 -0700"

	// These were copied from the defaults used by gitfs:
	// https://gerrit.googlesource.com/gitfs/+show/59c1163fd1737445281f2339399b2b986b0d30fe/gitiles/client.go#102
	maxQPS   = rate.Limit(4.0)
	maxBurst = 40

	// ModeHeader is an HTTP header which indicates the file mode.
	ModeHeader = "X-Gitiles-Path-Mode"
	// TypeHeader is an HTTP header which indicates the object type.
	TypeHeader = "X-Gitiles-Object-Type"
)

var (
	// ErrStopIteration is an error returned from a helper function passed to
	// LogFn which indicates that iteration over commits should stop.
	ErrStopIteration = errors.New("stop iteration")
)

// Repo is an object used for interacting with a single Git repo using Gitiles.
type Repo struct {
	client *http.Client
	rl     *rate.Limiter
	URL    string
}

// NewRepo creates and returns a new Repo object.
func NewRepo(url string, c *http.Client) *Repo {
	// TODO(borenet):Stop supporting a nil client; we should enforce that we
	// always use an authenticated client to talk to Gitiles.
	if c == nil {
		c = httputils.NewTimeoutClient()
	}
	return &Repo{
		client: c,
		rl:     rate.NewLimiter(maxQPS, maxBurst),
		URL:    url,
	}
}

// get executes a GET request to the given URL, returning the http.Response.
func (r *Repo) get(ctx context.Context, url string) (*http.Response, error) {
	// Respect the rate limit.
	if err := r.rl.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := httputils.GetWithContext(ctx, r.client, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		util.Close(resp.Body)
		return nil, skerr.Fmt("Request got status %q", resp.Status)
	}
	return resp, nil
}

// getJSON executes a GET request to the given URL, reads the response and
// unmarshals it to the given destination.
func (r *Repo) getJSON(ctx context.Context, url string, dest interface{}) error {
	resp, err := r.get(ctx, url)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return skerr.Fmt("Failed to read response: %s", err)
	}
	// Remove the first line.
	b = b[4:]
	return skerr.Wrap(json.Unmarshal(b, dest))
}

// ReadObject reads the given object at the given ref, returning its contents
// and FileInfo.
func (r *Repo) ReadObject(ctx context.Context, path, ref string) (os.FileInfo, []byte, error) {
	path = strings.TrimSuffix(path, "/")
	resp, err := r.get(ctx, fmt.Sprintf(DownloadURL, r.URL, ref, path))
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	var buf bytes.Buffer
	d := base64.NewDecoder(base64.StdEncoding, resp.Body)
	if _, err := io.Copy(&buf, d); err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	content := buf.Bytes()
	mh := resp.Header.Get(ModeHeader)
	typ := resp.Header.Get(TypeHeader)
	fi, err := git.MakeFileInfo(path, mh, git.ObjectType(typ), len(content))
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return fi, content, nil
}

// ReadFileAtRef reads the given file at the given ref.
func (r *Repo) ReadFileAtRef(ctx context.Context, srcPath, ref string) ([]byte, error) {
	_, rv, err := r.ReadObject(ctx, srcPath, ref)
	return rv, err
}

// ReadFile reads the current version of the given file from the master branch
// of the Repo.
func (r *Repo) ReadFile(ctx context.Context, srcPath string) ([]byte, error) {
	return r.ReadFileAtRef(ctx, srcPath, "master")
}

// DownloadFile downloads the current version of the given file from the master
// branch of the Repo.
func (r *Repo) DownloadFile(ctx context.Context, srcPath, dstPath string) error {
	return util.WithWriteFile(dstPath, func(w io.Writer) error {
		contents, err := r.ReadFile(ctx, srcPath)
		if err != nil {
			return skerr.Wrap(err)
		}
		_, err = w.Write(contents)
		return skerr.Wrap(err)
	})
}

// DownloadFileAtRef downloads the given file at the given ref.
func (r *Repo) DownloadFileAtRef(ctx context.Context, srcPath, ref, dstPath string) error {
	return util.WithWriteFile(dstPath, func(w io.Writer) error {
		contents, err := r.ReadFileAtRef(ctx, srcPath, ref)
		if err != nil {
			return skerr.Wrap(err)
		}
		_, err = w.Write(contents)
		return skerr.Wrap(err)
	})
}

// ListDirAtRef reads the given directory at the given ref. Returns a slice of
// file names and a slice of dir names, relative to the given directory, or any
// error which occurred.
func (r *Repo) ListDirAtRef(ctx context.Context, dir, ref string) ([]os.FileInfo, error) {
	_, contents, err := r.ReadObject(ctx, dir, ref)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return git.ParseDir(contents)
}

// ListDir reads the given directory on the master branch. Returns a slice of
// file names and a slice of dir names, relative to the given directory, or any
// error which occurred.
func (r *Repo) ListDir(ctx context.Context, dir string) ([]os.FileInfo, error) {
	return r.ListDirAtRef(ctx, dir, "master")
}

// ResolveRef resolves the given ref to a commit hash.
func (r *Repo) ResolveRef(ctx context.Context, ref string) (string, error) {
	commit, err := r.Details(ctx, ref)
	if err != nil {
		return "", err
	}
	return commit.Hash, nil
}

// ListFilesRecursiveAtRef returns a list of all file paths, relative to the
// given directory, under the given directory at the given ref.
func (r *Repo) ListFilesRecursiveAtRef(ctx context.Context, topDir, ref string) ([]string, error) {
	// First, resolve the given ref to a commit hash to ensure that we
	// return consistent results even if the ref changes between requests.
	hash, err := r.ResolveRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	// List files recursively.
	rv := []string{}
	var helper func(string) error
	helper = func(dir string) error {
		infos, err := r.ListDirAtRef(ctx, dir, hash)
		if err != nil {
			return err
		}
		for _, fi := range infos {
			if fi.IsDir() {
				if err := helper(dir + "/" + fi.Name()); err != nil {
					return err
				}
			} else {
				rv = append(rv, strings.TrimPrefix(dir+"/"+fi.Name(), topDir+"/"))
			}
		}
		return nil
	}
	if err := helper(topDir); err != nil {
		return nil, err
	}
	sort.Strings(rv)
	return rv, nil
}

// ListFilesRecursive returns a list of all file paths, relative to the given
// directory, under the given directory on the master branch.
func (r *Repo) ListFilesRecursive(ctx context.Context, dir string) ([]string, error) {
	return r.ListFilesRecursiveAtRef(ctx, dir, "master")
}

// Author represents the author of a Commit.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Time  string `json:"time"`
}

// TreeDiff represents a change to a file in a Commit.
type TreeDiff struct {
	// Type can be one of Copy, Rename, Add, Delete, Modify.
	Type string `json:"type"`
	// Previous location of the changed file.
	OldPath string `json:"old_path"`
	// New location of the changed file.
	NewPath string `json:"new_path"`
}

// Commit contains information about one Git commit.
type Commit struct {
	Commit    string      `json:"commit"`
	Parents   []string    `json:"parents"`
	Author    *Author     `json:"author"`
	Committer *Author     `json:"committer"`
	Message   string      `json:"message"`
	TreeDiffs []*TreeDiff `json:"tree_diff"`
}

// Log represents a series of Commits in the git log.
type Log struct {
	Log  []*Commit `json:"log"`
	Next string    `json:"next"`
}

func commitToLongCommit(c *Commit) (*vcsinfo.LongCommit, error) {
	var ts time.Time
	var err error
	if strings.Contains(c.Committer.Time, " +") || strings.Contains(c.Committer.Time, " -") {
		ts, err = time.Parse(dateFormatTZ, c.Committer.Time)
	} else {
		ts, err = time.Parse(dateFormatNoTZ, c.Committer.Time)
	}
	if err != nil {
		return nil, err
	}

	split := strings.Split(c.Message, "\n")
	subject := split[0]
	split = split[1:]
	body := ""
	if len(split) > 0 && split[0] == "" {
		split = split[1:]
	}
	if len(split) > 0 {
		body = strings.Join(split, "\n")
	}
	return &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    c.Commit,
			Author:  fmt.Sprintf("%s (%s)", c.Author.Name, c.Author.Email),
			Subject: subject,
		},
		Parents:   c.Parents,
		Body:      body,
		Timestamp: ts,
	}, nil
}

// LongCommitToCommit converts the given LongCommit to a Commit. Intended for
// use in tests.
func LongCommitToCommit(details *vcsinfo.LongCommit) (*Commit, error) {
	// vcsinfo.LongCommit expresses authors in the form: "Author Name (author@email.com)"
	split := strings.Split(details.Author, "(")
	if len(split) != 2 {
		return nil, skerr.Fmt("Bad author format: %q", details.Author)
	}
	authorName := strings.TrimSpace(split[0])
	authorEmail := strings.TrimSpace(strings.TrimRight(split[1], ")"))
	return &Commit{
		Commit:  details.Hash,
		Parents: details.Parents,
		Author: &Author{
			Name:  authorName,
			Email: authorEmail,
			Time:  details.Timestamp.Format(dateFormatTZ),
		},
		Committer: &Author{
			Name:  authorName,
			Email: authorEmail,
			Time:  details.Timestamp.Format(dateFormatTZ),
		},
		Message: details.Subject + "\n\n" + details.Body,
	}, nil
}

// getCommit returns a Commit for the given ref.
func (r *Repo) getCommit(ctx context.Context, ref string) (*Commit, error) {
	var c Commit
	if err := r.getJSON(ctx, fmt.Sprintf(CommitURLJSON, r.URL, ref), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Details returns a vcsinfo.LongCommit for the given commit.
func (r *Repo) Details(ctx context.Context, ref string) (*vcsinfo.LongCommit, error) {
	c, err := r.getCommit(ctx, ref)
	if err != nil {
		return nil, err
	}
	return commitToLongCommit(c)
}

// GetTreeDiffs returns a slice of TreeDiffs for the given commit.
func (r *Repo) GetTreeDiffs(ctx context.Context, ref string) ([]*TreeDiff, error) {
	c, err := r.getCommit(ctx, ref)
	if err != nil {
		return nil, err
	}
	return c.TreeDiffs, nil
}

// LogOption represents an optional parameter to a Log function. Either Key()
// AND Value() OR Path() must return non-empty strings. Only one LogOption in
// a given set may return a non-empty value for Path().
type LogOption interface {
	Key() string
	Value() string
	Path() string
}

type stringLogOption [2]string

func (s stringLogOption) Key() string {
	return s[0]
}

func (s stringLogOption) Value() string {
	return s[1]
}

func (s stringLogOption) Path() string {
	return ""
}

// LogReverse is a LogOption which indicates that the commits in the Log should
// be returned in reverse order from the typical "git log" ordering, ie. each
// commit's parents appear before the commit itself.
func LogReverse() LogOption {
	return stringLogOption([2]string{"reverse", "true"})
}

// LogBatchSize is a LogOption which indicates the number of commits which
// should be included in each batch of commits returned by Log.
func LogBatchSize(n int) LogOption {
	return stringLogOption([2]string{logLimit(0).Key(), strconv.Itoa(n)})
}

// logLimit is an implementation of LogOption which is a special case, because
// Gitiles' limit option is really just a batch size. We need a new type to
// indicate that we shouldn't load additional batches after the first N commits.
type logLimit int

func (n logLimit) Key() string {
	return "n"
}

func (n logLimit) Value() string {
	return strconv.Itoa(int(n))
}

func (n logLimit) Path() string {
	return ""
}

// LogLimit is a LogOption which makes Log return at most N commits.
func LogLimit(n int) LogOption {
	return logLimit(n)
}

// logPath restricts the log to a given path.
type logPath string

func (p logPath) Key() string {
	return ""
}

func (p logPath) Value() string {
	return ""
}

func (p logPath) Path() string {
	return string(p)
}

// LogPath is a LogOption which limits the git log to the given path.
func LogPath(path string) LogOption {
	return logPath(path)
}

// LogOptionsToQuery converts the given LogOptions to a URL sub-path and query
// string. Returns the URL sub-path and query string and the maximum number of
// commits to return from a Log query (or zero if none is provided, indicating
// no limit), or any error which occurred.
func LogOptionsToQuery(opts []LogOption) (string, string, int, error) {
	limit := 0
	path := ""
	query := ""
	if len(opts) > 0 {
		paramsMap := make(map[string]string, len(opts))
		for _, opt := range opts {
			optPath := opt.Path()
			if optPath != "" {
				if path != "" {
					return "", "", 0, skerr.Fmt("Only one log option may change the URL path")
				}
				path = optPath
			}
			if opt.Key() == "" || opt.Value() == "" {
				continue
			}
			// If LogLimit and LogBatchSize are both provided, or if
			// LogBatchSize is provided more than once, use the
			// smaller value. This ensures that we respect the batch
			// size when smaller than the limit but prevent loading
			// extra commits when the limit is smaller than the
			// batch size.
			// NOTE: We could try to be more efficient and ensure
			// that the final batch contains only as many commits as
			// we need to achieve the given limit. That would
			// require moving this logic into the loop below.
			if exist, ok := paramsMap[opt.Key()]; ok && opt.Key() == logLimit(0).Key() {
				existInt, err := strconv.Atoi(exist)
				if err != nil {
					// This shouldn't happen, since we used
					// strconv.Itoi to create it.
					return "", "", 0, skerr.Wrap(err)
				}
				newInt, err := strconv.Atoi(opt.Value())
				if err != nil {
					// This shouldn't happen, since we used
					// strconv.Itoi to create it.
					return "", "", 0, skerr.Wrap(err)
				}
				if newInt < existInt {
					paramsMap[opt.Key()] = opt.Value()
				}
			} else {
				paramsMap[opt.Key()] = opt.Value()
			}
			if n, ok := opt.(logLimit); ok {
				limit = int(n)
			}
		}
		params := make([]string, 0, len(paramsMap))
		for k, v := range paramsMap {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(params) // For consistency in tests.
		query = strings.Join(params, "&")
	}
	return path, query, limit, nil
}

// logHelper is used to perform requests which are equivalent to "git log".
// Loads commits in batches and calls the given function for each batch of
// commits. If the function returns an error, iteration stops, and the error is
// returned, unless it was ErrStopIteration.
func (r *Repo) logHelper(ctx context.Context, logExpr string, fn func(context.Context, []*vcsinfo.LongCommit) error, opts ...LogOption) error {
	// Build the query parameters.
	path, query, limit, err := LogOptionsToQuery(opts)
	if err != nil {
		return err
	}
	if path != "" {
		logExpr += "/" + path
	}
	url := fmt.Sprintf(LogURL, r.URL, logExpr)
	if query != "" {
		url += "&" + query
	}

	// Load commits in batches.
	seen := 0
	start := ""
	for {
		var l Log
		u := url
		if start != "" {
			u += "&s=" + start
		}
		if err := r.getJSON(ctx, u, &l); err != nil {
			return err
		}
		// Convert to vcsinfo.LongCommit.
		commits := make([]*vcsinfo.LongCommit, 0, len(l.Log))
		for _, c := range l.Log {
			vc, err := commitToLongCommit(c)
			if err != nil {
				return err
			}
			commits = append(commits, vc)
			seen++
			if limit > 0 && seen == limit {
				break
			}
		}
		if err := fn(ctx, commits); err == ErrStopIteration {
			return nil
		} else if err != nil {
			return err
		}
		if l.Next == "" || (limit > 0 && seen >= limit) {
			return nil
		}
		start = l.Next
	}
}

// Log returns Gitiles' equivalent to "git log" for the given expression.
func (r *Repo) Log(ctx context.Context, logExpr string, opts ...LogOption) ([]*vcsinfo.LongCommit, error) {
	rv := []*vcsinfo.LongCommit{}
	if err := r.logHelper(ctx, logExpr, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		rv = append(rv, commits...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}
	return rv, nil
}

// LogFirstParent is equivalent to "git log --first-parent A..B", ie. it
// only returns commits which are reachable from A by following the first parent
// (the "main" branch) but not from B.
func (r *Repo) LogFirstParent(ctx context.Context, from, to string, opts ...LogOption) ([]*vcsinfo.LongCommit, error) {
	// Retrieve the normal "git log".
	commits, err := r.Log(ctx, git.LogFromTo(from, to), opts...)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return commits, nil
	}

	// Now filter to only those commits which are on the first-parent path.
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, commit := range commits {
		commitsMap[commit.Hash] = commit
	}
	rv := make([]*vcsinfo.LongCommit, 0, len(commits))
	c := commitsMap[to]
	for c != nil {
		rv = append(rv, c)
		if len(c.Parents) > 0 {
			c = commitsMap[c.Parents[0]]
		} else {
			c = nil
		}
	}
	return rv, nil
}

// LogLinear is equivalent to "git log --first-parent --ancestry-path from..to",
// ie. it only returns commits which are on the direct path from A to B, and
// only on the "main" branch. This is as opposed to "git log from..to" which
// returns all commits which are ancestors of 'to' but not 'from'.
func (r *Repo) LogLinear(ctx context.Context, from, to string, opts ...LogOption) ([]*vcsinfo.LongCommit, error) {
	// Retrieve the normal "git log".
	commits, err := r.Log(ctx, git.LogFromTo(from, to), opts...)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return commits, nil
	}

	// Now filter to only those commits which are on the direct path.
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, commit := range commits {
		commitsMap[commit.Hash] = commit
	}
	isDescendant := make(map[string]bool, len(commits))
	var search func(string) bool
	search = func(hash string) bool {
		// Shortcut if we've already searched this commit.
		if rv, ok := isDescendant[hash]; ok {
			return rv
		}
		// If the commit isn't in our list, we can't include it.
		commit, ok := commitsMap[hash]
		if !ok {
			isDescendant[hash] = false
			return false
		}

		// The commit is on the ancestry path if it is reachable from
		// "to" and a descendant of "from". The former case is handled
		// by vanilla "git log", so we just need to find the commits
		// which have "from" as an ancestor.

		// If "from" is a parent of this commit, it's on the ancestry
		// path.
		if util.In(from, commit.Parents) {
			isDescendant[hash] = true
			return true
		}
		// If the first parent of this commit is on the direct line,
		// then this commit is as well.
		if len(commit.Parents) > 0 {
			if search(commit.Parents[0]) {
				isDescendant[hash] = true
				return true
			}
		}
		return false
	}
	search(commits[0].Hash)
	rv := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, commit := range commits {
		if isDescendant[commit.Hash] {
			rv = append(rv, commit)
		}
	}
	return rv, nil
}

// LogFn runs the given function for each commit in the log for the given
// expression. It stops when ErrStopIteration is returned.
func (r *Repo) LogFn(ctx context.Context, logExpr string, fn func(context.Context, *vcsinfo.LongCommit) error, opts ...LogOption) error {
	return r.LogFnBatch(ctx, logExpr, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		for _, c := range commits {
			if err := fn(ctx, c); err != nil {
				return err
			}
		}
		return nil
	}, opts...)
}

// LogFnBatch is the same as LogFn but it runs the given function over batches
// of commits.
func (r *Repo) LogFnBatch(ctx context.Context, logExpr string, fn func(context.Context, []*vcsinfo.LongCommit) error, opts ...LogOption) error {
	return r.logHelper(ctx, logExpr, fn, opts...)
}

// Ref represents a single ref, as returned by the API.
type Ref struct {
	Value string `json:"value"`
}

// RefsMap is the result of a request to REFS_URL.
type RefsMap map[string]Ref

// Branches returns the list of branches in the repo.
func (r *Repo) Branches(ctx context.Context) ([]*git.Branch, error) {
	branchMap := RefsMap{}
	if err := r.getJSON(ctx, fmt.Sprintf(RefsURL, r.URL), &branchMap); err != nil {
		return nil, err
	}
	rv := make([]*git.Branch, 0, len(branchMap))
	for branch, v := range branchMap {
		rv = append(rv, &git.Branch{
			Name: branch,
			Head: v.Value,
		})
	}
	sort.Sort(git.BranchList(rv))
	return rv, nil
}

// Tags returns the list of tags in the repo. The returned map has tag names as
// keys and commit hashes as values.
func (r *Repo) Tags(ctx context.Context) (map[string]string, error) {
	tags := map[string]struct {
		Value  string `json:"value"`
		Peeled string `json:"peeled"`
	}{}
	if err := r.getJSON(ctx, fmt.Sprintf(TagsURL, r.URL), &tags); err != nil {
		return nil, err
	}
	rv := make(map[string]string, len(tags))
	for k, tag := range tags {
		rv[k] = tag.Value
	}
	return rv, nil
}
