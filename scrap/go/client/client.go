// Package client is a client for the Scrap Exchange REST API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/scrap/go/scrap"
)

// Client implements api.ScrapExchange using the HTTP REST API talking to a server.
type Client struct {
	// baseURL holds the base scheme, host, and port that all requests should go to.
	baseURL *url.URL

	httpClient *http.Client
}

// New returns a new instance of Client.
//
// The value of the host should be the base scheme, host, and port that all
// requests should go to, e.g. "http://scrapexchange:9000".
func New(host string) (*Client, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Client{
		baseURL:    u,
		httpClient: httputils.DefaultClientConfig().With2xxOnly().WithoutRetries().Client(),
	}, nil
}

// checkResponseForError checks all the ways a request might have failed.
func checkResponseForError(resp *http.Response, err error) error {
	if err != nil {
		return skerr.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		return skerr.Fmt("Request failed: %s", resp.Status)
	}
	return nil
}

// decodeAndClose decodes the ReadCloser into the given 'v' and returns a
// wrapped error if any occurred.
func decodeAndClose(rc io.ReadCloser, v interface{}) error {
	defer util.Close(rc)
	return skerr.Wrap(json.NewDecoder(rc).Decode(v))
}

// pathToAbsoluteURL returns the full (non-relative) URL for the given path as a string.
func (c *Client) pathToAbsoluteURL(path string) string {
	// Make a copy of the base URL.
	u := &(*c.baseURL)
	u.Path = path
	return u.String()
}

// makeRequestCheckResponse makes the given request and then checks the response for any errors.
func (c *Client) makeRequestCheckResponse(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create request.")
	}
	resp, err := c.httpClient.Do(req)
	if err := checkResponseForError(resp, err); err != nil {
		return resp, err
	}
	return resp, nil
}

// Expand implements scrap.ScrapExchange.
func (c *Client) Expand(ctx context.Context, t scrap.Type, hashOrName string, lang scrap.Lang, w io.Writer) error {
	resp, err := c.makeRequestCheckResponse(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/tmpl/%s/%s/%s", t, hashOrName, lang)), nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Expand template.")
	}
	defer util.Close(resp.Body)
	if _, err := io.Copy(w, resp.Body); err != nil {
		return skerr.Wrapf(err, "Failed to read Expand response.")
	}
	return nil
}

// LoadScrap implements scrap.ScrapExchange.
func (c *Client) LoadScrap(ctx context.Context, t scrap.Type, hashOrName string) (scrap.ScrapBody, error) {
	var ret scrap.ScrapBody
	resp, err := c.makeRequestCheckResponse(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/scraps/%s/%s", t, hashOrName)), nil)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to load scrap.")
	}
	if err := decodeAndClose(resp.Body, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}

// CreateScrap implements scrap.ScrapExchange.
func (c *Client) CreateScrap(ctx context.Context, body scrap.ScrapBody) (scrap.ScrapID, error) {
	var ret scrap.ScrapID

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return ret, skerr.Wrap(err)
	}

	resp, err := c.makeRequestCheckResponse(ctx, "POST", c.pathToAbsoluteURL("/_/scraps/"), &b)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to create scrap.")
	}
	if err := decodeAndClose(resp.Body, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}

// DeleteScrap implements scrap.ScrapExchange.
func (c *Client) DeleteScrap(ctx context.Context, t scrap.Type, hashOrName string) error {
	_, err := c.makeRequestCheckResponse(ctx, "DELETE", c.pathToAbsoluteURL(fmt.Sprintf("/_/scraps/%s/%s", t, hashOrName)), nil)
	return err
}

// PutName implements scrap.ScrapExchange.
func (c *Client) PutName(ctx context.Context, t scrap.Type, name string, nameBody scrap.Name) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(nameBody); err != nil {
		return skerr.Wrap(err)
	}

	_, err := c.makeRequestCheckResponse(ctx, "PUT", c.pathToAbsoluteURL(fmt.Sprintf("/_/names/%s/%s", t, name)), &b)
	return err
}

// GetName implements scrap.ScrapExchange.
func (c *Client) GetName(ctx context.Context, t scrap.Type, name string) (scrap.Name, error) {
	var ret scrap.Name
	resp, err := c.makeRequestCheckResponse(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/names/%s/%s", t, name)), nil)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to get name.")
	}
	if err := decodeAndClose(resp.Body, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}

// DeleteName implements scrap.ScrapExchange.
func (c *Client) DeleteName(ctx context.Context, t scrap.Type, name string) error {
	_, err := c.makeRequestCheckResponse(ctx, "DELETE", c.pathToAbsoluteURL(fmt.Sprintf("/_/names/%s/%s", t, name)), nil)
	return err
}

// ListNames implements scrap.ScrapExchange.
func (c *Client) ListNames(ctx context.Context, t scrap.Type) ([]string, error) {
	var ret []string
	resp, err := c.makeRequestCheckResponse(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/names/%s/", t)), nil)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to create request.")
	}
	if err := decodeAndClose(resp.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}
