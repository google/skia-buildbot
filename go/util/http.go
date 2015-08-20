package util

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"

	// Below is a port of the exponential backoff implementation from
	// google-http-java-client.
	"github.com/cenkalti/backoff"
	"github.com/fiorix/go-web/autogzip"
	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
)

const (
	DIAL_TIMEOUT    = time.Minute
	REQUEST_TIMEOUT = 5 * time.Minute

	// Exponential backoff defaults.
	INITIAL_INTERVAL     = 500 * time.Millisecond
	RANDOMIZATION_FACTOR = 0.5
	BACKOFF_MULTIPLIER   = 1.5
	MAX_INTERVAL         = 60 * time.Second
	MAX_ELAPSED_TIME     = 5 * time.Minute
)

// DialTimeout is a dialer that sets a timeout.
func DialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, DIAL_TIMEOUT)
}

// NewTimeoutClient creates a new http.Client with both a dial timeout and a
// request timeout.
func NewTimeoutClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: DialTimeout,
		},
		Timeout: REQUEST_TIMEOUT,
	}
}

type BackOffConfig struct {
	initialInterval     time.Duration
	maxInterval         time.Duration
	maxElapsedTime      time.Duration
	randomizationFactor float64
	backOffMultiplier   float64
}

// NewBackOffTransport creates a BackOffTransport with default values. Look at
// NewConfiguredBackOffTransport for an example of how the values impact behavior.
func NewBackOffTransport() http.RoundTripper {
	config := &BackOffConfig{
		initialInterval:     INITIAL_INTERVAL,
		maxInterval:         MAX_INTERVAL,
		maxElapsedTime:      MAX_ELAPSED_TIME,
		randomizationFactor: RANDOMIZATION_FACTOR,
		backOffMultiplier:   BACKOFF_MULTIPLIER,
	}
	return NewConfiguredBackOffTransport(config)
}

type BackOffTransport struct {
	http.Transport
	backOffConfig *BackOffConfig
}

type ResponsePagination struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// NewBackOffTransport creates a BackOffTransport with the specified config.
//
// Example: The default retry_interval is .5 seconds, default randomization_factor
// is 0.5, default multiplier is 1.5 and the default max_interval is 1 minute. For
// 10 tries the sequence will be (values in seconds) and assuming we go over the
// max_elapsed_time on the 10th try:
//
//  request#     retry_interval     randomized_interval
//  1             0.5                [0.25,   0.75]
//  2             0.75               [0.375,  1.125]
//  3             1.125              [0.562,  1.687]
//  4             1.687              [0.8435, 2.53]
//  5             2.53               [1.265,  3.795]
//  6             3.795              [1.897,  5.692]
//  7             5.692              [2.846,  8.538]
//  8             8.538              [4.269, 12.807]
//  9            12.807              [6.403, 19.210]
//  10           19.210              backoff.Stop
func NewConfiguredBackOffTransport(config *BackOffConfig) http.RoundTripper {
	return &BackOffTransport{
		Transport:     http.Transport{Dial: DialTimeout},
		backOffConfig: config,
	}
}

