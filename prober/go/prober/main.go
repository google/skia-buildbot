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
	"os"
	"strings"
	"time"

	"github.com/cyberdelia/go-metrics-graphite"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metadata"
	imetrics "go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/util"
)

var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	config         = flag.String("config", "probers.json,buildbots.json", "Comma separated names of prober config files.")
	prefix         = flag.String("prefix", "prober", "Prefix to add to all prober values sent to Graphite.")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	apikey         = flag.String("apikey", "", "The API Key used to make issue tracker requests. Only for local testing.")
	runEvery       = flag.Duration("run_every", 1*time.Minute, "How often to run the probes.")

	// bodyTesters is a mapping of names to functions that test response bodies.
	bodyTesters = map[string]BodyTester{
		"buildbotJSON":     testBuildbotJSON,
		"skfiddleJSONGood": skfiddleJSONGood,
		"skfiddleJSONBad":  skfiddleJSONBad,
		"validJSON":        validJSON,
	}
)

const (
	TIMEOUT              = time.Duration(5 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
)

// BodyTester tests the response body from a probe and returns true if it passes all tests.
type BodyTester func(io.Reader) bool

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
	BodyTestName string `json:"bodytest"`

	bodyTest BodyTester
	failure  metrics.Gauge
	latency  metrics.Gauge // Latency in ms.
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
			if f, ok := bodyTesters[v.BodyTestName]; ok {
				v.bodyTest = f
				glog.Infof("Found a body test for %s", k)
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
	return net.DialTimeout(network, addr, TIMEOUT)
}

// validJSON tests whether the response contains valid JSON.
func validJSON(r io.Reader) bool {
	var i interface{}
	return json.NewDecoder(r).Decode(&i) == nil
}

// testBuildbotJSON tests that all of the slaves are reported as connected.
func testBuildbotJSON(r io.Reader) bool {
	type SlaveStatus struct {
		Connected bool `json:"connected"`
	}

	type Slaves map[string]SlaveStatus

	dec := json.NewDecoder(r)

	slaves := make(Slaves)
	if err := dec.Decode(&slaves); err != nil {
		glog.Errorf("Failed to decode buildslave JSON: %s", err)
		return false
	}
	allConnected := true
	for k, v := range slaves {
		allConnected = allConnected && v.Connected
		if !v.Connected {
			glog.Errorf("Disconnected buildslave: %s", k)
		}
	}
	return allConnected
}

// skfiddleJSONGood tests that the compile completed w/o error.
func skfiddleJSONGood(r io.Reader) bool {
	type skfiddleResp struct {
		CompileErrors []interface{} `json:"compileErrors"`
		Message       string        `json:"message"`
	}

	dec := json.NewDecoder(r)

	s := skfiddleResp{
		CompileErrors: []interface{}{},
	}
	if err := dec.Decode(&s); err != nil {
		glog.Errorf("Failed to decode skfiddle JSON: %#v %s", s, err)
		return false
	}
	glog.Infof("%#v", s)
	return len(s.CompileErrors) == 0
}

// skfiddleJSONBad tests that the compile completed w/error.
func skfiddleJSONBad(r io.Reader) bool {
	return !skfiddleJSONGood(r)
}

// monitorIssueTracker reads the counts for all the types of issues in the skia
// issue tracker (code.google.com/p/skia) and stuffs the counts into Graphite.
func monitorIssueTracker() {
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}

	if *useMetadata {
		*apikey = metadata.Must(metadata.ProjectGet(metadata.APIKEY))
	}

	// Create a new metrics registry for the issue tracker metrics.
	addr, err := net.ResolveTCPAddr("tcp", *graphiteServer)
	if err != nil {
		glog.Fatalln("Failed to resolve the Graphite server: ", err)
	}
	issueRegistry := metrics.NewRegistry()
	go graphite.Graphite(issueRegistry, common.SAMPLE_PERIOD, "issues", addr)

	// IssueStatus has all the info we need to capture and record a single issue status. I.e. capture
	// the count of all issues with a status of "New".
	type IssueStatus struct {
		Name   string
		Metric metrics.Gauge
		URL    string
	}

	allIssueStatusLabels := []string{
		"New", "Accepted", "Unconfirmed", "Started", "Fixed", "Verified", "Invalid", "WontFix", "Done", "Available", "Assigned",
	}

	issueStatus := []*IssueStatus{}
	for _, issueName := range allIssueStatusLabels {
		issueStatus = append(issueStatus, &IssueStatus{
			Name:   issueName,
			Metric: metrics.NewRegisteredGauge(strings.ToLower(issueName), issueRegistry),
			URL:    "https://www.googleapis.com/projecthosting/v2/projects/skia/issues?fields=totalResults&key=" + *apikey + "&status=" + issueName,
		})
	}

	liveness := imetrics.NewLiveness("issue-tracker")
	for _ = range time.Tick(ISSUE_TRACKER_PERIOD) {
		for _, issue := range issueStatus {
			resp, err := c.Get(issue.URL)
			jsonResp := map[string]int64{}
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&jsonResp); err != nil {
				glog.Warningf("Failed to decode JSON response: %s", err)
				util.Close(resp.Body)
				continue
			}
			issue.Metric.Update(jsonResp["totalResults"])
			glog.Infof("Num Issues: %s - %d", issue.Name, jsonResp["totalResults"])
			if err == nil && resp.Body != nil {
				util.Close(resp.Body)
			}
		}
		liveness.Update()
	}
}

