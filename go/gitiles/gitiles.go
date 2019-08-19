package gitiles

import (
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
	"strings"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/time/rate"
)

/*
	Utilities for working with Gitiles.
*/

const (
	COMMIT_URL        = "%s/+/%s?format=JSON"
	DATE_FORMAT_NO_TZ = "Mon Jan 02 15:04:05 2006"
	DATE_FORMAT_TZ    = "Mon Jan 02 15:04:05 2006 -0700"
	DOWNLOAD_URL      = "%s/+/%s/%s?format=TEXT"
	LOG_URL           = "%s/+log/%s?format=JSON"
	REFS_URL          = "%s/+refs%%2Fheads?format=JSON"
	TAGS_URL          = "%s/+refs%%2Ftags?format=JSON"

	// These were copied from the defaults used by gitfs:
	// https://gerrit.googlesource.com/gitfs/+/59c1163fd1737445281f2339399b2b986b0d30fe/gitiles/client.go#102
	MAX_QPS   = rate.Limit(4.0)
	MAX_BURST = 40
)

var (
	ErrStopIteration = errors.New("stop iteration")
)

// Repo is an object used for interacting with a single Git repo using Gitiles.
type Repo struct {
	client         *http.Client
	gitCookiesPath string
	rl             *rate.Limiter
	URL            string
}

// NewRepo creates and returns a new Repo object.
func NewRepo(url string, gitCookiesPath string, c *http.Client) *Repo {
	if c == nil {
		c = httputils.NewTimeoutClient()
	}
	return &Repo{
		client:         c,
		gitCookiesPath: gitCookiesPath,
		rl:             rate.NewLimiter(MAX_QPS, MAX_BURST),
		URL:            url,
	}
}

// get executes a GET request to the given URL, returning the http.Response.
func (r *Repo) get(ctx context.Context, url string) (*http.Response, error) {
	// Respect the rate limit.
	if err := r.rl.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if r.gitCookiesPath != "" {
		if err := gitauth.AddAuthenticationCookie(r.gitCookiesPath, req); err != nil {
			return nil, err
		}
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		util.Close(resp.Body)
		return nil, fmt.Errorf("Request got status %q", resp.Status)
	}
	return resp, nil
}

// getJson executes a GET request to the given URL, reads the response and
// unmarshals it to the given destination.
func (r *Repo) getJson(ctx context.Context, url string, dest interface{}) error {
	resp, err := r.get(ctx, url)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response: %s", err)
	}
	// Remove the first line.
	b = b[4:]
	return json.Unmarshal(b, dest)
}

// ReadFileAtRef reads the given file at the given ref.
func (r *Repo) ReadFileAtRef(ctx context.Context, srcPath, ref string, w io.Writer) error {
	resp, err := r.get(ctx, fmt.Sprintf(DOWNLOAD_URL, r.URL, ref, srcPath))
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	d := base64.NewDecoder(base64.StdEncoding, resp.Body)
	if _, err := io.Copy(w, d); err != nil {
		return err
	}
	return nil
}

// ReadFile reads the current version of the given file from the master branch
// of the Repo.
func (r *Repo) ReadFile(ctx context.Context, srcPath string, w io.Writer) error {
	return r.ReadFileAtRef(ctx, srcPath, "master", w)
}

// DownloadFile downloads the current version of the given file from the master
// branch of the Repo.
func (r *Repo) DownloadFile(ctx context.Context, srcPath, dstPath string) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer util.Close(f)
	if err := r.ReadFile(ctx, srcPath, f); err != nil {
		return err
	}
	return nil
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Time  string `json:"time"`
}

type Commit struct {
	Commit    string   `json:"commit"`
	Parents   []string `json:"parents"`
	Author    *Author  `json:"author"`
	Committer *Author  `json:"committer"`
	Message   string   `json:"message"`
}

type Log struct {
	Log  []*Commit `json:"log"`
	Next string    `json:"next"`
}

func commitToLongCommit(c *Commit) (*vcsinfo.LongCommit, error) {
	var ts time.Time
	var err error
	if strings.Contains(c.Committer.Time, " +") || strings.Contains(c.Committer.Time, " -") {
		ts, err = time.Parse(DATE_FORMAT_TZ, c.Committer.Time)
	} else {
		ts, err = time.Parse(DATE_FORMAT_NO_TZ, c.Committer.Time)
	}
	if err != nil {
		return nil, err
	}

	split := strings.Split(c.Message, "\n")
	subject := split[0]
	split = split[1:]
	body := ""
	if len(split) > 1 && split[0] == "" {
		split = split[1:]
	}
	if len(split) > 1 {
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

// Details returns a vcsinfo.LongCommit for the given commit.
func (r *Repo) Details(ctx context.Context, ref string) (*vcsinfo.LongCommit, error) {
	var c Commit
	if err := r.getJson(ctx, fmt.Sprintf(COMMIT_URL, r.URL, ref), &c); err != nil {
		return nil, err
	}
	return commitToLongCommit(&c)
}

// LogOption represents an optional parameter to a Log function.
type LogOption string

// LogReverse is a LogOption which indicates that the commits in the Log should
// be returned in reverse order from the typical "git log" ordering, ie. each
// commit's parents appear before the commit itself.
func LogReverse() LogOption {
	return LogOption("reverse=true")
}

// LogBatchSize is a LogOption which indicates the number of commits which
// should be included in each batch of commits returned by Log.
func LogBatchSize(n int) LogOption {
	return LogOption(fmt.Sprintf("n=%d", n))
}

// logHelper is used to perform requests which are equivalent to "git log".
// Loads commits in batches and calls the given function for each batch of
// commits. If the function returns an error, iteration stops, and the error is
// returned, unless it was ErrStopIteration.
func (r *Repo) logHelper(ctx context.Context, url string, fn func(context.Context, []*vcsinfo.LongCommit) error, opts ...LogOption) error {
	for _, opt := range opts {
		url += "&" + string(opt)
	}
	start := ""
	for {
		var l Log
		u := url
		if start != "" {
			u += "&s=" + start
		}
		if err := r.getJson(ctx, u, &l); err != nil {
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
		}
		if err := fn(ctx, commits); err == ErrStopIteration {
			return nil
		} else if err != nil {
			return err
		}
		if l.Next == "" {
			return nil
		} else {
			start = l.Next
		}
	}
}

// Log returns Gitiles' equivalent to "git log" for the given expression.
func (r *Repo) Log(ctx context.Context, logExpr string, opts ...LogOption) ([]*vcsinfo.LongCommit, error) {
	rv := []*vcsinfo.LongCommit{}
	url := fmt.Sprintf(LOG_URL, r.URL, logExpr)
	if err := r.logHelper(ctx, url, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		rv = append(rv, commits...)
		return nil
	}, opts...); err != nil {
		return nil, err
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
		if search(commit.Parents[0]) {
			isDescendant[hash] = true
			return true
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
	url := fmt.Sprintf(LOG_URL, r.URL, logExpr)
	return r.logHelper(ctx, url, fn, opts...)
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
	if err := r.getJson(ctx, fmt.Sprintf(REFS_URL, r.URL), &branchMap); err != nil {
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
	if err := r.getJson(ctx, fmt.Sprintf(TAGS_URL, r.URL), &tags); err != nil {
		return nil, err
	}
	rv := make(map[string]string, len(tags))
	for k, tag := range tags {
		rv[k] = tag.Value
	}
	return rv, nil
}
