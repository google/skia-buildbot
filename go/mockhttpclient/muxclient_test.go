package mockhttpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
)

func TestBasic(t *testing.T) {
	// This is the example in the documentation.

	r := chi.NewRouter()
	r.With(SchemeMatcher("https"), HostMatcher("www.google.com")).
		Get("/", MockGetDialogue([]byte("Here's a response.")).ServeHTTP)

	client := NewMuxClient(r)
	res, err := client.Get("https://www.google.com")
	require.NoError(t, err)
	respBody, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("Here's a response."), respBody)
}

func TestVars(t *testing.T) {
	// This is the example in the documentation.
	r := chi.NewRouter()
	expectedResponse := "Success."
	r.With(HostMatcher("example.com"), QueryMatcher("name", "foo", "size", "42")).
		Post("/add/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			t := MuxSafeT(t)
			values, err := url.ParseQuery(r.URL.RawQuery)
			require.NoError(t, err)
			require.Equal(t, chi.URLParam(r, "id"), "foo")
			require.Equal(t, values.Get("name"), "foo")
			_, err = w.Write([]byte(expectedResponse))
			require.NoError(t, err)
		})

	client := NewMuxClient(r)
	resp, err := client.Post("http://example.com/add/foo?name=foo&size=42", "", nil)
	require.NoError(t, err)
	actualResponse, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expectedResponse, string(actualResponse))
}

// mockTestingT mocks expect.TestingT.
type mockTestingT struct {
	errors []string
}

func (t *mockTestingT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func TestAssertionFailure(t *testing.T) {
	mockT := &mockTestingT{}

	r := chi.NewRouter()
	r.With(HostMatcher("example.com"), QueryMatcher("name", "bar", "size", "42")).
		Post("/add/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			t := MuxSafeT(mockT)
			values, err := url.ParseQuery(r.URL.RawQuery)
			require.NoError(t, err)
			require.Equal(t, chi.URLParam(r, "id"), values.Get("name"))
		})
	client := NewMuxClient(r)
	_, err := client.Post("http://example.com/add/foo?name=bar&size=42", "", nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "Test failed")
	require.Contains(t, err.Error(), "while handling HTTP request for http://example.com/add/foo?name=bar&size=42")

	require.Equal(t, 1, len(mockT.errors))
	re := regexp.MustCompile(`Not equal:\s+expected:\s+"foo"\s+actual\s+:\s+"bar"\s*`)
	require.True(t, re.MatchString(mockT.errors[0]), "Expected test failure message to match regexp %q, but got %q", re, mockT.errors[0])
}

func TestMissingHandler(t *testing.T) {
	r := chi.NewRouter()
	handlerCalled := false
	r.With(HostMatcher("example.com")).
		Post("/remove/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := client.Post("http://example.com/add/foo?name=foo&size=42", "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "No matching handler for http://example.com/add/foo?name=foo&size=42")
	require.False(t, handlerCalled)
}

func TestErrorResponse(t *testing.T) {
	r := chi.NewRouter()
	r.With(SchemeMatcher("https"), HostMatcher("www.google.com")).
		Get("/", MockGetError("TODO(benjaminwagner)", http.StatusTeapot).ServeHTTP)
	client := NewMuxClient(r)
	res, err := client.Get("https://www.google.com")
	require.NoError(t, err)
	require.Equal(t, http.StatusTeapot, res.StatusCode)
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
	require.False(t, aborted)
	require.Equal(t, io.ErrClosedPipe, writeErr)

	return resp, err
}

func TestStreamingBodyClosedForEmptyHandler(t *testing.T) {
	r := chi.NewRouter()
	r.With(HostMatcher("example.com")).
		Post("/add/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http://example.com/add/foo")
	require.NoError(t, err)
}

func TestStreamingBodyClosedForMissingHandler(t *testing.T) {
	r := chi.NewRouter()
	handlerCalled := false
	r.With(HostMatcher("example.com")).
		Post("/add/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http://example.com/remove/foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "No matching handler for http://example.com/remove/foo")
	require.False(t, handlerCalled)
}

func TestStreamingBodyClosedForInvalidURL(t *testing.T) {
	r := chi.NewRouter()
	handlerCalled := false
	r.With(HostMatcher("example.com")).
		Post("/add/{id:[a-zA-Z0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})
	client := NewMuxClient(r)
	_, err := doStreamingRequestAndAssertBodyClosed(t, client, "http:///remove/foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid request")
	require.False(t, handlerCalled)
}

func TestMockDialogueFailureInMuxClient(t *testing.T) {
	r := chi.NewRouter()
	r.With(SchemeMatcher("https"), HostMatcher("www.google.com")).
		Post("/", MockGetDialogue([]byte("Here's a response.")).ServeHTTP)
	client := NewMuxClient(r)
	_, err := client.Post("https://www.google.com", "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `Wrong Method, expected "GET", but was "POST"`)
}
