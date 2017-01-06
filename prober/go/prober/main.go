// Prober is an HTTP prober that periodically sends out HTTP requests to specified
// endpoints and reports if the returned results match the expectations. The results
// of the probe, including latency, are recored in InfluxDB using the Carbon protocol.
// See probers.json as an example of the config file format.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	config   = flag.String("config", "probers.json", "Comma separated names of prober config files.")
	runEvery = flag.Duration("run_every", 1*time.Minute, "How often to run the probes.")
	testing  = flag.Bool("testing", false, "Set to true for local testing.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")

	// responseTesters is a mapping of names to functions that test response bodies.
	responseTesters = map[string]ResponseTester{
		"nonZeroContenLength": nonZeroContenLength,
		"skfiddleJSONBad":     skfiddleJSONBad,
		"skfiddleJSONGood":    skfiddleJSONGood,
		"validJSON":           validJSON,
	}
)

const (
	DIAL_TIMEOUT         = time.Duration(5 * time.Second)
	REQUEST_TIMEOUT      = time.Duration(30 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
)

// ResponseTester tests the response from a probe and returns true if it passes all tests.
type ResponseTester func(io.Reader, http.Header) bool

// Probe is a single endpoint we are probing.
type Probe struct {
	// URL is the HTTP URL to probe.
	URL string `json:"url"`

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

	responseTest ResponseTester
	failure      metrics2.Int64Metric
	latency      metrics2.Int64Metric // Latency in ms.
}

// Probes is all the probes that are to be run.
type Probes map[string]*Probe

func readConfigFiles(filenames string) (Probes, error) {
	allProbes := Probes{}
	for _, filename := range strings.Split(filenames, ",") {
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to open config file: %s", err)
		}
		d := json.NewDecoder(file)
		p := &Probes{}
		if err := d.Decode(p); err != nil {
			return nil, fmt.Errorf("Failed to decode JSON in config file: %s", err)
		}
		for k, v := range *p {
			if f, ok := responseTesters[v.ResponseTestName]; ok {
				v.responseTest = f
				sklog.Infof("Found a request test for %s", k)
			}
			allProbes[k] = v
		}
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

// dialTimeout is a dialer that sets a timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, DIAL_TIMEOUT)
}

// nonZeroContenLength tests whether the Content-Length value is non-zero.
func nonZeroContenLength(r io.Reader, headers http.Header) bool {
	return headers.Get("Content-Length") != "0"
}

// validJSON tests whether the response contains valid JSON.
func validJSON(r io.Reader, headers http.Header) bool {
	var i interface{}
	return json.NewDecoder(r).Decode(&i) == nil
}

type skfiddleResp struct {
	CompileErrors []interface{} `json:"compile_errors"`
}

// skfiddleJSONGood tests that the compile completed w/o error.
func skfiddleJSONGood(r io.Reader, headers http.Header) bool {
	dec := json.NewDecoder(r)
	s := skfiddleResp{
		CompileErrors: []interface{}{},
	}
	if err := dec.Decode(&s); err != nil {
		sklog.Warningf("Failed to decode skfiddle JSON: %#v %s", s, err)
		return false
	}
	sklog.Infof("%#v", s)
	return len(s.CompileErrors) == 0
}

// skfiddleJSONBad tests that the compile completed w/error.
func skfiddleJSONBad(r io.Reader, headers http.Header) bool {
	dec := json.NewDecoder(r)
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
	if json.NewDecoder(r).Decode(&obj) != nil {
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

// monitorIssueTracker reads the counts for all the types of issues in the Skia
// issue tracker (bugs.chromium.org/p/skia) and stuffs the counts into Graphite.
func monitorIssueTracker(c *http.Client) {
	// IssueStatus has all the info we need to capture and record a single issue status. I.e. capture
	// the count of all issues with a status of "New".
	type IssueStatus struct {
		Name   string
		Metric metrics2.Int64Metric
		URL    string
	}

	allIssueStatusLabels := []string{
		"New", "Accepted", "Unconfirmed", "Started", "Fixed", "Verified", "Invalid", "WontFix", "Done", "Available", "Assigned",
	}

	issueStatus := []*IssueStatus{}
	for _, issueName := range allIssueStatusLabels {
		q := url.Values{}
		q.Set("fields", "totalResults")
		q.Set("status", issueName)
		issueStatus = append(issueStatus, &IssueStatus{
			Name:   issueName,
			Metric: metrics2.GetInt64Metric("issues", map[string]string{"status": strings.ToLower(issueName)}),
			URL:    issues.MONORAIL_BASE_URL + "?" + q.Encode(),
		})
	}

	liveness := metrics2.NewLiveness("issue-tracker")
	for _ = range time.Tick(ISSUE_TRACKER_PERIOD) {
		for _, issue := range issueStatus {
			resp, err := c.Get(issue.URL)
			if err != nil {
				sklog.Errorf("Failed to retrieve response from %s: %s", issue.URL, err)
				continue
			}
			jsonResp := map[string]int64{}
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&jsonResp); err != nil {
				sklog.Warningf("Failed to decode JSON response: %s", err)
				util.Close(resp.Body)
				continue
			}
			issue.Metric.Update(jsonResp["totalResults"])
			sklog.Infof("Num Issues: %s - %d", issue.Name, jsonResp["totalResults"])
			if err == nil && resp.Body != nil {
				util.Close(resp.Body)
			}
		}
		liveness.Reset()
	}
}

func probeOneRound(cfg Probes, c *http.Client) {
	var resp *http.Response
	var begin time.Time
	for name, probe := range cfg {
		sklog.Infof("Probe: %s Starting fail value: %d", name, probe.failure.Get())
		begin = time.Now()
		var err error
		if probe.Method == "GET" {
			resp, err = c.Get(probe.URL)
		} else if probe.Method == "HEAD" {
			resp, err = c.Head(probe.URL)
		} else if probe.Method == "POST" {
			resp, err = c.Post(probe.URL, probe.MimeType, strings.NewReader(probe.Body))
		} else {
			sklog.Errorf("Error: unknown method: %s", probe.Method)
			continue
		}
		d := time.Since(begin)
		probe.latency.Update(d.Nanoseconds() / int64(time.Millisecond))
		if err != nil {
			sklog.Warningf("Failed to make request: Name: %s URL: %s Error: %s", name, probe.URL, err)
			probe.failure.Update(1)
			continue
		}
		responseTestResults := true
		if probe.responseTest != nil && resp.Body != nil {
			responseTestResults = probe.responseTest(resp.Body, resp.Header)
		}
		if resp.Body != nil {
			util.Close(resp.Body)
		}
		// TODO(jcgregorio) Save the last N responses and present them in a web UI.

		if !In(resp.StatusCode, probe.Expected) {
			sklog.Warningf("Got wrong status code: Name %s Got %d Want %v", name, resp.StatusCode, probe.Expected)
			probe.failure.Update(1)
			continue
		}
		if !responseTestResults {
			sklog.Warningf("Response test failed: Name: %s %#v", name, probe)
			probe.failure.Update(1)
			continue
		}

		probe.failure.Update(0)
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("probeserver", influxHost, influxUser, influxPassword, influxDatabase, testing)

	client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to create client for talking to the issue tracker: %s", err)
	}
	go monitorIssueTracker(client)

	liveness := metrics2.NewLiveness("probes")

	// TODO(jcgregorio) Monitor config file and reload if it changes.
	cfg, err := readConfigFiles(*config)
	if err != nil {
		sklog.Fatalln("Failed to read config file: ", err)
	}
	sklog.Infoln("Successfully read config file.")
	// Register counters for each probe.
	for name, probe := range cfg {
		probe.failure = metrics2.GetInt64Metric("prober", map[string]string{"type": "failure", "probename": name})
		probe.latency = metrics2.GetInt64Metric("prober", map[string]string{"type": "latency", "probename": name})
	}

	// Create a client that uses our dialer with a timeout.
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
		Timeout: REQUEST_TIMEOUT,
	}
	probeOneRound(cfg, c)
	for _ = range time.Tick(*runEvery) {
		probeOneRound(cfg, c)
		liveness.Reset()
	}
}
