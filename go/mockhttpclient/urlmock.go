package mockhttpclient

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"sync"

	"github.com/texttheater/golang-levenshtein/levenshtein"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	TEST_FAILED_STATUS_CODE = 599
)

// URLMock implements http.RoundTripper but returns mocked responses. It
// provides two methods for mocking responses to requests for particular URLs:
//
// - Mock: Adds a fake response for the given URL to be used every time a
//         request is made for that URL.
//
// - MockOnce: Adds a fake response for the given URL to be used one time.
//         MockOnce may be called multiple times for the same URL in order to
//         simulate the response changing over time. Takes precedence over mocks
//         specified using Mock.
//
// Examples:
//
//    // Mock out a URL to always respond with the same body.
//    m := NewURLMock()
//    m.Mock("https://www.google.com", MockGetDialogue([]byte("Here's a response.")))
//    res, _ := m.Client().Get("https://www.google.com")
//    respBody, _ := ioutil.ReadAll(res.Body)  // respBody == []byte("Here's a response.")
//
//    // Mock out a URL to give different responses.
//    m.MockOnce("https://www.google.com", MockGetDialogue([]byte("hi")))
//    m.MockOnce("https://www.google.com", MockGetDialogue([]byte("Second response.")))
//    res1, _ := m.Client().Get("https://www.google.com")
//    body1, _ := ioutil.ReadAll(res1.Body)  // body1 == []byte("hi")
//    res2, _ := m.Client().Get("https://www.google.com")
//    body2, _ := ioutil.ReadAll(res2.Body)  // body2 == []byte("Second response.")
//    // Fall back on the value previously set using Mock():
//    res3, _ := m.Client().Get("https://www.google.com")
//    body3, _ := ioutil.ReadAll(res3.Body)  // body3 == []byte("Here's a response.")
type URLMock struct {
	mtx        sync.Mutex
	mockAlways map[string]MockDialogue
	mockOnce   map[string][]MockDialogue
}

var DONT_CARE_REQUEST = []byte{0, 1, 2, 3, 4}

type MockDialogue struct {
	requestMethod  string
	requestType    string
	requestPayload []byte

	responseStatus  string
	responseCode    int
	responsePayload []byte
	responseHeaders map[string][]string
}

// ResponseHeader adds the given header to the response.
func (md *MockDialogue) ResponseHeader(key, value string) {
	if md.responseHeaders == nil {
		md.responseHeaders = map[string][]string{}
	}
	md.responseHeaders[key] = append(md.responseHeaders[key], value)
}

func (md *MockDialogue) GetResponse(r *http.Request) (*http.Response, error) {
	if md.requestMethod != r.Method {
		return nil, fmt.Errorf("Wrong Method, expected %q, but was %q", md.requestMethod, r.Method)
	}
	if md.requestPayload == nil {
		if r.Body != nil {
			requestBody, _ := ioutil.ReadAll(r.Body)
			return nil, fmt.Errorf("No request payload expected, but was %s (%#v) ", string(requestBody), r.Body)
		}
	} else {
		if ct := r.Header.Get("Content-Type"); md.requestType != ct {
			return nil, fmt.Errorf("Content-Type was wrong, expected %q, but was %q", md.requestType, ct)
		}
		defer util.Close(r.Body)
		requestBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("Error reading request body: %s", err)
		}
		if !reflect.DeepEqual(md.requestPayload, DONT_CARE_REQUEST) && !reflect.DeepEqual(md.requestPayload, requestBody) {
			return nil, fmt.Errorf("Wrong request payload, expected \n%s, but was \n%s", md.requestPayload, requestBody)
		}
	}
	return &http.Response{
		Body:       &respBodyCloser{bytes.NewReader(md.responsePayload)},
		Header:     md.responseHeaders,
		Status:     md.responseStatus,
		StatusCode: md.responseCode,
	}, nil
}

// ServeHTTP implements http.Handler.
func (md MockDialogue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := md.GetResponse(r)
	if err != nil {
		http.Error(w, err.Error(), TEST_FAILED_STATUS_CODE)
		return
	}
	defer util.Close(resp.Body)
	// TODO(benjaminwagner): I don't see an easy way to include resp.Status.
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, err.Error(), TEST_FAILED_STATUS_CODE)
		return
	}
}

func MockGetDialogue(responseBody []byte) MockDialogue {
	return MockDialogue{
		requestMethod:  "GET",
		requestType:    "",
		requestPayload: nil,

		responseStatus:  "OK",
		responseCode:    http.StatusOK,
		responsePayload: responseBody,
	}
}

func MockGetError(responseStatus string, responseCode int) MockDialogue {
	return MockDialogue{
		requestMethod:  "GET",
		requestType:    "",
		requestPayload: nil,

		responseStatus:  responseStatus,
		responseCode:    responseCode,
		responsePayload: []byte{},
	}
}

func MockGetWithRequestDialogue(requestType string, requestBody, responseBody []byte) MockDialogue {
	return MockDialogue{
		requestMethod:  "GET",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    http.StatusOK,
		responsePayload: responseBody,
	}
}

