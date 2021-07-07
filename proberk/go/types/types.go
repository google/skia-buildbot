package types

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	_ "embed" // For embed functionality.

	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// schema is a json schema for InstanceConfig, it is created by
// running go generate on ./generate/main.go.
//
//go:embed probesSchema.json
var schema []byte

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
	Body string `json:"body,omitempty"`

	// The mimetype of the Body.
	MimeType string `json:"mimetype"`

	// The body testing function we should use.
	ResponseTestName string `json:"responsetest,omitempty"`

	// If true, attach an OAuth 2.0 Bearer Token to the request.
	Authenticated bool `json:"authenticated,omitempty"`

	ResponseTest ResponseTester `json:"-"`

	//      map[url]metric.
	Failure map[string]metrics2.Int64Metric `json:"-"`
	Latency map[string]metrics2.Int64Metric `json:"-"` // Latency in ms.

	// Note is some comment about this prober.
	Note string `json:"note,omitempty"`
}

// Probes is all the probes that are to be run.
type Probes map[string]*Probe

// LoadFromJSONFile loads the configuration of the probers from the given JSON
// file.
func LoadFromJSONFile(ctx context.Context, filename string) (Probes, error) {
	var probes Probes
	err := util.WithReadFile(filename, func(r io.Reader) error {
		document, err := ioutil.ReadAll(r)
		if err != nil {
			return skerr.Wrap(err)
		}
		validationErrors, err := jsonschema.Validate(ctx, document, schema)
		if err != nil {
			for _, v := range validationErrors {
				sklog.Error(v)
			}
		}
		err = json.Unmarshal(document, &probes)
		if err != nil {
			return fmt.Errorf("failed to decode JSON in config file: %s", err)
		}
		return nil
	})
	return probes, err
}
