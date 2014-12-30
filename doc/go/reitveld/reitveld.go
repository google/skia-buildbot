// reitveld contains utilities for interacting with Reitveld servers.
package reitveld

/*
  Reitveld has a simple JSON API and for each issue can return JSON of the following form:

  {
   "all_required_reviewers_approved":true,
   "description":"First rough pass at a Markdown server.\n\nBUG=skia:",
   "created":"2014-12-22 20:44:14.508030",
   "cc":[
      "reviews@skia.org"
   ],
   "reviewers":[

   ],
   "owner_email":"jcgregorio@google.com",
   "patchsets":[
      1,
      20001,
      40001,
      60001,
      80001,
      100001,
      120001,
      140001,
      160001,
      180001,
      200001,
      220001,
      240001,
      260001,
      280001,
      300001
   ],
   "modified":"2014-12-27 05:09:55.579040",
   "private":false,
   "base_url":"https://skia.googlesource.com/buildbot@master",
   "project":"skiabuildbot",
   "target_ref":"refs/heads/master",
   "closed":false,
   "owner":"jcgregorio",
   "commit":false,
   "issue":821773002,
   "subject":"First rough pass at a Markdown server."
  }

*/

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"skia.googlesource.com/buildbot.git/go/util"
)

// Issue is a struct for decoding the JSON returned from Reitveld.
type Issue struct {
	OwnerEmail string  `json:"owner_email"`
	Patchsets  []int64 `json:"patchsets"`
	Closed     bool    `json:"closed"`
}

// Reitveld allows retrieving information from a Rietveld issue.
type Reitveld struct {
	client *http.Client
}

// NewClient creates a new client for interacting with Rietveld.
func NewClient() *Reitveld {
	return &Reitveld{
		client: util.NewTimeoutClient(),
	}
}

// Issue returns the owner email, the most recent patchset id, and if an issue
// is closed, for the given issue.
func (r *Reitveld) Issue(issue int64) (*Issue, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://codereview.chromium.org/api/%d/", issue), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to build request: %s", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve issue info: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("No such issue %d", issue)
	}
	dec := json.NewDecoder(resp.Body)
	info := &Issue{}
	if err := dec.Decode(info); err != nil {
		return nil, fmt.Errorf("Failed to read Reitveld issue info: %s", err)

	}
	return info, nil
}

// Patchset return an io.ReadCloser of the diff for the given issue and patchset id.
func (r Reitveld) Patchset(issue, patchset int64) (io.ReadCloser, error) {
	resp, err := r.client.Get(fmt.Sprintf("https://codereview.chromium.org/download/issue%d_%d.diff", issue, patchset))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve tarball: %s", err)
	}
	return resp.Body, nil
}
