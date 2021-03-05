package httputils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/fiorix/go-web/autogzip"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
)

const (
	DIAL_TIMEOUT    = time.Minute
	REQUEST_TIMEOUT = 5 * time.Minute

	FAST_DIAL_TIMEOUT    = 50 * time.Millisecond
	FAST_REQUEST_TIMEOUT = 100 * time.Millisecond

	// Exponential backoff defaults.
	INITIAL_INTERVAL     = 500 * time.Millisecond
	RANDOMIZATION_FACTOR = 0.5
	BACKOFF_MULTIPLIER   = 1.5
	MAX_INTERVAL         = 60 * time.Second
	MAX_ELAPSED_TIME     = 5 * time.Minute

	MAX_BYTES_IN_RESPONSE_BODY = 10 * 1024 //10 KB

	// SCHEME_AT_LOAD_BALANCER_HEADER is the header, added by the load balancer,
	// the has the scheme [http|https] that the original request was made under.
	SCHEME_AT_LOAD_BALANCER_HEADER = "x-forwarded-proto"
)

var (
	serverErr = errors.New("Server error")
	clientErr = errors.New("Client error")
)

// HealthCheckHandler returns 200 OK with an empty body, appropriate
// for a healtcheck endpoint.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}

// ClientConfig represents options for the behavior of an http.Client. Each field, when set,
// modifies the default http.Client behavior.
//
// Example:
// client := DefaultClientConfig().WithoutRetries().Client()
type ClientConfig struct {
	// DialTimeout, if non-zero, sets the http.Transport's dialer to a net.DialTimeout with the
	// specified timeout.
	DialTimeout time.Duration

	// RequestTimeout, if non-zero, sets the http.Client.Timeout. The timeout applies until the
	// response body is fully read. See more details in the docs for http.Client.Timeout.
	RequestTimeout time.Duration

	// Retries, if non-nil, uses a BackOffTransport to automatically retry requests until receiving a
	// non-5xx response, as specified by the BackOffConfig. See more details in the docs for
	// NewConfiguredBackOffTransport.
	Retries *BackOffConfig

	// TokenSource, if non-nil, uses a oauth2.Transport to authenticate all requests with the
	// specified TokenSource. See auth package for functions to create a TokenSource.
	TokenSource oauth2.TokenSource

	// Response2xxOnly, if true, transforms non-2xx HTTP responses to an error return value.
	Response2xxOnly bool

	// Metrics, if true, logs each request to metrics.
	Metrics bool
}

// DefaultClientConfig returns a ClientConfig with reasonable defaults.
//  - Timeouts are DIAL_TIMEOUT and REQUEST_TIMEOUT.
//  - Retries are enabled with the values from DefaultBackOffConfig().
//  - Non-2xx responses are not considered errors.
//  - Metrics are enabled.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		DialTimeout:     DIAL_TIMEOUT,
		RequestTimeout:  REQUEST_TIMEOUT,
		Retries:         DefaultBackOffConfig(),
		Response2xxOnly: false,
		Metrics:         true,
	}
}

// WithDialTimeout returns a new ClientConfig with the DialTimeout set as specified.
func (c ClientConfig) WithDialTimeout(dialTimeout time.Duration) ClientConfig {
	c.DialTimeout = dialTimeout
	return c
}

// With2xxOnly returns a new ClientConfig where non-2xx responses cause an error.
func (c ClientConfig) With2xxOnly() ClientConfig {
	c.Response2xxOnly = true
	return c
}

// WithoutRetries returns a new ClientConfig where requests are not retried.
func (c ClientConfig) WithoutRetries() ClientConfig {
	c.Retries = nil
	return c
}

// WithTokenSource returns a new ClientConfig where requests are authenticated with the given
// TokenSource.
func (c ClientConfig) WithTokenSource(t oauth2.TokenSource) ClientConfig {
	c.TokenSource = t
	return c
}

// Client returns a new http.Client as configured by the ClientConfig.
func (c ClientConfig) Client() *http.Client {
	var t http.RoundTripper = http.DefaultTransport
	if c.DialTimeout != 0 {
		t = &http.Transport{
			Dial: ConfiguredDialTimeout(c.DialTimeout),
		}
	}
	if c.Retries != nil {
		if c.RequestTimeout != 0 && c.Retries.maxElapsedTime > c.RequestTimeout {
			sklog.Warningf("Setting ClientConfig.Retries.maxElapsedTime to value of ClientConfig.RequestTimeout. Was %s, now %s.", c.Retries.maxElapsedTime, c.RequestTimeout)
			c.Retries.maxElapsedTime = c.RequestTimeout
		}
		t = NewConfiguredBackOffTransport(c.Retries, t)
	}
	if c.TokenSource != nil {
		t = &oauth2.Transport{
			Source: c.TokenSource,
			Base:   t,
		}
	}
	if c.Response2xxOnly {
		t = Response2xxOnlyTransport{t}
	}
	if c.Metrics {
		t = NewMetricsTransport(t)
	}
	return &http.Client{
		Transport: t,
		Timeout:   c.RequestTimeout,
	}
}

