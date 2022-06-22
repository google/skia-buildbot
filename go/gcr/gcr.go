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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	Server = "gcr.io"
)

var (
	// DockerTagRegex is used to parse a Docker image tag as set by our
	// infrastructure, which uses the following format:
	//
	// ${datetime}-${user}-${git_hash:0:7}-${repo_state}
	//
	// Where datetime is a UTC timestamp following the format:
	//
	// +%Y-%m-%dT%H_%M_%SZ
	//
	// User is the username of the person who built the image, git_hash is the
	// abbreviated Git commit hash at which the image was built, and repo_state
	// is either "clean" or "dirty", depending on whether there were local
	// changes to the checkout at the time when the image was built.
	DockerTagRegex = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}_\d{2}_\d{2}Z)-(\w+)-([a-f0-9]+)-(\w+)`)
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
	resp, err := g.client.Get(fmt.Sprintf("https://%s/v2/token?scope=repository:%s/%s:pull", Server, g.projectId, g.imageName))
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return nil, skerr.Fmt("Got unexpected status: %s", resp.Status)
	}
	var res struct {
		AccessToken  string `json:"token"`
		ExpiresInSec int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, skerr.Wrapf(err, "Invalid token JSON from metadata: %v", err)
	}
	if res.ExpiresInSec == 0 || res.AccessToken == "" {
		return nil, skerr.Fmt("Incomplete token received from metadata: %#v", res)
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

// TagsResponse is the response returned by Tags().
type TagsResponse struct {
	Manifest map[string]struct {
		ImageSizeBytes string   `json:"imageSizeBytes"`
		LayerID        string   `json:"layerId"`
		Tags           []string `json:"tag"`
		TimeCreatedMs  string   `json:"timeCreatedMs"`
		TimeUploadedMs string   `json:"timeUploadedMs"`
	} `json:"manifest"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// Tags returns all of the tags for all versions of the image.
func (c *Client) Tags(ctx context.Context) (*TagsResponse, error) {
	var rv *TagsResponse
	const batchSize = 100
	url := fmt.Sprintf("https://%s/v2/%s/%s/tags/list?n=%d", Server, c.projectId, c.imageName, batchSize)
	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to create HTTP request")
		}
		req = req.WithContext(ctx)
		req.Header.Add("Accept", "*")
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to request tags")
		}
		defer util.Close(resp.Body)
		if resp.StatusCode != 200 {
			return nil, skerr.Fmt("Got unexpected response: %s", resp.Status)
		}
		response := new(TagsResponse)
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return nil, skerr.Wrapf(err, "could not decode response")
		}
		if rv == nil {
			rv = response
		} else {
			rv.Tags = append(rv.Tags, response.Tags...)
			for k, v := range response.Manifest {
				rv.Manifest[k] = v
			}
		}

		nextUrl, ok := resp.Header["Link"]
		if !ok {
			break
		}
		url = strings.Split(nextUrl[0], ";")[0]
	}
	return rv, nil
}
