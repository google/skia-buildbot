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
	client *http.Client
	URL    string
}

// NewRepo creates and returns a new Repo object.
func NewRepo(url string, c *http.Client) *Repo {
	if c == nil {
		c = httputils.NewTimeoutClient()
	}
	return &Repo{
		client: c,
		URL:    url,
	}
}

// ReadFileAtRef reads the given file at the given ref.
func (r *Repo) ReadFileAtRef(srcPath, ref string, w io.Writer) error {
	resp, err := r.client.Get(fmt.Sprintf(DOWNLOAD_URL, r.URL, ref, srcPath))
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request got status %q", resp.Status)
	}
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
	Log []*Commit `json:"log"`
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
	resp, err := r.client.Get(fmt.Sprintf(COMMIT_URL, r.URL, ref))
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request got status %q", resp.Status)
	}
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
	resp, err := r.client.Get(fmt.Sprintf(LOG_URL, r.URL, from, to))
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request got status %q", resp.Status)
	}
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
	rv := make([]*vcsinfo.LongCommit, 0, len(l.Log))
	for _, c := range l.Log {
		vc, err := commitToLongCommit(c)
		if err != nil {
			return nil, err
		}
		rv = append(rv, vc)
	}
	return rv, nil
}
