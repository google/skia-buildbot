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
	"go.skia.org/infra/go/sklog"
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

// LogBuilder returns a URLBuilder used to perform a query equivalent to "git log".
func (r *Repo) LogBuilder() *URLBuilder {
	return &URLBuilder{
		repo:      r,
		directive: "log",
	}
}

// URLBuilder helps to construct API URLs for Gitiles.
type URLBuilder struct {
	repo         *Repo
	directive    string
	firstParent  bool
	revisionExpr string
	path         string
	query        map[string]string
	limit        int
}

// Revision sets the Revision at which the Log should be based.
func (b *URLBuilder) Revision(revision string) *URLBuilder {
	b.revisionExpr = revision
	return b
}

// LogFromTo indicates that the Log should run from the given starting commit to
// the given ending commit.
func (b *URLBuilder) LogFromTo(start, end string) *URLBuilder {
	b.revisionExpr = git.LogFromTo(start, end)
	return b
}

// Reverse is a LogOption which indicates that the commits in the Log should
// be returned in reverse order from the typical "git log" ordering, ie. each
// commit's parents appear before the commit itself.
func (b *URLBuilder) Reverse() *URLBuilder {
	b.query["reverse"] = "true"
	return b
}

// BatchSize sets the number of commits to load at a time.
func (b *URLBuilder) BatchSize(n int) *URLBuilder {
	b.query["n"] = strconv.Itoa(n)
	return b
}

// Limit is a LogOption which makes Log return at most N commits.
func (b *URLBuilder) Limit(n int) *URLBuilder {
	b.limit = n
	return b
}

// Path is a LogOption which limits the git log to the given path.
func (b *URLBuilder) Path(path string) *URLBuilder {
	b.path = path
	return b
}

// StartAt indicates that the log should start at the given commit.
func (b *URLBuilder) StartAt(commit string) *URLBuilder {
	b.query["s"] = commit
	return b
}

// FirstParent indicates that the returned log should be filtered to be
// equivalent to "git log --first-parent --ancestry-path".
func (b *URLBuilder) FirstParent() *URLBuilder {
	b.firstParent = true
	return b
}

// String constructs a URL based on the options applied to the URLBuilder.
func (b *URLBuilder) String() string {
	parts := []string{b.repo.URL, "+" + b.directive}
	if b.revisionExpr != "" {
		parts = append(parts, b.revisionExpr)
	}
	if b.path != "" {
		parts = append(parts, b.path)
	}
	url := strings.Join(parts, "/")
	query := []string{}
	if len(b.query) > 0 {
		for k, v := range b.query {
			query = append(query, fmt.Sprintf("%s=%s", k, v))
		}
	}
	// If no batch size was specified, use the specified limit, if any.
	if _, ok := b.query["n"]; !ok && b.limit > 0 {
		query = append(query, fmt.Sprintf("n=%d", b.limit))
	}
	if len(query) > 0 {
		// Sort for consistency in testing.
		sort.Strings(query)
		url += "?" + strings.Join(query, "&")
	}
	return url
}

// GitLogArgs returns set of arguments which produce an equivalent "git log" to
// the current query.
func (b *URLBuilder) GitLogArgs() []string {
	rv := []string{}

}

// DoBatches performs a sequence of requests, retrieving batches of commits and
// calling the given function for each.  If the callback returns an error,
// iteration stops.  If the error is anything other than ErrStopIteration,
// DoBatches returns that error.  DoBatches invalidates the URLBuilder.
func (b *URLBuilder) DoBatches(ctx context.Context, callback func(context.Context, []*vcsinfo.LongCommit) error) error {
	sklog.Errorf("URL (pre): ", b.String())

	// Load commits in batches.
	seen := 0
	for {
		var l Log
		u := b.String()
		if err := b.repo.getJSON(ctx, u, &l); err != nil {
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
			if b.limit > 0 && seen == b.limit {
				break
			}
		}
		if err := callback(ctx, commits); err == ErrStopIteration {
			return nil
		} else if err != nil {
			return err
		}
		if l.Next == "" || (b.limit > 0 && seen >= b.limit) {
			return nil
		}
		b.StartAt(l.Next)
	}
}

// Do performs the log request and returns all commits. Do invalidates the
// URLBuilder.
func (b *URLBuilder) Do(ctx context.Context) ([]*vcsinfo.LongCommit, error) {
	rv := []*vcsinfo.LongCommit{}
	if err := b.DoBatches(ctx, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		rv = append(rv, commits...)
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// LogFirstParent is equivalent to "git log --first-parent A..B", ie. it
// only returns commits which are reachable from A by following the first parent
// (the "main" branch) but not from B.
func (r *Repo) LogFirstParent(ctx context.Context, from, to string, b *URLBuilder) ([]*vcsinfo.LongCommit, error) {
	// Retrieve the normal "git log".
	commits, err := b.Revision(git.LogFromTo(from, to)).Do(ctx)
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
func (r *Repo) LogLinear(ctx context.Context, from, to string) ([]*vcsinfo.LongCommit, error) {
	// Retrieve the normal "git log".
	commits, err := r.LogBuilder().Revision(git.LogFromTo(from, to)).Do(ctx)
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
