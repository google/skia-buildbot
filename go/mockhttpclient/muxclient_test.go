package mockhttpclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestBasic(t *testing.T) {
	testutils.SmallTest(t)
	// This is the example in the documentation.
	r := mux.NewRouter()
	r.Schemes("https").Host("www.google.com").Methods("GET").
		Handler(MockGetDialogue([]byte("Here's a response.")))
	client := NewMuxClient(r)
	res, err := client.Get("https://www.google.com")
	assert.NoError(t, err)
	respBody, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Here's a response."), respBody)
}

func TestVars(t *testing.T) {
	testutils.SmallTest(t)
	// This is the example in the documentation.
	r := mux.NewRouter()
	expectedResponse := "Success."
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		Queries("name", "{name}", "size", "42").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := MuxSafeT(t)
			assert.Equal(t, mux.Vars(r)["id"], mux.Vars(r)["name"])
			_, err := w.Write([]byte(expectedResponse))
			assert.NoError(t, err)
		})
	client := NewMuxClient(r)
	resp, err := client.Post("http://example.com/add/foo?name=foo&size=42", "", nil)
	assert.NoError(t, err)
	actualResponse, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, string(actualResponse))
}

// mockTestingT mocks expect.TestingT.
type mockTestingT struct {
	errors []string
}

func (t *mockTestingT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func TestAssertionFailure(t *testing.T) {
	testutils.SmallTest(t)
	mockT := &mockTestingT{}

	r := mux.NewRouter()
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		Queries("name", "{name}", "size", "42").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := MuxSafeT(mockT)
			assert.Equal(t, mux.Vars(r)["id"], mux.Vars(r)["name"])
		})
	client := NewMuxClient(r)
	_, err := client.Post("http://example.com/add/foo?name=bar&size=42", "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Test failed")
	assert.Contains(t, err.Error(), "while handling HTTP request for http://example.com/add/foo?name=bar&size=42")

	assert.Equal(t, 1, len(mockT.errors))
	re := regexp.MustCompile(`Not equal:\s+expected:\s+"foo"\s+actual\s+:\s+"bar"\s*`)
	assert.True(t, re.MatchString(mockT.errors[0]), "Expected test failure message to match regexp %q, but got %q", re, mockT.errors[0])
}

func TestMissingHandler(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	handlerCalled := false
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		// Intentional typo "naem".
		Queries("naem", "{name}", "size", "42").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := client.Post("http://example.com/add/foo?name=foo&size=42", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No matching handler for http://example.com/add/foo?name=foo&size=42")
	assert.False(t, handlerCalled)
}

func TestErrorResponse(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	r.Schemes("https").Host("www.google.com").Methods("GET").
		Handler(MockGetError("TODO(benjaminwagner)", http.StatusTeapot))
	client := NewMuxClient(r)
	res, err := client.Get("https://www.google.com")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, res.StatusCode)
}

// doStreamingRequestAndCheckBodyClosed performs a POST request to url using client with a streaming
// body. Asserts that the body is closed by client within 10 seconds. Returns the result of
// client.Post.
func doStreamingRequestAndAssertBodyClosed(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	reader, writer := io.Pipe()
	abort := time.After(10 * time.Second)
	aborted := false
	var writeErr error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer util.Close(writer)
		defer wg.Done()
		for {
			select {
			case <-abort:
				aborted = true
				return
			default:
			}
			_, writeErr = writer.Write([]byte("aaaaaaaaaaaaaaaa"))
			if writeErr != nil {
				return
			}
		}
	}()

	resp, err := client.Post(url, "text/plain", reader)

	wg.Wait()
	assert.False(t, aborted)
	assert.Equal(t, io.ErrClosedPipe, writeErr)

	return resp, err
}

func TestStreamingBodyClosedForEmptyHandler(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http://example.com/add/foo")
	assert.NoError(t, err)
}

func TestStreamingBodyClosedForMissingHandler(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	handlerCalled := false
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http://example.com/remove/foo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No matching handler for http://example.com/remove/foo")
	assert.False(t, handlerCalled)
}

func TestStreamingBodyClosedForInvalidURL(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	handlerCalled := false
	r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http:///remove/foo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
	assert.False(t, handlerCalled)
}

func TestMockDialogueFailureInMuxClient(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	r.Schemes("https").Host("www.google.com").Methods("POST").
		Handler(MockGetDialogue([]byte("Here's a response.")))
	client := NewMuxClient(r)
	_, err := client.Post("https://www.google.com", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `Wrong Method, expected "GET", but was "POST"`)
}