// DialTimeout is a dialer that sets a timeout.
func DialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, DIAL_TIMEOUT)
}

// FastDialTimeout is a dialer that sets a timeout.
func FastDialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, FAST_DIAL_TIMEOUT)
}

// ConfiguredDialTimeout is a dialer that sets a given timeout.
func ConfiguredDialTimeout(timeout time.Duration) func(string, string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, timeout)
	}
}

// NewTimeoutClient creates a new http.Client with both a dial timeout and a
// request timeout.
func NewTimeoutClient() *http.Client {
	return NewConfiguredTimeoutClient(DIAL_TIMEOUT, REQUEST_TIMEOUT)
}

// NewFastTimeoutClient creates a new http.Client with both a dial timeout and a
// request timeout.
func NewFastTimeoutClient() *http.Client {
	return NewConfiguredTimeoutClient(FAST_DIAL_TIMEOUT, FAST_REQUEST_TIMEOUT)
}

// NewConfiguredTimeoutClient creates a new http.Client with both a dial timeout
// and a request timeout.
func NewConfiguredTimeoutClient(dialTimeout, reqTimeout time.Duration) *http.Client {
	return AddMetricsToClient(&http.Client{
		Transport: &http.Transport{
			Dial: ConfiguredDialTimeout(dialTimeout),
		},
		Timeout: reqTimeout,
	})
}

// Response2xxOnlyTransport is a RoundTripper that transforms non-2xx HTTP responses to an error
// return value. Delegates all requests to the wrapped RoundTripper, which must be non-nil. Add this
// behavior to an existing client with Response2xxOnly below.
type Response2xxOnlyTransport struct {
	http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t Response2xxOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(req)
	if err == nil && resp != nil && (resp.StatusCode < 200 || resp.StatusCode > 299) {
		return nil, fmt.Errorf("Got error response status code %d from the HTTP %s request to %s\nResponse: %s", resp.StatusCode, req.Method, req.URL, ReadAndClose(resp.Body))
	}
	return resp, err
}

// Response2xxOnly modifies client so that non-2xx HTTP responses cause a non-nil error return
// value.
func Response2xxOnly(client *http.Client) *http.Client {
	wrap := client.Transport
	if wrap == nil {
		wrap = http.DefaultTransport
	}
	client.Transport = Response2xxOnlyTransport{wrap}
	return client
}

type BackOffConfig struct {
	initialInterval     time.Duration
	maxInterval         time.Duration
	maxElapsedTime      time.Duration
	randomizationFactor float64
	backOffMultiplier   float64
}

func DefaultBackOffConfig() *BackOffConfig {
	return &BackOffConfig{
		initialInterval:     INITIAL_INTERVAL,
		maxInterval:         MAX_INTERVAL,
		maxElapsedTime:      MAX_ELAPSED_TIME,
		randomizationFactor: RANDOMIZATION_FACTOR,
		backOffMultiplier:   BACKOFF_MULTIPLIER,
	}
}

type BackOffTransport struct {
	Transport     http.RoundTripper
	backOffConfig *BackOffConfig
}

type ResponsePagination struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// NewConfiguredBackOffTransport creates a BackOffTransport with the specified config, wrapping the
// given base RoundTripper.
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
func NewConfiguredBackOffTransport(config *BackOffConfig, base http.RoundTripper) http.RoundTripper {
	return &BackOffTransport{
		Transport:     base,
		backOffConfig: config,
	}
}

