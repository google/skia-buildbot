package mockhttpclient

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	expect "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
)

// muxClient implements http.RoundTripper and sends requests to a mux.Router.
type muxClient struct {
	router *mux.Router
}

// muxClientNotFoundHandler provides a useful error message for client requests that don't match any
// mux.Route.
func muxClientNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, fmt.Sprintf("No matching handler for %s", r.URL.String()), TEST_FAILED_STATUS_CODE)
}

// NewMuxClient returns an http.Client instance which sends requests to the given mux.Router.
//
// NewMuxClient is more flexible than using httptest.NewServer because the returned client can
// accept requests for any scheme or host. It is more flexible than URLMock because it allows
// handling arbitrary URLs with arbitrary handlers. However, it is more difficult to use than
// URLMock when the same request URL should be handled differently on subsequent requests.
//
// TODO(benjaminwagner): NewMuxClient does not currently support streaming responses, but does
// support streaming requests.
//
// Examples:
//
//    // Mock out a URL to always respond with the same body.
//    r := mux.NewRouter()
//    r.Schemes("https").Host("www.google.com").Methods("GET").
//        Handler(MockGetDialogue([]byte("Here's a response.")))
//    client := NewMuxClient(r)
//    res, _ := client.Get("https://www.google.com")
//    respBody, _ := ioutil.ReadAll(res.Body)  // respBody == []byte("Here's a response.")
//
//    // Check that the client uses the correct ID in the request.
//    r.Host("example.com").Methods("POST").Path("/add/{id:[a-zA-Z0-9]+}").
//        Queries("name", "{name}", "size", "42").
//        HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//          t := MuxSafeT(t)
//          assert.Equal(t, mux.Vars(r)["id"], mux.Vars(r)["name"])
//        })
func NewMuxClient(r *mux.Router) *http.Client {
	if r.NotFoundHandler == nil {
		r.NotFoundHandler = http.HandlerFunc(muxClientNotFoundHandler)
	}
	m := &muxClient{
		router: r,
	}
	return &http.Client{
		Transport: m,
	}
}

// responseWriter implements http.ResponseWriter for handlers and provides an http.Response for
// clients.
type responseWriter struct {
	resp http.Response
	body bytes.Buffer
}

func newResponseWriter() *responseWriter {
	w := &responseWriter{}
	w.resp.Body = &respBodyCloser{&w.body}
	w.resp.Header = http.Header{}
	w.resp.StatusCode = http.StatusOK
	w.resp.ContentLength = -1
	return w
}

func (w *responseWriter) Header() http.Header {
	return w.resp.Header
}

func (w *responseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *responseWriter) WriteHeader(code int) {
	w.resp.StatusCode = code
}

// RoundTrip is an implementation of http.RoundTripper.RoundTrip. It sends requests to the
// mux.Router.
func (m *muxClient) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	defer func() {
		if req.Body != nil {
			util.Close(req.Body)
		}
	}()
	if req.URL == nil || req.URL.Scheme == "" || req.URL.Host == "" || req.Header == nil {
		return nil, fmt.Errorf("invalid request; URL: %#v Header: %#v", req.URL, req.Header)
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}

	w := newResponseWriter()

	// Check for muxClientFailNowValue and set err if found.
	defer func() {
		r := recover()
		if r != nil {
			v, ok := r.(muxClientFailNowValue)
			if ok {
				loc := ""
				if v.file != "" {
					loc = fmt.Sprintf("at %s:%d ", v.file, v.line)
				}
				err = fmt.Errorf("Test failed %swhile handling HTTP request for %s", loc, req.URL.String())
			} else {
				panic(r)
			}
		}
	}()
	m.router.ServeHTTP(w, req)
	if w.resp.StatusCode == TEST_FAILED_STATUS_CODE {
		return nil, errors.New(w.body.String())
	}

	w.resp.Request = req
	return &w.resp, nil
}

// muxSafeT implements assert.TestingT (aka require.TestingT) but allows muxClient to translate
// FailNow into a regular error. This is necessary because some users of http.Client behave badly
// when runtime.Goexit() (called by testing.T.FailNow) is called within muxClient.RoundTrip.
type muxSafeT struct {
	expect.TestingT
}

// MuxSafeT wraps *testing.T to allow using the assert package (aka require package) within handler
// functions of the mux.Router passed to MuxClient. This is necessary because some users of
// http.Client behave badly when runtime.Goexit() (called by testing.T.FailNow) occurs during a
// request.
//
// The documentation for testing.T.FailNow states "FailNow must be called from the goroutine running
// the test or benchmark function, not from other goroutines created during the test," so if the
// http.Client returned from MuxClient is used by a different goroutine, you should use MuxSafeT to
// ensure the test doesn't hang.
func MuxSafeT(orig expect.TestingT) require.TestingT {
	return muxSafeT{orig}
}

// muxClientFailNowValue indicates to muxClient.RoundTrip that the test has failed and records the
// file and line where the failure occurred.
type muxClientFailNowValue struct {
	file string
	line int
}

// Implements assert.TestingT.FailNow().
func (muxSafeT) FailNow() {
	// 3 frames up seems to give the correct spot.
	_, file, line, ok := runtime.Caller(3)
	if ok {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
		panic(muxClientFailNowValue{
			file: file,
			line: line,
		})
	}
	panic(muxClientFailNowValue{})
}
