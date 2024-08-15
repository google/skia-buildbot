package httpclient

import (
	"context"
	"io"
	"net/http"

	"go.skia.org/infra/go/skerr"
)

// HTTPClient makes it easier to mock out goldclient's dependencies on
// http.Client by representing a smaller interface.
type HTTPClient interface {
	Get(ctx context.Context, url string) (*http.Response, error)
	Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
}

type wrapped struct {
	client *http.Client
}

func (w wrapped) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return w.client.Do(req)
}

func (w wrapped) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	req.Header.Set("Content-Type", contentType)
	return w.client.Do(req)
}

func WrapNative(hc *http.Client) *wrapped {
	return &wrapped{client: hc}
}

func Unwrap(h HTTPClient) *http.Client {
	unwrapped, ok := h.(*wrapped)
	if !ok {
		return nil
	}
	return unwrapped.client
}

var _ HTTPClient = (*wrapped)(nil)
