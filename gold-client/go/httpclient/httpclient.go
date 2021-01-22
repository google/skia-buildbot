package httpclient

import (
	"io"
	"net/http"
)

// HTTPClient makes it easier to mock out goldclient's dependencies on
// http.Client by representing a smaller interface.
// TODO(kjlubick) is there a way to make these take a context.Context?
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
	Post(url, contentType string, body io.Reader) (resp *http.Response, err error)
}
