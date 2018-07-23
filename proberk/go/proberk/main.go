// Proberk is an HTTP prober that periodically sends out HTTP requests to specified
// endpoints and reports if the returned results match the expectations. The results
// of the probe, including latency, are recored in metrics2.
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flynn/json5"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/proberk/go/types"
)

// flags
var (
	config   = flag.String("config", "probersk.json5", "Prober config filename.")
	promPort = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	runEvery = flag.Duration("run_every", 1*time.Minute, "How often to run the probes.")
	validate = flag.Bool("validate", false, "Validate the config file and then exit.")
)

var (
	// responseTesters is a mapping of names to functions that test response bodies.
	responseTesters = map[string]types.ResponseTester{
		"nonZeroContentLength":     nonZeroContentLength,
		"skfiddleJSONBad":          skfiddleJSONBad,
		"skfiddleJSONGood":         skfiddleJSONGood,
		"skfiddleJSONSecViolation": skfiddleJSONSecViolation,
		"validJSON":                validJSON,
	}

	// The hash of the config file contents when the app started.
	startHash = ""
)

const (
	DIAL_TIMEOUT         = time.Duration(5 * time.Second)
	REQUEST_TIMEOUT      = time.Duration(30 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
)

func readConfigFile(filename string) (types.Probes, error) {
	allProbes := types.Probes{}
	errs := []string{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %s", err)
	}
	d := json5.NewDecoder(file)
	p := &types.Probes{}
	if err := d.Decode(p); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON in config file: %s", err)
	}
	for k, v := range *p {
		v.Failure = map[string]metrics2.Int64Metric{}
		v.Latency = map[string]metrics2.Int64Metric{}
		if v.ResponseTestName != "" {
			if f, ok := responseTesters[v.ResponseTestName]; ok {
				v.ResponseTest = f
			} else {
				errs = append(errs, fmt.Sprintf("ResponseTestName Not Found %q", k))
			}
		}
		allProbes[k] = v
	}
	if len(errs) != 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "\n  "))
	}
	return allProbes, nil
}

// In returns true if n is found in list.
func In(n int, list []int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

// nonZeroContentLength tests whether the Content-Length value is non-zero.
func nonZeroContentLength(r io.Reader, headers http.Header) bool {
	return headers.Get("Content-Length") != "0"
}

// validJSON tests whether the response contains valid JSON.
func validJSON(r io.Reader, headers http.Header) bool {
	var i interface{}
	return json5.NewDecoder(r).Decode(&i) == nil
}

type skfiddleResp struct {
	CompileErrors []interface{} `json:"compile_errors"`
	RuntimeError  string        `json:"runtime_error"`
}

// skfiddleJSONSecViolation tests that the compile failed with a runtime error (which includes security violations).
func skfiddleJSONSecViolation(r io.Reader, headers http.Header) bool {
	dec := json5.NewDecoder(r)
	s := skfiddleResp{
		CompileErrors: []interface{}{},
	}
	if err := dec.Decode(&s); err != nil {
		sklog.Warningf("Failed to decode skfiddle JSON: %#v %s", s, err)
		return false
	}
	sklog.Infof("%#v", s)
	return s.RuntimeError != ""
}

// skfiddleJSONGood tests that the compile completed w/o error.
func skfiddleJSONGood(r io.Reader, headers http.Header) bool {
	dec := json5.NewDecoder(r)
	s := skfiddleResp{
		CompileErrors: []interface{}{},
	}
	if err := dec.Decode(&s); err != nil {
		sklog.Warningf("Failed to decode skfiddle JSON: %#v %s", s, err)
		return false
	}
	sklog.Infof("%#v", s)
	return len(s.CompileErrors) == 0 && s.RuntimeError == ""
}

// skfiddleJSONBad tests that the compile completed w/error.
func skfiddleJSONBad(r io.Reader, headers http.Header) bool {
	dec := json5.NewDecoder(r)
	s := skfiddleResp{
		CompileErrors: []interface{}{},
	}
	if err := dec.Decode(&s); err != nil {
		sklog.Warningf("Failed to decode skfiddle JSON: %#v %s", s, err)
		return false
	}
	sklog.Infof("%#v", s)
	return len(s.CompileErrors) != 0
}

// decodeJSONObject reads a JSON object from r and returns the resulting object. Returns nil if the
// JSON is invalid or can't be decoded to a map[string]interface{}.
func decodeJSONObject(r io.Reader) map[string]interface{} {
	var obj map[string]interface{}
	if json5.NewDecoder(r).Decode(&obj) != nil {
		return nil
	}
	return obj
}

// hasKeys tests that the given decoded JSON object has at least the provided keys. If obj is nil,
// returns false.
func hasKeys(obj map[string]interface{}, keys []string) bool {
	if obj == nil {
		return false
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			return false
		}
	}
	return true
}

