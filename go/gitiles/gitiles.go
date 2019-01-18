package gitiles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

/*
	Utilities for working with Gitiles.
*/

const (
	COMMIT_URL        = "%s/+/%s?format=JSON"
	DATE_FORMAT_NO_TZ = "Mon Jan 02 15:04:05 2006"
	DATE_FORMAT_TZ    = "Mon Jan 02 15:04:05 2006 -0700"
	DOWNLOAD_URL      = "%s/+/%s/%s?format=TEXT"
	LOG_URL           = "%s/+log/%s..%s?format=JSON"
)

// Repo is an object used for interacting with a single Git repo using Gitiles.
type Repo struct {
	client         *http.Client
	gitCookiesPath string
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
		URL:            url,
	}
}

// get executes a GET request to the given URL, returning the http.Response.
func (r *Repo) get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
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

// ReadFileAtRef reads the given file at the given ref.
func (r *Repo) ReadFileAtRef(srcPath, ref string, w io.Writer) error {
	resp, err := r.get(fmt.Sprintf(DOWNLOAD_URL, r.URL, ref, srcPath))
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
func (r *Repo) ReadFile(srcPath string, w io.Writer) error {
	return r.ReadFileAtRef(srcPath, "master", w)
}

// DownloadFile downloads the current version of the given file from the master
// branch of the Repo.
func (r *Repo) DownloadFile(srcPath, dstPath string) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer util.Close(f)
	if err := r.ReadFile(srcPath, f); err != nil {
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

// GetCommit returns a vcsinfo.LongCommit for the given commit.
func (r *Repo) GetCommit(ref string) (*vcsinfo.LongCommit, error) {
	resp, err := r.get(fmt.Sprintf(COMMIT_URL, r.URL, ref))
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %s", err)
	}
	// Remove the first line.
	b = b[4:]
	var c Commit
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return commitToLongCommit(&c)
}

// Log returns Gitiles' equivalent to "git log" for the given start and end
// commits.
func (r *Repo) Log(from, to string) ([]*vcsinfo.LongCommit, error) {
	rv := []*vcsinfo.LongCommit{}
	for {
		resp, err := r.get(fmt.Sprintf(LOG_URL, r.URL, from, to))
		if err != nil {
			return nil, err
		}
		defer util.Close(resp.Body)
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to read response: %s", err)
		}
		// Remove the first line.
		b = b[4:]
		var l Log
		if err := json.Unmarshal(b, &l); err != nil {
			return nil, fmt.Errorf("Failed to decode response: %s", err)
		}
		// Convert to vcsinfo.LongCommit.
		for _, c := range l.Log {
			vc, err := commitToLongCommit(c)
			if err != nil {
				return nil, err
			}
			rv = append(rv, vc)
		}
		if l.Next == "" {
			break
		} else {
			to = l.Next
		}
	}
	return rv, nil
}

// LogLinear is equivalent to "git log --first-parent --ancestry-path from..to",
// ie. it only returns commits which are on the direct path from A to B, and
// only on the "main" branch. This is as opposed to "git log from..to" which
// returns all commits which are ancestors of 'to' but not 'from'.
func (r *Repo) LogLinear(from, to string) ([]*vcsinfo.LongCommit, error) {
	// Retrieve the normal "git log".
	commits, err := r.Log(from, to)
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