// RoundTrip implements the RoundTripper interface.
func (t *BackOffTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Initialize the exponential backoff client.
	backOffClient := backoff.WithContext(&backoff.ExponentialBackOff{
		InitialInterval:     t.backOffConfig.initialInterval,
		RandomizationFactor: t.backOffConfig.randomizationFactor,
		Multiplier:          t.backOffConfig.backOffMultiplier,
		MaxInterval:         t.backOffConfig.maxInterval,
		MaxElapsedTime:      t.backOffConfig.maxElapsedTime,
		Clock:               backoff.SystemClock,
	}, req.Context())
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
		if resp != nil {
			panic("Expected notifyFunc to be called between retries.")
		}
		resp, err = t.Transport.RoundTrip(req)
		if err != nil {
			return err
		}
		if resp != nil {
			if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				// This error will be retried.
				return serverErr
			} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
				// Using Permanent so that the request will not be retried.
				return backoff.Permanent(clientErr)
			}
		}
		return nil
	}
	notifyFunc := func(notifyErr error, wait time.Duration) {
		if notifyErr == serverErr {
			sklog.Warningf("Got server error status code %d while making the HTTP %s request to %s\nResponse: %s", resp.StatusCode, req.Method, req.URL, ReadAndClose(resp.Body))
			resp = nil
		} else {
			sklog.Warningf("Got error while making the round trip to %s: %s. Retrying HTTP request after sleeping for %s", req.URL, notifyErr, wait)
			if resp != nil {
				panic("Expected serverErr when resp is non-nil")
			}
		}
	}

	// Overall return values should be the return values of the final call to t.Transport.RoundTrip.
	if err := backoff.RetryNotify(roundTripOp, backOffClient, notifyFunc); err == nil || err == clientErr {
		return resp, nil
	} else if err == serverErr {
		sklog.Warningf("Final attempt got server error status code %d in spite of exponential backoff while making the HTTP %s request to %s", resp.StatusCode, req.Method, req.URL)
		return resp, nil
	} else {
		sklog.Warningf("Final attempt failed in spite of exponential backoff for HTTP %s request to %s: %s", req.Method, req.URL, err)
		return nil, err
	}
}

// ReadAndClose reads the content of a ReadCloser (e.g. http Response), and returns it as a string.
// If the response was nil or there was a problem, it will return empty string.  The reader,
// if non-null, will be closed by this function.
func ReadAndClose(r io.ReadCloser) string {
	if r != nil {
		defer util.Close(r)
		if b, err := ioutil.ReadAll(io.LimitReader(r, MAX_BYTES_IN_RESPONSE_BODY)); err != nil {
			sklog.Warningf("There was a potential problem reading the response body: %s", err)
		} else {
			return fmt.Sprintf("%q", string(b))
		}
	}
	return ""
}

// ReportError formats an HTTP error response and also logs the detailed error message.
// The message parameter is returned in the HTTP response. If it is not provided then
// "Unknown error" will be returned instead.
func ReportError(w http.ResponseWriter, err error, message string, code int) {
	sklog.Error(message, err)
	if err != io.ErrClosedPipe {
		httpErrMsg := message
		if message == "" {
			httpErrMsg = "Unknown error"
		}
		http.Error(w, httpErrMsg, code)
	}
}

// responseProxy implements http.ResponseWriter and records the status codes.
type responseProxy struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rp *responseProxy) WriteHeader(code int) {
	if !rp.wroteHeader {
		sklog.Infof("Response Code: %d", code)
		metrics2.GetCounter("http_response", map[string]string{"statuscode": strconv.Itoa(code)}).Inc(1)
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

// LoggingGzipRequestResponse records parts of the request and the response to
// the logs and gzips responses when appropriate.
func LoggingGzipRequestResponse(h http.Handler) http.Handler {
	return autogzip.Handle(LoggingRequestResponse(h))
}

// LoggingRequestResponse records parts of the request and the response to the logs.
func LoggingRequestResponse(h http.Handler) http.Handler {
	// Closure to capture the request.
	f := func(w http.ResponseWriter, r *http.Request) {
		sklog.Infof("Incoming request: %s %s %#v ", r.URL.Path, r.Method, *(r.URL))
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				sklog.Errorf("panic serving %v: %v\n%s", r.URL.Path, err, buf)

				// Note: This will only change the response if WriteHeader has not been called yet.
				// In practice that should still a lot of code since most of our HTTP handlers
				// calculate a result first and serialize it/write it to the client at the very end.
				http.Error(w, "Error Handing request", http.StatusInternalServerError)
			}
		}()
		defer timer.New(fmt.Sprintf("Request: %s Latency:", r.URL.Path)).Stop()
		h.ServeHTTP(w, r)
	}

	return recordResponse(http.HandlerFunc(f))
}

// MakeResourceHandler is an HTTP handler function designed for serving files.
func MakeResourceHandler(resourcesDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
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

	return offset, util.MinInt(size, maxSize), nil
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
			return 0, fmt.Errorf("Not a valid integer value.")
		}
	}
	if val < 0 {
		return defaultVal, nil
	}
	return val, nil
}