func probeOneRound(cfg Probes, c *http.Client) {
	var resp *http.Response
	var begin time.Time
	for name, probe := range cfg {
		glog.Infof("Probe: %s Starting fail value: %d", name, probe.failure.Value())
		begin = time.Now()
		var err error
		if probe.Method == "GET" {
			resp, err = c.Get(probe.URL)
		} else if probe.Method == "POST" {
			resp, err = c.Post(probe.URL, probe.MimeType, strings.NewReader(probe.Body))
		} else {
			glog.Errorf("Error: unknown method: %s", probe.Method)
			continue
		}
		if err != nil {
			glog.Errorf("Failed to make request: Name: %s URL: %s Error: %s", name, probe.URL, err)
			probe.failure.Update(1)
			continue
		}
		bodyTestResults := true
		if probe.bodyTest != nil && resp.Body != nil {
			bodyTestResults = probe.bodyTest(resp.Body)
		}
		if resp.Body != nil {
			util.Close(resp.Body)
		}
		d := time.Since(begin)
		// TODO(jcgregorio) Save the last N responses and present them in a web UI.

		if !In(resp.StatusCode, probe.Expected) {
			glog.Errorf("Got wrong status code: Got %d Want %v", resp.StatusCode, probe.Expected)
			probe.failure.Update(1)
			continue
		}
		if !bodyTestResults {
			glog.Errorf("Body test failed. %#v", probe)
			probe.failure.Update(1)
			continue
		}

		probe.failure.Update(0)
		probe.latency.Update(d.Nanoseconds() / int64(time.Millisecond))
	}
}

func main() {
	common.InitWithMetrics("probeserver", graphiteServer)
	go monitorIssueTracker()
	glog.Infoln("Looking for Graphite server.")
	addr, err := net.ResolveTCPAddr("tcp", *graphiteServer)
	if err != nil {
		glog.Fatalln("Failed to resolve the Graphite server: ", err)
	}
	glog.Infoln("Found Graphite server.")

	liveness := imetrics.NewLiveness("probes")

	// We have two sets of metrics, one for the probes and one for the probe
	// server itself. The server's metrics are handled by common.Init()
	probeRegistry := metrics.NewRegistry()
	go metrics.Graphite(probeRegistry, common.SAMPLE_PERIOD, *prefix, addr)

	// TODO(jcgregorio) Monitor config file and reload if it changes.
	cfg, err := readConfigFiles(*config)
	if err != nil {
		glog.Fatalln("Failed to read config file: ", err)
	}
	glog.Infoln("Successfully read config file.")
	// Register counters for each probe.
	for name, probe := range cfg {
		probe.failure = metrics.NewRegisteredGauge(name+".failure", probeRegistry)
		probe.latency = metrics.NewRegisteredGauge(name+".latency", probeRegistry)
	}

	// Create a client that uses our dialer with a timeout.
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}
	probeOneRound(cfg, c)
	for _ = range time.Tick(*runEvery) {
		probeOneRound(cfg, c)
		liveness.Update()
	}
}
