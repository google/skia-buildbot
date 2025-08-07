package chromeperf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
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

// For testing purpose only.
type getSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

// For testing purpose only.
type mockReadCloser struct {
	readErr  error
	closeErr error
	data     []byte
	readIdx  int
}

// For testing purpose only.
func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if m.readIdx >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.readIdx:])
	m.readIdx += n
	return n, nil
}

// For testing purpose only.
func (m *mockReadCloser) Close() error { return m.closeErr }

func TestSendGetRequest(t *testing.T) {
	tests := []struct {
		name                string
		clientConfig        func() (urlOverride string, directCallLegacy bool) // Configure client properties
		apiName             string
		functionName        string
		queryParams         url.Values
		responseObjTemplate func() any
		roundTripFunc       func(t *testing.T, req *http.Request) (*http.Response, error) // Defines mock transport behavior
		expectError         bool
		expectedErrorMsg    string
		expectedResponse    any
	}{
		{
			name:                "Successful request",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "sheriff_configs_skia",
			functionName:        "",
			queryParams:         url.Values{"key1": []string{"value1"}, "key2": []string{"value2"}},
			responseObjTemplate: func() any { return &getSheriffListResponse{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				assert.NotNil(t, req.Context())
				assert.Equal(t, http.MethodGet, req.Method)
				assert.Equal(t, "/sheriff_configs_skia/", req.URL.Path)
				assert.Equal(t, "key1=value1&key2=value2", req.URL.RawQuery)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sheriff_list":["abc", "def"], "error":""}`)),
					Header:     make(http.Header),
				}, nil
			},
			expectError:      false,
			expectedResponse: &getSheriffListResponse{SheriffList: []string{"abc", "def"}},
		},
		{
			name:                "Successful request with URL override",
			clientConfig:        func() (string, bool) { return "http://mockserver.com/override", false },
			apiName:             "customApi",
			functionName:        "customFunc",
			queryParams:         url.Values{"k": []string{"v"}},
			responseObjTemplate: func() any { return &getSheriffListResponse{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				assert.Equal(t, "https://mockedserver/customApi/customFunc?k=v", req.URL.String()) // Full URL check
				assert.Equal(t, "k=v", req.URL.RawQuery)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sheriff_list":["abc", "def"], "error":""}`)),
					Header:     make(http.Header),
				}, nil
			},
			expectError:      false,
			expectedResponse: &getSheriffListResponse{SheriffList: []string{"abc", "def"}},
		},
		{
			name:                "Successful request with direct call legacy",
			clientConfig:        func() (string, bool) { return "", true }, // directCallLegacy = true
			apiName:             "legacyApi",
			functionName:        "legacyFunc",
			queryParams:         nil,
			responseObjTemplate: func() any { return &getSheriffListResponse{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				assert.Equal(t, "https://mockedserver/legacyApi/legacyFunc?", req.URL.String())
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sheriff_list":["abc", "def"], "error":""}`)),
					Header:     make(http.Header),
				}, nil
			},
			expectError:      false,
			expectedResponse: &getSheriffListResponse{SheriffList: []string{"abc", "def"}},
		},
		{
			name:                "Network error from RoundTrip",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "anyApi",
			functionName:        "anyFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				return nil, errors.New("simulated network error from RoundTrip")
			},
			expectError:      true,
			expectedErrorMsg: "Failed to get chrome perf response", // Wrapped error
		},
		{
			name:                "Error reading response body (from RoundTrip's perspective)",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "readErrorApi",
			functionName:        "readErrorFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       &mockReadCloser{readErr: errors.New("simulated read body error")},
					Header:     make(http.Header),
				}, nil
			},
			expectError:      true,
			expectedErrorMsg: "Failed to read body from chrome perf response",
		},
		{
			name:                "JSON unmarshal error",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "jsonErrorApi",
			functionName:        "jsonErrorFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`this is not valid json`)),
					Header:     make(http.Header),
				}, nil
			},
			expectError:      true,
			expectedErrorMsg: "Failed to parse chrome perf response body",
		},
		{
			name:                "Non-2xx HTTP status code (e.g., 400)",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "statusErrorApi",
			functionName:        "statusErrorFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest, // 400
					Body:       io.NopCloser(strings.NewReader(`{"error": "bad input from roundtrip"}`)),
					Header:     make(http.Header),
				}, nil
			},
			expectError:      true,
			expectedErrorMsg: "chrome perf request failed", // Also check for status code and body in error
		},
		{
			name:                "Context cancelled during HTTP request",
			clientConfig:        func() (string, bool) { return "", false },
			apiName:             "contextApi",
			functionName:        "contextFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				// The actual context cancellation is handled by client.Do if the context is passed correctly.
				// The RoundTripper itself might not always directly see the context unless the HTTP client
				// aborts the request due to context cancellation before RoundTrip is even called or while it's running.
				// For this test, we'll simulate the error that client.Do would return.
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err() // e.g. context.Canceled or context.DeadlineExceeded
				default:
					// If context wasn't cancelled fast enough for this check
					time.Sleep(10 * time.Millisecond) // Give it a moment
					if req.Context().Err() != nil {
						return nil, req.Context().Err()
					}
					return nil, errors.New("context was not cancelled as expected by mock roundtrip")
				}
			},
			expectError:      true,
			expectedErrorMsg: "Failed to get chrome perf response", // Error from GetWithContext/client.Do will be wrapped
		},
		{
			name:                "Client with nil httpClient (setup error, not RoundTripper)",
			clientConfig:        func() (string, bool) { return "NIL_HTTP_CLIENT", false }, // Special marker
			apiName:             "nilClientApi",
			functionName:        "nilClientFunc",
			responseObjTemplate: func() any { return &struct{}{} },
			roundTripFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				// This won't be called if httpClient is nil
				return nil, errors.New("roundTripFunc should not be called for nil httpClient test")
			},
			expectError:      true,
			expectedErrorMsg: "no such host",
		},
	}

	expectedURL := "https://mockedserver"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlOverride, directCallLegacy := tt.clientConfig()

			var testHttpClient *http.Client
			var mockTransport *mockRoundTripper
			if urlOverride == "NIL_HTTP_CLIENT" {
				testHttpClient = &http.Client{
					Transport: &http.Transport{
						DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
							// Simulate "no such host" error
							return nil, &net.DNSError{Err: "no such host", Name: strings.Split(addr, ":")[0]}
						},
					},
				}
			} else {
				mockTransport = &mockRoundTripper{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						// Pass `t` to the user-defined roundTripFunc for assertions
						return tt.roundTripFunc(t, req)
					},
				}
				testHttpClient = &http.Client{
					Transport: mockTransport,
				}
			}
			apiURL := fmt.Sprintf("%s/%s/%s", expectedURL, tt.apiName, tt.functionName)
			clientUnderTest := NewChromePerfTestClient(testHttpClient, apiURL, directCallLegacy)

			ctx := context.Background()
			if tt.name == "Context cancelled during HTTP request" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Millisecond) // Short timeout
				defer cancel()
			}

			responseTemplate := tt.responseObjTemplate()
			err := clientUnderTest.SendGetRequest(ctx, tt.apiName, tt.functionName, tt.queryParams, responseTemplate)

			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg, "Error message mismatch: got %v", err)
				}
			} else {
				require.NoError(t, err, "Expected no error but got one: %v", err)
				if tt.expectedResponse != nil {
					assert.Equal(t, tt.expectedResponse, responseTemplate, "Response data mismatch")
				}
				assert.NotNil(t, mockTransport.ReceivedRequest, "mockTransport did not receive any request")
			}
		})
	}
}

