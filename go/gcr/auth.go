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

type gcrTokenSource struct {
	client *http.Client
	scope  string
}

func (g *gcrTokenSource) Token() (*oauth2.Token, error) {
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

type Client struct {
	client *http.Client
	scope  string
}

// tokenSource - Has read access to the bucket that the docker images are stored in.
// scope - project-id/image, i.e. skia-public/docserver2
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

func (c *Client) Tags() ([]string, error) {
	// $ curl -H "Authorization: Bearer $BEARER" https://gcr.io/v2/$name/tags/list
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
