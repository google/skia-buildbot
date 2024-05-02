package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
)

const (
	crrevRedirectUrl = "https://cr-rev.appspot.com/_ah/api/crrev/v1/redirect/"
	ChromiumRepo     = "chromium/src"
)

// CrrevClient creates an API for alert grouping and Pinpoint UI to convert
// commit positions to git hashes before submitting them as Pinpoint jobs.
type CrrevClient interface {
	// GetCommitInfo returns the git hash, project, and repo of a commit
	// Supports commit hashes and positions and can return non-chromium commits.
	GetCommitInfo(ctx context.Context, commit string) (*CrrevResponse, error)
}

// CrrevClientImpl implements CrrevClient
type CrrevClientImpl struct {
	Client *http.Client
}

// CrrevResponse is the response provided by the crrev redirect endpoint
type CrrevResponse struct {
	GitHash string `json:"git_sha"`
	Project string `json:"project"`
	Repo    string `json:"repo"`
}

func NewCrrevClient(ctx context.Context) (*CrrevClientImpl, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create crrev client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &CrrevClientImpl{
		Client: client,
	}, nil
}

func (c *CrrevClientImpl) GetCommitInfo(ctx context.Context, commit string) (*CrrevResponse, error) {
	resp, err := httputils.GetWithContext(ctx, c.Client, fmt.Sprintf("%s/%s", crrevRedirectUrl, commit))
	if err != nil {
		return nil, skerr.Wrapf(err, "could not make request to crrev with commit %v", commit)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, skerr.Fmt("Response failed with status code: %d", resp.StatusCode)
	}
	var crrevResp CrrevResponse
	if err := json.Unmarshal(body, &crrevResp); err != nil {
		return nil, skerr.Wrapf(err, "could not unmarshal crrev content")
	}
	return &crrevResp, nil
}
