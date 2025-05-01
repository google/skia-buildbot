package chromeperf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

// mockRoundTripper allows you to define a function that will be called
// when the client's Do method (which uses RoundTrip) is invoked.
type mockRoundTripper struct {
	roundTripFunc   func(req *http.Request) (*http.Response, error)
	ReceivedRequest *http.Request // To store the request for inspection
}

// RoundTrip implements the http.RoundTripper interface.
func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.ReceivedRequest = req // Store the request for later assertions
	return m.roundTripFunc(req)
}

func TestGenereateUrl_Bridge(t *testing.T) {
	api := "api_name"
	function := "func_name"
	direct := false
	urlOverride := ""

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, "https://skia-bridge-dot-chromeperf.appspot.com/api_name/func_name")
}

func TestGenereateUrl_Direct(t *testing.T) {
	api := "api_name"
	function := "func_name" // will be ignored
	direct := true
	urlOverride := ""

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, "https://chromeperf.appspot.com/api_name")
}

func TestGenereateUrl_Override(t *testing.T) {
	api := "api_name"       // will be ignored
	function := "func_name" // will be ignored
	direct := true          // will be ignored
	urlOverride := "override.url/someapi/andfunction"

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, urlOverride)
}

// NewChromePerfTestClient is a constructor for ChromePerfClient
func NewChromePerfTestClient(httpClient *http.Client, urlOverride string, directCallLegacy bool) ChromePerfClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second} // Default client if none provided
	}
	return &chromePerfClientImpl{
		httpClient:       httpClient,
		urlOverride:      urlOverride,
		directCallLegacy: directCallLegacy,
	}
}

type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

func TestSendGetRequest_Success(t *testing.T) {
	expectedURL := "https://chromeperf.appspot.com/api_name"
	expectedResponseBody := `{"sheriff_list":["abc", "def"], "error":""}`
	expected := &GetSheriffListResponse{
		SheriffList: []string{"abc", "def"},
	}
	mockTransport := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			// Assertions on the request created by NewRequestWithContext (inside httputils.GetWithContext)
			if req.Method != http.MethodGet {
				return nil, fmt.Errorf("expected method GET, got %s", req.Method)
			}
			if req.URL.String() != expectedURL+"?" {
				return nil, fmt.Errorf("expected URL %s, got %s", expectedURL, req.URL.String())
			}
			// Check if the context was passed correctly
			if req.Context() == nil {
				return nil, errors.New("request context is nil")
			}
			if _, ok := req.Context().Deadline(); !ok {
				t.Log("Test context does not have a deadline, which is fine for this specific check.")
			}

			// Simulate a successful response
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(expectedResponseBody)),
				Header:     make(http.Header),
			}, nil
		},
	}
	testHttpClient := &http.Client{
		Transport: mockTransport,
	}

	clientUnderTest := NewChromePerfTestClient(testHttpClient, expectedURL, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	getSheriffListResponse := &GetSheriffListResponse{}
	err := clientUnderTest.SendGetRequest(ctx, "sheriff_configs_skia", "", url.Values{}, getSheriffListResponse)

	assert.NoError(t, err, "FetchSomeResource returned an unexpected error")
	assertdeep.Equal(t, getSheriffListResponse, expected)
	assert.NotNil(t, mockTransport.ReceivedRequest, "mockTransport did not receive any request")
}
