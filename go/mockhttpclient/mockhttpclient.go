package mockhttpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// mockRoundTripper implements http.RoundTripper but returns mocked responses.
type mockRoundTripper struct {
	URLMap map[string][]byte
}

func (t *mockRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if data, ok := t.URLMap[r.URL.String()]; ok {
		return &http.Response{
			Body: &respBodyCloser{bytes.NewReader(data)},
		}, nil
	}
	return nil, fmt.Errorf("No such URL in urlMap!")
}

// respBodyCloser is a wrapper which lets us pretend to implement io.ReadCloser
// by wrapping a bytes.Reader.
type respBodyCloser struct {
	io.Reader
}

// Close is a stub method which lets us pretend to implement io.ReadCloser.
func (r respBodyCloser) Close() error {
	return nil
}

// New returns a new mocked HTTPClient.
func New(urlMap map[string][]byte) *http.Client {
	rt := &mockRoundTripper{
		URLMap: urlMap,
	}
	return &http.Client{
		Transport: rt,
	}
}
