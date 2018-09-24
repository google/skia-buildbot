// Client for interacting with the Google Container Registry.
//
// The Docker v2 API is protected by OAuth2, but it doesn't look
// exactly like Google OAuth2, so we have to create our own token
// source.
//
// Go implementation of the bash commands from:
//
//   https://stackoverflow.com/questions/34037256/does-google-container-registry-support-docker-remote-api-v2/34046435#34046435
//
// I.e.:
//   $ export NAME=project-id/image
//   $ export BEARER=$(curl -u _token:$(gcloud auth print-access-token) https://gcr.io/v2/token?scope=repository:$NAME:pull | cut -d'"' -f 10)
//   $ curl -H "Authorization: Bearer $BEARER" https://gcr.io/v2/$NAME/tags/list
package gcr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	SERVER = "gcr.io"
)

// gcrTokenSource it an oauth2.TokenSource that works with the Google Container Registry API.
type gcrTokenSource struct {
	// client is an authorized client that has access to the GCS bucket where gcr stores docker images.
	client *http.Client

	// projectId - The Google Cloud project name, e.g. 'skia-public'.
	projectId string

	// imageName - The name of the image.
	imageName string
}

func (g *gcrTokenSource) Token() (*oauth2.Token, error) {
	// Use the authorized client to get a gcr.io specific oauth token.
	resp, err := g.client.Get(fmt.Sprintf("https://%s/v2/token?scope=repository:%s/%s:pull", SERVER, g.projectId, g.imageName))
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
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("Invalid token JSON from metadata: %v", err)
	}
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

	// projectId - The Google Cloud project name, e.g. 'skia-public'.
	projectId string

	// imageName - The name of the image.
	imageName string
}

// NewClient creates a Client that retrieves information about the docker images store under 'projectID'/'image'.
//
// tokenSource - An oauth2.TokenSource that Has read access to the bucket that the docker images are stored in.
// projectId - The Google Cloud project name, e.g. 'skia-public'.
// imageName - The name of the image, e.g. docserver.
func NewClient(tokenSource oauth2.TokenSource, projectId, imageName string) *Client {
	gcrTokenSource := &gcrTokenSource{
		client:    httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client(),
		projectId: projectId,
		imageName: imageName,
	}
	return &Client{
		client:    httputils.DefaultClientConfig().WithTokenSource(gcrTokenSource).With2xxOnly().Client(),
		projectId: projectId,
		imageName: imageName,
	}
}

// Tags returns all of the tags for all versions of the image.
func (c *Client) Tags() ([]string, error) {
	// TODO(jcgregorio) Look for link rel=next header to do pagination. https://docs.docker.com/registry/spec/api/#listing-image-tags
	resp, err := c.client.Get(fmt.Sprintf("https://%s/v2/%s/%s/tags/list", SERVER, c.projectId, c.imageName))
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
