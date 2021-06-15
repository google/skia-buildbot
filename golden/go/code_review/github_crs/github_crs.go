// Package github_crs provides a client for Gold's interaction with
// the GitHub code review system.
package github_crs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/code_review"
	"golang.org/x/time/rate"
)

const (
	// Authenticated clients can do up to 5000 queries per hour. These limits
	// are conservative based on that.
	maxQPS   = rate.Limit(1)
	maxBurst = 100
)

type CRSImpl struct {
	client *http.Client
	rl     *rate.Limiter
	repo   string
}

// New returns a new instance of CRSImpl, ready to target a single
// GitHub repo. repo should be the user/repo, e.g. "google/skia",
// "flutter/flutter", etc.
func New(client *http.Client, repo string) *CRSImpl {
	return &CRSImpl{
		client: client,
		rl:     rate.NewLimiter(maxQPS, maxBurst),
		repo:   repo,
	}
}

type user struct {
	UserName string `json:"login"`
}

// See https://developer.github.com/v3/pulls/#get-a-single-pull-request
type pullRequestResponse struct {
	Title   string `json:"title"`
	User    user   `json:"user"`
	State   string `json:"state"`
	Updated string `json:"updated_at"` // e.g.  "2011-01-26T19:01:12Z"
	Merged  string `json:"merged_at"`
}

// GetChangelist implements the code_review.Client interface.
func (c *CRSImpl) GetChangelist(ctx context.Context, id string) (code_review.Changelist, error) {
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return code_review.Changelist{}, skerr.Fmt("invalid Changelist ID")
	}
	// Respect the rate limit.
	if err := c.rl.Wait(ctx); err != nil {
		return code_review.Changelist{}, skerr.Wrap(err)
	}
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%s", c.repo, id)
	resp, err := httputils.GetWithContext(ctx, c.client, u)
	if err != nil {
		sklog.Errorf("Error getting Changelist from %s: %s", u, err)
		// Assume an error here is the Changelist is not found
		return code_review.Changelist{}, code_review.ErrNotFound
	}
	defer util.Close(resp.Body)

	var prr pullRequestResponse
	err = json.NewDecoder(resp.Body).Decode(&prr)
	if err != nil {
		return code_review.Changelist{}, skerr.Wrapf(err, "received invalid JSON from GitHub: %s", u)
	}

	state := code_review.Open
	if prr.State == "closed" {
		if prr.Merged != "" {
			state = code_review.Landed
		} else {
			state = code_review.Abandoned
		}
	}

	updated, err := time.Parse(time.RFC3339, prr.Updated)
	if err != nil {
		return code_review.Changelist{}, skerr.Wrapf(err, "invalid time %q", prr.Updated)
	}

	return code_review.Changelist{
		SystemID: id,
		Owner:    prr.User.UserName,
		Subject:  prr.Title,
		Status:   state,
		Updated:  updated,
	}, nil
}

type commit struct {
	Hash   string     `json:"sha"`
	Commit commitInfo `json:"commit"`
}

type commitInfo struct {
	Committer person `json:"committer"`
}

type person struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	DateInRFC3339 string `json:"date"`
}

// https://developer.github.com/v3/pulls/#list-commits-on-a-pull-request
type commitsOnPullRequestResponse []commit