func MockPostDialogue(requestType string, requestBody, responseBody []byte) MockDialogue {
	return MockDialogue{
		requestMethod:  "POST",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    http.StatusOK,
		responsePayload: responseBody,
	}
}

func MockPostDialogueWithResponseCode(requestType string, requestBody, responseBody []byte, responseCode int) MockDialogue {
	return MockDialogue{
		requestMethod:  "POST",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    responseCode,
		responsePayload: responseBody,
	}
}

func MockPostError(requestType string, requestBody []byte, responseStatus string, responseCode int) MockDialogue {
	return MockDialogue{
		requestMethod:  "POST",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  responseStatus,
		responseCode:    responseCode,
		responsePayload: []byte{},
	}
}

func MockPutDialogue(requestType string, requestBody, responseBody []byte) MockDialogue {
	return MockDialogue{
		requestMethod:  "PUT",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    http.StatusOK,
		responsePayload: responseBody,
	}
}

func MockPatchDialogue(requestType string, requestBody, responseBody []byte) MockDialogue {
	return MockDialogue{
		requestMethod:  "PATCH",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    http.StatusOK,
		responsePayload: responseBody,
	}
}

func MockDeleteDialogueWithResponseCode(requestType string, requestBody, responseBody []byte, responseCode int) MockDialogue {
	return MockDialogue{
		requestMethod:  "DELETE",
		requestType:    requestType,
		requestPayload: requestBody,

		responseStatus:  "OK",
		responseCode:    responseCode,
		responsePayload: responseBody,
	}
}

// Mock adds a mocked response for the given URL; whenever this URLMock is used
// as a transport for an http.Client, requests to the given URL will always
// receive the given body in their responses. Mocks specified using Mock() are
// independent of those specified using MockOnce(), except that those specified
// using MockOnce() take precedence when present.
func (m *URLMock) Mock(url string, md MockDialogue) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.mockAlways[url] = md
}

// MockOnce adds a mocked response for the given URL, to be used exactly once.
// Mocks are stored in a FIFO queue and removed from the queue as they are
// requested. Therefore, multiple requests to the same URL must each correspond
// to a call to MockOnce, in the same order that the requests will be made.
// Mocks specified this way are independent of those specified using Mock(),
// except that those specified using MockOnce() take precedence when present.
func (m *URLMock) MockOnce(url string, md MockDialogue) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.mockOnce[url]; !ok {
		m.mockOnce[url] = []MockDialogue{}
	}
	m.mockOnce[url] = append(m.mockOnce[url], md)
}

// Client returns an http.Client instance which uses the URLMock.
func (m *URLMock) Client() *http.Client {
	return &http.Client{
		Transport: m,
	}
}

// RoundTrip is an implementation of http.RoundTripper.RoundTrip. It fakes
// responses for requests to URLs based on past calls to Mock() and MockOnce().
func (m *URLMock) RoundTrip(r *http.Request) (*http.Response, error) {
	url := r.URL.String()
	var md *MockDialogue
	// Unlock not deferred because we want to be able to handle multiple
	// requests simultaneously.
	closest := "(no mocked URLs)"
	m.mtx.Lock()
	if resps, ok := m.mockOnce[url]; ok {
		if resps != nil && len(resps) > 0 {
			md = &resps[0]
			m.mockOnce[url] = m.mockOnce[url][1:]
		}
	} else if data, ok := m.mockAlways[url]; ok {
		md = &data
	} else {
		// For debugging; find the closest match.
		min := math.MaxInt32
		for mocked := range m.mockOnce {
			d := levenshtein.DistanceForStrings([]rune(url), []rune(mocked), levenshtein.DefaultOptions)
			if d < min {
				min = d
				closest = mocked
			}
		}
		for mocked := range m.mockAlways {
			d := levenshtein.DistanceForStrings([]rune(url), []rune(mocked), levenshtein.DefaultOptions)
			if d < min {
				min = d
				closest = mocked
			}
		}

	}
	m.mtx.Unlock()
	if md == nil {
		return nil, fmt.Errorf("Unknown URL %q; closest match: %s", url, closest)
	}
	return md.GetResponse(r)
}

// Empty returns true iff all of the URLs registered via MockOnce() have been
// used.
func (m *URLMock) Empty() bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for url, resps := range m.mockOnce {
		if resps != nil && len(resps) > 0 {
			sklog.Errorf("not empty: %s", url)
			return false
		}
	}
	return true
}

// List returns the list of all URLs registered via MockOnce.
func (m *URLMock) List() []string {
	rv := []string{}
	for url, resps := range m.mockOnce {
		if resps != nil && len(resps) > 0 {
			rv = append(rv, url)
		}
	}
	return rv
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

// NewURLMock returns an empty URLMock instance.
func NewURLMock() *URLMock {
	return &URLMock{
		mockAlways: map[string]MockDialogue{},
		mockOnce:   map[string][]MockDialogue{},
	}
}

// New returns a new mocked HTTPClient.
func New(urlMap map[string]MockDialogue) *http.Client {
	m := NewURLMock()
	for k, v := range urlMap {
		m.Mock(k, v)
	}
	return m.Client()
}
