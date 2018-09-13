package types

import (
	"io"
	"net/http"

	"go.skia.org/infra/go/metrics2"
)

// ResponseTester tests the response from a probe and returns true if it passes all tests.
type ResponseTester func(io.Reader, http.Header) bool

// Probe is a single endpoint we are probing.
type Probe struct {
	// URL is the HTTP URL to probe.
	URLs []string `json:"urls"`

	// Method is the HTTP method to use when probing.
	Method string `json:"method"`

	// Expected is the list of expected HTTP status code, i.e. [200, 201]
	Expected []int `json:"expected"`

	// Body is the body of the request to send if the method is POST.
	Body string `json:"body"`

	// The mimetype of the Body.
	MimeType string `json:"mimetype"`

	// The body testing function we should use.
	ResponseTestName string `json:"responsetest"`

	// If true, attach an OAuth 2.0 Bearer Token to the request.
	Authenticated bool `json:"authenticated"`

	ResponseTest ResponseTester `json:"-"`

	//      map[url]metric.
	Failure map[string]metrics2.Int64Metric `json:"-"`
	Latency map[string]metrics2.Int64Metric `json:"-"` // Latency in ms.
}

// Probes is all the probes that are to be run.
type Probes map[string]*Probe
