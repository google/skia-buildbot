package httputils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	// Below is a port of the exponential backoff implementation from
	// google-http-java-client.
	"github.com/cenkalti/backoff"
	"github.com/fiorix/go-web/autogzip"
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

// HealthCheckHandler returns 200 OK with an empty body, appropriate
// for a healtcheck endpoint.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
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

// NewBackOffClient creates a new http.Client with default exponential backoff
// configuration.
func NewBackOffClient() *http.Client {
	return AddMetricsToClient(&http.Client{
		Transport: NewBackOffTransport(),
	})
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
			return fmt.Errorf("Error while making the round trip to %s: %s", req.URL, err)
		}
		if resp != nil {
			if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				// We can't close the resp.Body on success, so we must do it in each of the failure cases.
				return fmt.Errorf("Got server error statuscode %d while making the HTTP %s request to %s\nResponse: %s", resp.StatusCode, req.Method, req.URL, ReadAndClose(resp.Body))
			} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
				// We can't close the resp.Body on success, so we must do it in each of the failure cases.
				// Stop backing off if there are non server errors.
				backOffClient.MaxElapsedTime = backoff.Stop
				return fmt.Errorf("Got non server error statuscode %d while making the HTTP %s request to %s\nResponse: %s", resp.StatusCode, req.Method, req.URL, ReadAndClose(resp.Body))
			}
		}
		return nil
	}
	notifyFunc := func(err error, wait time.Duration) {
		sklog.Warningf("Got error: %s. Retrying HTTP request after sleeping for %s", err, wait)
	}

	if err := backoff.RetryNotify(roundTripOp, backOffClient, notifyFunc); err != nil {
		return nil, fmt.Errorf("HTTP request failed inspite of exponential backoff: %s", err)
	}
	return resp, nil
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

// TODO(stephana): Remove 'r' from the argument list since it's not used. It would
// be also useful if we could specify a return status explicitly.

// ReportError formats an HTTP error response and also logs the detailed error message.
// The message parameter is returned in the HTTP response. If it is not provided then
// "Unknown error" will be returned instead.
func ReportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	sklog.Errorln(message, err)
	if err != io.ErrClosedPipe {
		httpErrMsg := message
		if message == "" {
			httpErrMsg = "Unknown error"
		}
		http.Error(w, httpErrMsg, 500)
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

// MakeRenamingResourceHandler is an HTTP handler function designed for serving files.
// It takes a map that can be used to alias a url. The primary usecase is to have the
// url be distinct from the file that will show the content. e.g. /foo/bar can be represented
// by my-custom-element.html in the passed in resourcesDir
func MakeRenamingResourceHandler(resourcesDir string, aliases map[string]string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		if newURL, ok := aliases[r.URL.Path]; ok {
			r.URL.Path = newURL
		}
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

// CorsCredentialsHandler is an HTTPS handler function which adds the necessary header
// for a CORS request using credentials. This allows, for example, status.skia.org to
// make a request to fuzzer.skia.org using the *.skia.org cookie that is shared
// between them.
//
// To have the browser use the cookie, the withCredentials on the XMLHttpRequest
// must be true, or the credentials option must be true when using fetch.
// In addition to this client-side setting, the server must set
// Access-Control-Allow-Credentials to be true.
//
// However, Access-Control-Allow-Origin (ACAO) cannot be the wildcard "*", or the browser
// gets upset (https://stackoverflow.com/q/19743396) The ACAO must be exactly the origin
// of the request (lists of origins aren't supported [1]). The best practice is to
// write the ACAO header dynamically, based on the requesting Origin header, which is what
// is done here, assuming the header ends with the passed in originSuffix (e.g. ".skia.org")
//
// [1] https://www.w3.org/TR/cors/#access-control-allow-origin-response-header
//     "In practice the origin-list-or-null production is more constrained. Rather
//      than allowing a space-separated list of origins, it is either a single origin
//      or the string "null"."
func CorsCredentialsHandler(h func(http.ResponseWriter, *http.Request), originSuffix string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin, ok := r.Header["Origin"]; ok {
			if strings.HasSuffix(origin[0], originSuffix) {
				w.Header().Add("Access-Control-Allow-Origin", origin[0])
				w.Header().Add("Access-Control-Allow-Credentials", "true")
			}
		}

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

// ParseFormValues reads form values from the http.Request and sets them on the
// given struct. Follows JSON decoding rules.
func ParseFormValues(r *http.Request, rv interface{}) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	// Take the first value for each key.
	m := make(map[string]string, len(r.Form))
	for k, v := range r.Form {
		m[k] = v[0]
	}

	// Decode using the json package.
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, rv)
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

// GetBaseURL strips everything but the scheme and hostname from the given URL e.g.:
//
//    https://example.com/some/path/action#abcde => https://example.com
//
// If the input URL cannot be parsed an error is returned.
func GetBaseURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	rv := url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
	}
	return rv.String(), nil
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
	s := func(w http.ResponseWriter, r *http.Request) {
		if "http" == r.Header.Get(SCHEME_AT_LOAD_BALANCER_HEADER) {
			u := *r.URL
			u.Host = r.Host
			u.Scheme = "https"
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		} else {
			// We are running in Kubernetes as a Service so the requesting IP address
			// isn't available, the only indicators that this is a healthcheck is the
			// User-Agent, the request path, and that SCHEME_AT_LOAD_BALANCER_HEADER
			// isn't set.
			if r.URL.Path == "/" && r.Header.Get("User-Agent") == "GoogleHC/1.0" {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.URL.Path == "/healthz" {
				w.WriteHeader(http.StatusOK)
				return
			}
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
