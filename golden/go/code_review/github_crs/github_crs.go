package github_crs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/skerr"
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

func New(client *http.Client, repo string) *CRSImpl {
	return &CRSImpl{
		client: client,
		rl:     rate.NewLimiter(maxQPS, maxBurst),
		repo:   repo,
	}
}

type user struct {
	Name string `json:"login"`
}

type pullRequestResponse struct {
	Title   string `json:"title"`
	User    user   `json:"user"`
	State   string `json:"state"`
	Updated string `json:"updated_at"` // e.g.  "2011-01-26T19:01:12Z"
	Merged  string `json:"merged_at"`
}

// GetChangeList implements the code_review.Client interface.
func (c *CRSImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%s", c.repo, id)
	// TODO(kjlubick): use https://golang.org/pkg/net/http/#NewRequestWithContext
	fmt.Println(u)
	resp, err := c.client.Get(u)
	if err != nil {
		return code_review.ChangeList{}, skerr.Wrapf(err, "GitHub request pulls/%s", id)
	}

	var prr pullRequestResponse
	err = json.NewDecoder(resp.Body).Decode(&prr)
	if err != nil {
		return code_review.ChangeList{}, skerr.Wrapf(err, "received invalid JSON from GitHub")
	}

	state := code_review.Open
	if prr.State == "closed" {
		if prr.Merged != "" {
			state = code_review.Landed
		} else {
			state = code_review.Abandoned
		}
	}

	updated, err := time.Parse("2006-01-02T15:04:05Z", prr.Updated)
	if err != nil {
		return code_review.ChangeList{}, skerr.Wrapf(err, "invalid time %q", prr.Updated)
	}

	return code_review.ChangeList{
		SystemID: id,
		Owner:    prr.User.Name,
		Subject:  prr.Title,
		Status:   state,
		Updated:  updated,
	}, nil
}
