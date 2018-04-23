// Client for interacting with the Google Container Registry.
//
// The Docker v2 API is protected by oauth2, but it doesn't look
// exactly like Google OAuth2, so we have to create our own
// token source.
package gcr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

// gcrTokenSource it an oauth2.TokenSource that works with the Google Container
// Registry API.
type gcrTokenSource struct {
	// client is an authorized client that has access to the GCS bucket where gcr stores docker images.
	client *http.Client

	// scope - The set of images we are querying, project-id/image, i.e. "skia-public/docserver"
	scope string
}

func (g *gcrTokenSource) Token() (*oauth2.Token, error) {
	// Use the authorized client to get a gcr.io specific oauth token.
	resp, err := g.client.Get(fmt.Sprintf("https://gcr.io/v2/token?scope=repository:%s:pull", g.scope))
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Got unexpected status: %s", resp.Status)
	}
	var res struct {
		AccessToken  string `json:"token"`
		ExpiresInSec int    `json:"expires_in"`
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read gcr token response: %s", err)
	}
	sklog.Infof("Got %s", string(b))
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, fmt.Errorf("Invalid token JSON from metadata: %v", err)
	}
	sklog.Infof("Got %#v", res)
	if res.ExpiresInSec == 0 || res.AccessToken == "" {
		return nil, fmt.Errorf("Incomplete token received from metadata: %#v", res)
	}
	return &oauth2.Token{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Duration(res.ExpiresInSec) * time.Second),
	}, nil
}

// Client talks to the Google Cloud Registry that supports the v2 Docker API.
type Client struct {
	client *http.Client
	scope  string
}

// NewClient creates a Client that retrieves information about the docker images store under 'scope'.
//
// tokenSource - An oauth2.TokenSource that Has read access to the bucket that the docker images are stored in.
// scope - project-id/image, i.e. "skia-public/docserver"
func NewClient(tokenSource oauth2.TokenSource, scope string) *Client {
	gcrTokenSource := &gcrTokenSource{
		client: auth.ClientFromTokenSource(tokenSource),
		scope:  scope,
	}
	return &Client{
		client: auth.ClientFromTokenSource(gcrTokenSource),
		scope:  scope,
	}
}

// Tags returns all of the tags for all versions of the image.
func (c *Client) Tags() ([]string, error) {
	resp, err := c.client.Get(fmt.Sprintf("https://gcr.io/v2/%s/tags/list", c.scope))
	if err != nil {
		return nil, fmt.Errorf("Failed to request tags: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Got unexpected response: %s", resp.Status)
	}
	type Response struct {
		Tags []string `json:"tags"`
	}
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("Could not decode response: %s", err)
	}
	return response.Tags, nil
}