// MetricsTransport is an http.RoundTripper which logs each request to metrics.
type MetricsTransport struct {
	counters    map[string]metrics2.Counter
	countersMtx sync.Mutex
	rt          http.RoundTripper
}

// getCounter returns the cached metrics2.Counter for the given host.
func (mt *MetricsTransport) getCounter(host string) metrics2.Counter {
	mt.countersMtx.Lock()
	defer mt.countersMtx.Unlock()
	c, ok := mt.counters[host]
	if !ok {
		c = metrics2.GetCounter("http_request_metrics", map[string]string{
			"host": host,
		})
		mt.counters[host] = c
	}
	return c
}

// See docs for http.RoundTripper.
func (mt *MetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	mt.getCounter(req.URL.Host).Inc(1)
	return mt.rt.RoundTrip(req)
}

// NewMetricsTransport returns a MetricsTransport instance which wraps the given
// http.RoundTripper.
func NewMetricsTransport(rt http.RoundTripper) http.RoundTripper {
	// Prevent double-wrapping and thus double-counting requests in metrics.
	if rt == nil {
		rt = &http.Transport{
			Dial: DialTimeout,
		}
	} else {
		if reflect.TypeOf(rt) == reflect.TypeOf(&MetricsTransport{}) {
			return rt
		}
	}
	return &MetricsTransport{
		counters: map[string]metrics2.Counter{},
		rt:       rt,
	}
}

// AddMetricsToClient adds metrics for each request to the http.Client.
func AddMetricsToClient(c *http.Client) *http.Client {
	c.Transport = NewMetricsTransport(c.Transport)
	return c
}

// HTTPS forces traffic to go over HTTPS.  See:
// https://github.com/kubernetes/ingress-gce#redirecting-http-to-https
//
// h - The http.Handler to wrap.
//
// Example:
//    if !*local {
//      h := httputils.HTTPS(h)
//    }
//    http.Handle("/", h)
//
func HTTPS(h http.Handler) http.Handler {
	s := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(SCHEME_AT_LOAD_BALANCER_HEADER) == "http" {
			u := *r.URL
			u.Host = r.Host
			u.Scheme = "https"
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			return
		} else {
			h.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(s)
}

// HealthzAndHTTPS forces traffic to go over HTTPS and also handles
// healthchecks at /healthz.  See:
// https://github.com/kubernetes/ingress-gce#redirecting-http-to-https
//
// h - The http.Handler to wrap.
//
// Example:
//    if !*local {
//      h := httputils.HealthzAndHTTPS(h)
//    }
//    http.Handle("/", h)
//
func HealthzAndHTTPS(h http.Handler) http.Handler {
	return Healthz(HTTPS(h))
}

// Healthz handles healthchecks at /healthz and GFE healthchecks at /.
//
// Example:
//    if !*local {
//      h := httputils.Healthz(h)
//    }
//    http.Handle("/", h)
//
func Healthz(h http.Handler) http.Handler {
	s := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Header.Get("User-Agent") == "GoogleHC/1.0" {
			w.WriteHeader(http.StatusOK)
			return
		} else if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		} else {
			h.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(s)
}

// ReadyHandleFunc can be used to set up a ready-handler used to check
// whether a service is ready. Simply returns 'ready'.
func ReadyHandleFunc(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("ready"))
	util.LogErr(err)
}

// RunHealthCheckServer is a helper function which runs an HTTP server which
// only handles health checks. This is used for processes which don't run an
// HTTP server of their own but still want health checks. Does not return.
func RunHealthCheckServer(port string) {
	h := http.NotFoundHandler()
	h = HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(port, nil))
}

// GetWithContext is a helper function to execute a GET request to the given url using the
// given client and the provided context.
func GetWithContext(ctx context.Context, c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// PostWithContext is a helper function to execute a POST request to the given url using the
// given client and the provided context, contentType and body.
func PostWithContext(ctx context.Context, c *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// CrossOriginResourcePolicy adds a Cross-Origin-Resource-Policy: cross-origin
// to every response.
//
// Example:
//    if !*local {
//      h := httputils.CrossOriginResourcePolicy(h)
//    }
//    http.Handle("/", h)
//
func CrossOriginResourcePolicy(h http.Handler) http.Handler {
	s := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(s)
}

// XFrameOptionsDeny adds "X-Frame-Options: DENY" to every response.
func XFrameOptionsDeny(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}
