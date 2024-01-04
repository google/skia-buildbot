package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// DepsURL is the format of the URL used to retrieve a parsed DEPS file
	// for a particular repository and revision.
	DepsURL = "%s/deps?repository_url=%s&git_hash=%s"

	// SkiaBridgeURL is the default production skia-bridge service.
	SkiaBridgeURL = "https://skia-bridge-dot-chromeperf.appspot.com"
)

// SkiaBridge is an interface to SkiaBridge service APIs.
type SkiaBridge interface {
	// GetDeps returns a map of git-based repository urls to git hashes parsed from a DEPS file.
	GetDeps(ctx context.Context, repositoryUrl, gitHash string) (map[string]string, error)
}

// SkiaBridgeClient is an object used to interact with a single SkiaBridge instance.
type SkiaBridgeClient struct {
	client *http.Client

	// URL, which is defaults to SkiaBridgeURL
	Url string
}

// NewSkiaBridgeClient creates and returns a new SkiaBridgeClient object.
func NewSkiaBridgeClient(c *http.Client) *SkiaBridgeClient {
	return &SkiaBridgeClient{
		client: c,
		Url:    SkiaBridgeURL,
	}
}

// WithURL overrides the default SkiaBridgeURL and returns the updated Client object.
func (sc SkiaBridgeClient) WithURL(url string) SkiaBridgeClient {
	sc.Url = url
	return sc
}

// get executes the GET request to the provided url.
func (s *SkiaBridgeClient) get(ctx context.Context, url string) (*http.Response, error) {
	resp, err := httputils.GetWithContext(ctx, s.client, url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		util.Close(resp.Body)
		return nil, skerr.Fmt("Request returned status %q", resp.Status)
	}
	return resp, nil
}

// getJSON executes a GET request to the given url, reads the response and
// unmarshals it to the provided destination.
func (s *SkiaBridgeClient) getJSON(ctx context.Context, url string, dest interface{}) error {
	resp, err := s.get(ctx, url)
	if err != nil {
		return skerr.Wrapf(err, "GET %s", url)
	}
	defer util.Close(resp.Body)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return skerr.Fmt("Failed to read response: %s", err)
	}

	return skerr.Wrap(json.Unmarshal(b, dest))
}

// GetDeps fetches git-based dependencies parsed from a DEPS file for a
// repository at the provided git hash.
func (s *SkiaBridgeClient) GetDeps(ctx context.Context, repoUrl, gitHash string) (map[string]string, error) {
	var resp map[string]string

	url := fmt.Sprintf(DepsURL, s.Url, repoUrl, gitHash)
	err := s.getJSON(ctx, url, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