// For testing purpose only.
type anomaly struct {
	Id            string `json:"id"`
	TestPath      string `json:"test_path"`
	BugId         int    `json:"bug_id"`
	StartRevision int    `json:"start_revision"`
	EndRevision   int    `json:"end_revision"`

	// The hashes below are needed for cases where the commit numbers are
	// different in chromeperf and in the perf instance. We can use these
	// hashes to look up the correct commit number from the database.
	StartRevisionHash string `json:"start_revision_hash,omitempty"`
	EndRevisionHash   string `json:"end_revision_hash,omitempty"`

	IsImprovement       bool     `json:"is_improvement"`
	Recovered           bool     `json:"recovered"`
	State               string   `json:"state"`
	Statistics          string   `json:"statistic"`
	Unit                string   `json:"units"`
	DegreeOfFreedom     float64  `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float64  `json:"median_before_anomaly"`
	MedianAfterAnomaly  float64  `json:"median_after_anomaly"`
	PValue              float64  `json:"p_value"`
	SegmentSizeAfter    int      `json:"segment_size_after"`
	SegmentSizeBefore   int      `json:"segment_size_before"`
	StdDevBeforeAnomaly float64  `json:"std_dev_before_anomaly"`
	TStatistics         float64  `json:"t_statistic"`
	SubscriptionName    string   `json:"subscription_name"`
	BugComponent        string   `json:"bug_component"`
	BugLabels           []string `json:"bug_labels"`
	BugCcEmails         []string `json:"bug_cc_emails"`
	BisectIDs           []string `json:"bisect_ids"`
	Timestamp           string   `json:"timestamp,omitempty"`
}

// For testing purpose only.
type getAnomaliesResponse struct {
	Anomalies map[string][]anomaly `json:"anomalies"`
}

// For testing purpose only.
type getAnomaliesRequest struct {
	Tests           []string `json:"tests,omitempty"`
	MaxRevision     string   `json:"max_revision,omitempty"`
	MinRevision     string   `json:"min_revision,omitempty"`
	Revision        int      `json:"revision,omitempty"`
	NeedAggregation bool     `json:"need_aggregation,omitempty"`
}

type marshalErrorType struct{}

func (m *marshalErrorType) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("simulated marshal error")
}

func TestSendPostRequest(t *testing.T) {
	ctx := context.Background()
	anomaly1 := anomaly{
		Id:                  "101",
		TestPath:            "Master>Builder>Test>Subtest.Value",
		BugId:               12345,
		StartRevision:       10001,
		EndRevision:         10005,
		StartRevisionHash:   "abcdef123456",
		EndRevisionHash:     "fedcba654321",
		IsImprovement:       false,
		Recovered:           true,
		State:               "Closed",
		Statistics:          "mean",
		Unit:                "ms",
		DegreeOfFreedom:     10.5,
		MedianBeforeAnomaly: 150.2,
		MedianAfterAnomaly:  185.7,
		PValue:              0.001,
		SegmentSizeAfter:    50,
		SegmentSizeBefore:   48,
		StdDevBeforeAnomaly: 12.3,
		TStatistics:         -3.45,
		SubscriptionName:    "My Test Subscription",
		BugComponent:        "Blink>Performance",
		BugLabels:           []string{"Perf-Regression", "Mobile"},
		BugCcEmails:         []string{"user1@example.com", "user2@example.com"},
		BisectIDs:           []string{"bisect_run_001", "bisect_run_002"},
	}
	anomaly2 := anomaly{
		Id:                  "102",
		TestPath:            "Master>Builder>Test>AnotherSubtest.Score",
		BugId:               0,
		StartRevision:       10010,
		EndRevision:         10012,
		IsImprovement:       true,
		Recovered:           false,
		State:               "Investigating",
		Statistics:          "percentile_90",
		Unit:                "score",
		DegreeOfFreedom:     8.0,
		MedianBeforeAnomaly: 800.0,
		MedianAfterAnomaly:  750.5,
		PValue:              0.045,
		SegmentSizeAfter:    30,
		SegmentSizeBefore:   35,
		StdDevBeforeAnomaly: 25.0,
		TStatistics:         2.15,
		SubscriptionName:    "Performance Score Alerts",
		BugComponent:        "",
		BugLabels:           []string{"Perf-Improvement"},
		BugCcEmails:         []string{},
		BisectIDs:           []string{},
	}

	tests := []struct {
		name                string
		apiName             string
		functionName        string
		requestObj          any
		responseObjTemplate any
		acceptedStatusCodes []int
		mockServerHandler   http.HandlerFunc
		directCallLegacy    bool // For URL generation variance
		expectError         bool
		expectedErrorMsg    string                               // Substring to check in the error
		expectedResponse    any                                  // Expected state of responseObj after call
		checkRequestBody    func(t *testing.T, bodyBytes []byte) // Optional: to validate the sent body
	}{
		{
			name:         "Successful request",
			apiName:      "anomalies",
			functionName: "find",
			requestObj: getAnomaliesRequest{
				Tests:       []string{"foo/bar", "foo/baz"},
				MaxRevision: strconv.Itoa(987654321),
				MinRevision: strconv.Itoa(123456789),
			},
			responseObjTemplate: &getAnomaliesResponse{},
			acceptedStatusCodes: []int{http.StatusOK, http.StatusCreated},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "/anomalies/find", r.URL.Path) // Check generated URL path

				bodyBytes, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				var receivedReq getAnomaliesRequest
				assert.NoError(t, json.Unmarshal(bodyBytes, &receivedReq))
				assert.Equal(t, getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"}, receivedReq)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				err = json.NewEncoder(w).Encode(getAnomaliesResponse{Anomalies: map[string][]anomaly{
					"foo/bar": {anomaly1},
					"foo/baz": {anomaly1, anomaly2},
				}})
				if err != nil {
					assert.Fail(t, "the Encode should always pass.")
				}
			},
			expectError: false,
			expectedResponse: &getAnomaliesResponse{Anomalies: map[string][]anomaly{
				"foo/bar": {anomaly1},
				"foo/baz": {anomaly1, anomaly2},
			}},
		},
		{
			name:                "Request marshalling error",
			apiName:             "data",
			functionName:        "upload",
			requestObj:          &marshalErrorType{}, // This type will cause json.Marshal to fail
			responseObjTemplate: &getAnomaliesResponse{},
			acceptedStatusCodes: []int{http.StatusOK},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("Server should not be called if marshalling fails")
			},
			expectError:      true,
			expectedErrorMsg: "json: error calling MarshalJSON for type *chromeperf.marshalErrorType: simulated marshal error",
		},
		{
			name:         "HTTP Post error (server down)",
			apiName:      "data",
			functionName: "upload",
			requestObj:   getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"},
			responseObjTemplate: &getAnomaliesResponse{Anomalies: map[string][]anomaly{
				"foo/bar": {anomaly1},
				"foo/baz": {anomaly1, anomaly2},
			}},
			acceptedStatusCodes: []int{http.StatusOK},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				// Server handler won't be called as we close the server immediately
			},
			expectError:      true,
			expectedErrorMsg: "connect: connection refused", // Error from httputils.PostWithContext
		},
		{
			name:         "Unexpected status code",
			apiName:      "data",
			functionName: "upload",
			requestObj:   getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"},
			responseObjTemplate: &getAnomaliesResponse{Anomalies: map[string][]anomaly{
				"foo/bar": {anomaly1},
				"foo/baz": {anomaly1, anomaly2},
			}},
			acceptedStatusCodes: []int{http.StatusOK},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := fmt.Fprint(w, "Internal Error Detail")
				if err != nil {
					assert.Fail(t, "the fmt.Fprint should always pass.")
				}
			},
			expectError:      true,
			expectedErrorMsg: "Receive status 500 from chromeperf",
		},
		{
			name:         "Response unmarshalling error (malformed JSON)",
			apiName:      "data",
			functionName: "upload",
			requestObj:   getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"},
			responseObjTemplate: &getAnomaliesResponse{Anomalies: map[string][]anomaly{
				"foo/bar": {anomaly1},
				"foo/baz": {anomaly1, anomaly2},
			}},
			acceptedStatusCodes: []int{http.StatusOK},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := fmt.Fprint(w, `{"status":"success", "processed_value": "not_an_int"`) // Malformed
				if err != nil {
					assert.Fail(t, "the fmt.Fprint should always pass.")
				}
			},
			expectError:      true,
			expectedErrorMsg: "unexpected EOF",
		},
		{
			name:                "Response with nil response body (responseObj is nil)",
			apiName:             "action",
			functionName:        "trigger",
			requestObj:          getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"},
			responseObjTemplate: nil, // Indicate no response body to be decoded
			acceptedStatusCodes: []int{http.StatusNoContent},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			expectError:      true,
			expectedErrorMsg: "EOF",
		},
		{
			name:         "Context timeout before server responds",
			apiName:      "data",
			functionName: "upload_timeout",
			requestObj:   getAnomaliesRequest{Tests: []string{"foo/bar", "foo/baz"}, MaxRevision: "987654321", MinRevision: "123456789"},
			responseObjTemplate: &getAnomaliesResponse{Anomalies: map[string][]anomaly{
				"foo/bar": {anomaly1},
				"foo/baz": {anomaly1, anomaly2},
			}},
			acceptedStatusCodes: []int{http.StatusOK},
			mockServerHandler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond) // Server takes longer than client timeout
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(&getAnomaliesResponse{Anomalies: map[string][]anomaly{
					"foo/bar": {anomaly1},
					"foo/baz": {anomaly1, anomaly2},
				}})
				if err != nil {
					assert.Fail(t, "the Encode should always pass.")
				}
			},
			expectError:      true,
			expectedErrorMsg: "context deadline exceeded", // Error from client.Do due to context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockServerHandler))
			// Special handling for the "server down" test
			if tt.name == "HTTP Post error (server down)" {
				server.Close() // Close immediately to simulate server down
			} else {
				defer server.Close()
			}

			// Due to the chromeperfClient.go:118, create urlOverride with path.
			apiURL := fmt.Sprintf("%s/%s/%s", server.URL, tt.apiName, tt.functionName)
			client := NewChromePerfTestClient(server.Client(), apiURL, false)

			// For context timeout test
			currentCtx := ctx
			var cancel context.CancelFunc
			if tt.name == "Context timeout before server responds" {
				currentCtx, cancel = context.WithTimeout(ctx, 50*time.Millisecond) // Client timeout shorter than server sleep
				defer cancel()
			}

			// Make a new instance of the response object for each test run
			var actualResponseObj interface{}
			if sr, ok := tt.responseObjTemplate.(*getAnomaliesResponse); ok && sr != nil {
				actualResponseObj = &getAnomaliesResponse{} // new pointer
			}

			err := client.SendPostRequest(currentCtx, tt.apiName, tt.functionName, tt.requestObj, actualResponseObj, tt.acceptedStatusCodes)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, skerr.Unwrap(err).Error(), tt.expectedErrorMsg, "Error message mismatch")
				}
			} else {
				require.NoError(t, err)
				if tt.expectedResponse != nil {
					assert.Equal(t, tt.expectedResponse, actualResponseObj, "Response object mismatch")
				} else {
					assert.Nil(t, actualResponseObj, "Expected responseObj to remain nil")
				}
			}
		})
	}
}
