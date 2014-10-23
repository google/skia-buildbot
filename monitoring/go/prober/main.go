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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/perf/go/flags"
)

var (
	config     = flag.String("config", "probers.json,buildbots.json", "Comma separated names of prober config files.")
	prefix     = flag.String("prefix", "prober", "Prefix to add to all prober values sent to Carbon.")
	carbon     = flag.String("carbon", "localhost:2003", "Address of Carbon server and port.")
	apikeyFlag = flag.String("apikey", "", "The API Key used to make issue tracker requests. Only for local testing.")
	runEvery   = flag.Duration("run_every", 1*time.Minute, "How often to run the probes.")

	// bodyTesters is a mapping of names to functions that test response bodies.
	bodyTesters = map[string]BodyTester{
		"buildbotJSON": testBuildbotJSON,
	}
)

const (
	SAMPLE_PERIOD        = time.Minute
	TIMEOUT              = time.Duration(5 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
	APIKEY_METADATA_URL  = "http://metadata/computeMetadata/v1/instance/attributes/apikey"
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
	latency  metrics.Timer
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

// monitorIssueTracker reads the counts for all the types of issues in the skia
// issue tracker (code.google.com/p/skia) and stuffs the counts into Graphite.
func monitorIssueTracker() {
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}
	apikey := *apikeyFlag
	// If apikey isn't passed in then read it from the metadata server.
	if apikey == "" {
		// Get the API Key we need to make requests to the issue tracker.
		req, err := http.NewRequest("GET", APIKEY_METADATA_URL, nil)
		if err != nil {
			glog.Fatalln(err)
		}
		req.Header.Add("X-Google-Metadata-Request", "True")
		if resp, err := c.Do(req); err == nil {
			apikeyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				glog.Fatalln("Failed to read password from metadata server:", err)
			}
			apikey = string(apikeyBytes)
		}
	}

	// Create a new metrics registry for the issue tracker metrics.
	addr, err := net.ResolveTCPAddr("tcp", *carbon)
	if err != nil {
		glog.Fatalln("Failed to resolve the Carbon server: ", err)
	}
	issueRegistry := metrics.NewRegistry()
	go metrics.Graphite(issueRegistry, SAMPLE_PERIOD, "issues", addr)

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
			URL:    "https://www.googleapis.com/projecthosting/v2/projects/skia/issues?fields=totalResults&key=" + apikey + "&status=" + issueName,
		})
	}

	for _ = range time.Tick(ISSUE_TRACKER_PERIOD) {
		for _, issue := range issueStatus {
			resp, err := c.Get(issue.URL)
			jsonResp := map[string]int64{}
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&jsonResp); err != nil {
				glog.Warningf("Failed to decode JSON response: %s", err)
				resp.Body.Close()
				continue
			}
			issue.Metric.Update(jsonResp["totalResults"])
			glog.Infof("Num Issues: %s - %d", issue.Name, jsonResp["totalResults"])
			if err == nil && resp.Body != nil {
				resp.Body.Close()
			}
		}
	}
}

func main() {
	flag.Parse()
	flags.Log()
	defer glog.Flush()
	go monitorIssueTracker()
	glog.Infoln("Looking for Carbon server.")
	addr, err := net.ResolveTCPAddr("tcp", *carbon)
	if err != nil {
		glog.Fatalln("Failed to resolve the Carbon server: ", err)
	}
	glog.Infoln("Found Carbon server.")

	// We have two sets of metrics, one for the probes and one for the probe
	// server itself.
	serverRegistry := metrics.NewRegistry()
	metrics.RegisterRuntimeMemStats(serverRegistry)
	go metrics.CaptureRuntimeMemStats(serverRegistry, SAMPLE_PERIOD)
	go metrics.Graphite(serverRegistry, SAMPLE_PERIOD, "probeserver", addr)

	probeRegistry := metrics.NewRegistry()
	go metrics.Graphite(probeRegistry, SAMPLE_PERIOD, *prefix, addr)

	// TODO(jcgregorio) Monitor config file and reload if it changes.
	cfg, err := readConfigFiles(*config)
	if err != nil {
		glog.Fatalln("Failed to read config file: ", err)
	}
	glog.Infoln("Successfully read config file.")
	// Register counters for each probe.
	for name, probe := range cfg {
		probe.failure = metrics.NewRegisteredGauge(name+".failure", probeRegistry)
		probe.latency = metrics.NewRegisteredTimer(name+".latency", probeRegistry)
	}
	var resp *http.Response
	var begin time.Time

	// Create a client that uses our dialer with a timeout.
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}
	for _ = range time.Tick(*runEvery) {
		for name, probe := range cfg {
			glog.Infof("Probe: %s Starting fail value: %d", name, probe.failure.Value())
			begin = time.Now()
			if probe.Method == "GET" {
				resp, err = c.Get(probe.URL)
			} else if probe.Method == "POST" {
				resp, err = c.Post(probe.URL, probe.MimeType, strings.NewReader(probe.Body))
			} else {
				glog.Errorf("Error: unknown method: ", probe.Method)
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
				resp.Body.Close()
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
			probe.latency.Update(d)
		}
	}
}