func probeOneRound(cfg types.Probes, c *http.Client) {
	var resp *http.Response
	var begin time.Time
	for name, probe := range cfg {
		for _, url := range probe.URLs {
			sklog.Infof("Probe: %s Starting fail value: %d", name, probe.Failure[url].Get())
			begin = time.Now()
			var err error
			if probe.Method == "GET" {
				resp, err = c.Get(url)
			} else if probe.Method == "HEAD" {
				resp, err = c.Head(url)
			} else if probe.Method == "POST" {
				resp, err = c.Post(url, probe.MimeType, strings.NewReader(probe.Body))
			} else {
				sklog.Errorf("Error: unknown method: %s", probe.Method)
				continue
			}
			d := time.Since(begin)
			probe.Latency[url].Update(d.Nanoseconds() / int64(time.Millisecond))
			if err != nil {
				sklog.Warningf("Failed to make request: Name: %s URL: %s Error: %s", name, url, err)
				probe.Failure[url].Update(1)
				continue
			}
			responseTestResults := true
			if probe.ResponseTest != nil && resp.Body != nil {
				responseTestResults = probe.ResponseTest(resp.Body, resp.Header)
			}
			if resp.Body != nil {
				util.Close(resp.Body)
			}
			// TODO(jcgregorio) Save the last N responses and present them in a web UI.

			if !In(resp.StatusCode, probe.Expected) {
				sklog.Errorf("Got wrong status code: Name %s Got %d Want %v", name, resp.StatusCode, probe.Expected)
				probe.Failure[url].Update(1)
				continue
			}
			if !responseTestResults {
				sklog.Warningf("Response test failed: Name: %s %#v", name, probe)
				probe.Failure[url].Update(1)
				continue
			}

			probe.Failure[url].Update(0)
		}
	}
}

func getHash() (string, error) {
	f, err := os.Open(*config)
	if err != nil {
		return "", fmt.Errorf("Failed to read config file while checking hash: %s", err)
	}
	defer util.Close(f)

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("Failed to copy bytes while checking hash: %s", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func main() {
	common.InitWithMust(
		"probeserver",
		common.PrometheusOpt(promPort),
	)
	var err error
	startHash, err = getHash()
	if err != nil {
		sklog.Fatalln("Failed to calculate hash of config file: ", err)
	}
	cfg, err := readConfigFile(*config)
	if *validate {
		if err != nil {
			fmt.Printf("Validation Failed:\n  %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Validation Successful")
		os.Exit(0)
	}
	if err != nil {
		sklog.Fatalln("Failed to read config file: ", err)
	}

	liveness := metrics2.NewLiveness("probes")

	// Register counters for each probe.
	for name, probe := range cfg {
		for _, url := range probe.URLs {
			probe.Failure[url] = metrics2.GetInt64Metric("prober", map[string]string{"type": "failure", "probename": name, "url": url})
			probe.Latency[url] = metrics2.GetInt64Metric("prober", map[string]string{"type": "latency", "probename": name, "url": url})
		}
	}

	// Create a client that uses our dialer with a timeout.
	c := httputils.NewConfiguredTimeoutClient(DIAL_TIMEOUT, REQUEST_TIMEOUT)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	probeOneRound(cfg, c)
	for range time.Tick(*runEvery) {
		probeOneRound(cfg, c)
		liveness.Reset()

		currentHash, err := getHash()
		if err != nil {
			sklog.Errorf("Failed to verify hash of config file: %s", err)
			continue
		}
		if currentHash != startHash {
			fmt.Println("Restarting to pick up new config.")
			os.Exit(0)
		}
	}
}