// GetPatchset implements the code_review.Client interface.
func (c *CRSImpl) GetPatchset(ctx context.Context, clID, psID string, psOrder int) (code_review.Patchset, error) {
	if _, err := strconv.ParseInt(clID, 10, 64); err != nil {
		return code_review.Patchset{}, skerr.Fmt("invalid Changelist ID")
	}
	// At the moment, paging returns 30 at a time and the API docs say that it stops after
	// 250 commits. Just to be safe, we should bail out of page is more than 20 (~600 patchsets) in
	// an effort to not hang on some unanticipated response. Normally we break when the API
	// gives us 0 responses for a page, indicating we have all the patchsets.
	order := 0
	for page := 1; page < 20; page++ {
		// Respect the rate limit.
		if err := c.rl.Wait(ctx); err != nil {
			return code_review.Patchset{}, skerr.Wrap(err)
		}
		u := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%s/commits?page=%d", c.repo, clID, page)
		resp, err := httputils.GetWithContext(ctx, c.client, u)
		if err != nil {
			sklog.Errorf("Error getting commits on PR %s with url %s: %s", clID, u, err)
			// Assume an error here is the Changelist is not found
			return code_review.Patchset{}, code_review.ErrNotFound
		}
		var cprr commitsOnPullRequestResponse
		err = json.NewDecoder(resp.Body).Decode(&cprr)
		if err != nil {
			util.Close(resp.Body)
			return code_review.Patchset{}, skerr.Wrapf(err, "received invalid JSON from GitHub: %s", u)
		}
		util.Close(resp.Body)

		// Assume GitHub returns these in ascending order
		for _, ps := range cprr {
			order++
			if psOrder == order || psID == ps.Hash {
				ts, err := time.Parse(time.RFC3339, ps.Commit.Committer.DateInRFC3339)
				if err != nil {
					return code_review.Patchset{}, skerr.Wrapf(err, "parsing date on PR %s commit %s: %#v", clID, psID, ps.Commit)
				}
				return code_review.Patchset{
					SystemID:     ps.Hash,
					ChangelistID: clID,
					Order:        order,
					GitHash:      ps.Hash,
					Created:      ts,
				}, nil
			}
		}
		if len(cprr) == 0 {
			break
		}
	}
	return code_review.Patchset{}, code_review.ErrNotFound
}

// GetChangelistIDForCommit implements the code_review.Client interface.
func (c *CRSImpl) GetChangelistIDForCommit(ctx context.Context, commit *vcsinfo.LongCommit) (string, error) {
	if commit == nil {
		return "", skerr.Fmt("commit cannot be nil")
	}
	id, err := extractPRFromTitle(commit.Subject)
	if err != nil {
		sklog.Debugf("Could not find github issue: %s", err)
		return "", code_review.ErrNotFound
	}
	return id, nil
}

// We assume a PR has the pull request number in the Subject/Title, at the end.
// e.g. "Turn off docs upload temporarily (#44365) (#44413)" refers to PR 44413
var prSuffix = regexp.MustCompile(`.+\(#(?P<id>\d+)\)\s*$`)

// extractPRFromTitle returns the pull request id extracted from the title
// of a landed PR, or an error if it cannot.
func extractPRFromTitle(t string) (string, error) {
	if match := prSuffix.FindStringSubmatch(t); match != nil {
		// match[0] is the whole string, match[1] is the first group
		return match[1], nil
	}
	return "", skerr.Fmt("Could not find PR in Subject %q", t)
}

// CommentOn implements the code_review.Client interface.
// https://developer.github.com/v3/issues/comments/#create-a-comment
func (c *CRSImpl) CommentOn(ctx context.Context, clID, message string) error {
	sklog.Infof("Commenting on GitHub CL (PR) %s with message %q", clID, message)
	if _, err := strconv.ParseInt(clID, 10, 64); err != nil {
		return skerr.Fmt("invalid Changelist ID")
	}
	// Respect the rate limit.
	if err := c.rl.Wait(ctx); err != nil {
		return skerr.Wrap(err)
	}
	u := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", c.repo, clID)
	j := fmt.Sprintf(`{"body":%q}`, message)
	_, err := httputils.PostWithContext(ctx, c.client, u, "application/json", strings.NewReader(j))
	return skerr.Wrap(err)
}

// System implements the code_review.Client interface.
func (c *CRSImpl) System() string {
	return "github"
}

// Make sure CRSImpl fulfills the code_review.Client interface.
var _ code_review.Client = (*CRSImpl)(nil)
