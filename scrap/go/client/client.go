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

// pathToAbsoluteURL returns the full (non-relative) URL for the given path as a string.
func (c *Client) pathToAbsoluteURL(path string) string {
	// Make a copy of the base URL.
	u := &(*c.baseURL)
	u.Path = path
	return u.String()
}

// Expand implements scrap.ScrapExchange.
func (c *Client) Expand(ctx context.Context, t scrap.Type, hashOrName string, lang scrap.Lang, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/tmpl/%s/%s/%s", t, hashOrName, lang)), nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create request.")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return skerr.Wrapf(err, "Expand request failed.")
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return skerr.Wrapf(err, "Failed to read Expand response.")
	}
	return nil
}

// LoadScrap implements scrap.ScrapExchange.
func (c *Client) LoadScrap(ctx context.Context, t scrap.Type, hashOrName string) (scrap.ScrapBody, error) {
	var ret scrap.ScrapBody
	req, err := http.NewRequestWithContext(ctx, "GET", c.pathToAbsoluteURL(fmt.Sprintf("/_/scraps/%s/%s", t, hashOrName)), nil)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to create request.")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ret, skerr.Wrapf(err, "Load request failed.")
	}
	defer util.Close(resp.Body)
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return ret, skerr.Wrap(err)
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.pathToAbsoluteURL("/_/scraps/"), &b)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to create request.")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ret, skerr.Wrapf(err, "Create request failed.")
	}
	defer util.Close(resp.Body)
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return ret, skerr.Wrap(err)
	}
	return ret, nil
}

// DeleteScrap implements scrap.ScrapExchange.
func (c *Client) DeleteScrap(ctx context.Context, t scrap.Type, hashOrName string) error {
	panic("not implemented") // TODO: Implement
}

// PutName implements scrap.ScrapExchange.
func (c *Client) PutName(ctx context.Context, t scrap.Type, name string, nameBody scrap.Name) error {
	panic("not implemented") // TODO: Implement
}

// GetName implements scrap.ScrapExchange.
func (c *Client) GetName(ctx context.Context, t scrap.Type, name string) (scrap.Name, error) {
	panic("not implemented") // TODO: Implement
}

// DeleteName implements scrap.ScrapExchange.
func (c *Client) DeleteName(ctx context.Context, t scrap.Type, name string) error {
	panic("not implemented") // TODO: Implement
}

// ListNames implements scrap.ScrapExchange.
func (c *Client) ListNames(ctx context.Context, t scrap.Type) ([]string, error) {
	panic("not implemented") // TODO: Implement
}
