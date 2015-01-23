package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	// Below is a port of the exponential backoff implementation from
	// google-http-java-client.
	"github.com/cenkalti/backoff"
	"github.com/skia-dev/glog"
)

const (
	// TIMEOUT is the http timeout when making requests.
	TIMEOUT = time.Duration(time.Minute)

	// Exponential backoff defaults.
	INITIAL_INTERVAL     = 500 * time.Millisecond
	RANDOMIZATION_FACTOR = 0.5
	BACKOFF_MULTIPLIER   = 1.5
	MAX_INTERVAL         = 60 * time.Second
	MAX_ELAPSED_TIME     = 5 * time.Minute
)

// DialTimeout is a dialer that sets a timeout.
func DialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, TIMEOUT)
}

// NewTimeoutClient creates a new http.Client with a timeout.
func NewTimeoutClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: DialTimeout,
		},
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
		bodyBuf.ReadFrom(req.Body)
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

// ReportError formats an HTTP error response and also logs the detailed error message.
func ReportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, fmt.Sprintf("%s %s", message, err), 500)
}