// RoundTrip implements the RoundTripper interface.
func (t *BackOffTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Initialize the exponential backoff client.
	backOffClient := &backoff.ExponentialBackOff{
		InitialInterval:     t.backOffConfig.initialInterval,
		RandomizationFactor: t.backOffConfig.randomizationFactor,
		Multiplier:          t.backOffConfig.backOffMultiplier,
		MaxInterval:         t.backOffConfig.maxInterval,
		MaxElapsedTime:      t.backOffConfig.maxElapsedTime,
		Clock:               backoff.SystemClock,
	}
	// Make a copy of the request's Body so that we can reuse it if the request
	// needs to be backed off and retried.
	bodyBuf := bytes.Buffer{}
	if req.Body != nil {
		if _, err := bodyBuf.ReadFrom(req.Body); err != nil {
			return nil, fmt.Errorf("Failed to read request body: %v", err)
		}
	}

	var resp *http.Response
	var err error
	roundTripOp := func() error {
		if req.Body != nil {
			req.Body = ioutil.NopCloser(bytes.NewBufferString(bodyBuf.String()))
		}
		resp, err = t.Transport.RoundTrip(req)
		if err != nil {
			return fmt.Errorf("Error while making the round trip: %s", err)
		}
		if resp != nil {
			if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				return fmt.Errorf("Got server error statuscode %d while making the HTTP %s request to %s", resp.StatusCode, req.Method, req.URL)
			} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
				// Stop backing off if there are non server errors.
				backOffClient.MaxElapsedTime = backoff.Stop
				return fmt.Errorf("Got non server error statuscode %d while making the HTTP %s request to %s", resp.StatusCode, req.Method, req.URL)
			}
		}
		return nil
	}
	notifyFunc := func(err error, wait time.Duration) {
		glog.Warningf("Got error: %s. Retrying HTTP request after sleeping for %s", err, wait)
	}

	if err := backoff.RetryNotify(roundTripOp, backOffClient, notifyFunc); err != nil {
		return nil, fmt.Errorf("HTTP request failed inspite of exponential backoff: %s", err)
	}
	return resp, nil
}

// TODO(stephana): Remove 'r' from the argument list since it's not used. It would
// be also useful if we could specify a return status explicitly.

// ReportError formats an HTTP error response and also logs the detailed error message.
func ReportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	if err != io.ErrClosedPipe {
		http.Error(w, fmt.Sprintf("%s %s", message, err), 500)
	}
}

// responseProxy implements http.ResponseWriter and records the status codes.
type responseProxy struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rp *responseProxy) WriteHeader(code int) {
	if !rp.wroteHeader {
		glog.Infof("Response Code: %d", code)
		metrics.GetOrRegisterCounter(fmt.Sprintf("http.statuscode.%d", code), metrics.DefaultRegistry).Inc(1)
		rp.ResponseWriter.WriteHeader(code)
		rp.wroteHeader = true
	}
}

// recordResponse returns a wrapped http.Handler that records the status codes of the
// responses.
//
// Note that if a handler doesn't explicitly set a response code and goes with
// the default of 200 then this will never record anything.
func recordResponse(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(&responseProxy{ResponseWriter: w}, r)
	})
}

// LoggingGzipRequestResponse records parts of the request and the response to the logs.
func LoggingGzipRequestResponse(h http.Handler) http.Handler {
	// Closure to capture the request.
	f := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				glog.Errorf("panic serving %v: %v\n%s", r.URL.Path, err, buf)
			}
		}()
		defer timer.New(fmt.Sprintf("Request: %s %s %#v Content Length: %d Latency:", r.URL.Path, r.Method, r.URL, r.ContentLength)).Stop()
		h.ServeHTTP(w, r)
	}

	return autogzip.Handle(recordResponse(http.HandlerFunc(f)))
}

// MakeResourceHandler is an HTTP handler function designed for serving files.
func MakeResourceHandler(resourcesDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

// CorsHandler is an HTTP handler function which adds the necessary header for CORS.
func CorsHandler(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		h(w, r)
	}
}

// PaginationParams is helper function to extract pagination parameters from a
// URL query string. It assumes that 'offset' and 'size' are the query parameters
// used for pagination. It parses the values and returns an error if they are
// not integers. If the params are not set the defaults are proviced.
// Further it ensures that size is never above max size.
func PaginationParams(query url.Values, defaultOffset, defaultSize, maxSize int) (int, int, error) {
	size, err := getPositiveInt(query, "size", defaultSize)
	if err != nil {
		return 0, 0, err
	}

	offset, err := getPositiveInt(query, "offset", defaultOffset)
	if err != nil {
		return 0, 0, err
	}

	return offset, MinInt(size, maxSize), nil
}

// getPositiveInt parses the param in query and ensures it is >= 0 using
// default value when necessary.
func getPositiveInt(query url.Values, param string, defaultVal int) (int, error) {
	var val int
	var err error
	if valStr := query.Get(param); valStr == "" {
		return defaultVal, nil
	} else {
		val, err = strconv.Atoi(valStr)
		if err != nil {
			return 0, err
		}
	}
	if val < 0 {
		return defaultVal, nil
	}
	return val, nil
}
