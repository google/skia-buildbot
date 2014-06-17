// Prober is an HTTP prober that periodically sends out HTTP requests to specified
// endpoints and reports if the returned results match the expectations. The results
// of the probe, including latency, are recored in Carbon, which is presumed to run
// on the same machine as the prober. See probers.json as an example of the config
// file format.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	config = flag.String("config", "probers.json", "Name of prober config file.")
	prefix = flag.String("prefix", "prober", "Prefix to add to all prober values sent to Carbon.")
	carbon = flag.String("carbon", "localhost:2003", "Address of Carbon server and port.")
)

const (
	SAMPLE_PERIOD = time.Minute
	TIMEOUT       = time.Duration(20 * time.Second)
)

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

	Success metrics.Counter
	Failure metrics.Counter
	Latency metrics.Timer
}

// Probes is all the probes that are to be run.
type Probes map[string]*Probe

func readConfigFile(filename string) (*Probes, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %s", err)
	}
	d := json.NewDecoder(file)
	p := &Probes{}
	if err := d.Decode(p); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON in config file: %s", err)
	}
	return p, nil
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

func main() {
	flag.Parse()
	log.Println("Looking for Carbon server.")
	addr, err := net.ResolveTCPAddr("tcp", *carbon)
	if err != nil {
		log.Fatalln("Failed to resolve the Carbon server: ", err)
	}
	log.Println("Found Carbon server.")

	// We have two sets of metrics, one for the probes and one for the probe
	// server itself.
	serverRegistry := metrics.NewRegistry()
	metrics.RegisterRuntimeMemStats(serverRegistry)
	go metrics.CaptureRuntimeMemStats(serverRegistry, SAMPLE_PERIOD)
	go metrics.Graphite(serverRegistry, SAMPLE_PERIOD, "probeserver", addr)

	probeRegistry := metrics.NewRegistry()
	go metrics.Graphite(probeRegistry, SAMPLE_PERIOD, *prefix, addr)

	// TODO(jcgregorio) Monitor config file and reload if it changes.
	cfg, err := readConfigFile(*config)
	if err != nil {
		log.Fatalln("Failed to read config file: ", err)
	}
	log.Println("Successfully read config file.")
	// Register counters for each probe.
	for name, probe := range *cfg {
		probe.Success = metrics.NewRegisteredCounter(name+".success", probeRegistry)
		probe.Failure = metrics.NewRegisteredCounter(name+".failure", probeRegistry)
		probe.Latency = metrics.NewRegisteredTimer(name+".latency", probeRegistry)
	}
	var resp *http.Response
	var begin time.Time

	// Create a client that uses our dialer with a timeout.
	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}
	for _ = range time.Tick(SAMPLE_PERIOD) {
		for name, probe := range *cfg {
			log.Println("Running probe: ", name)
			begin = time.Now()
			if probe.Method == "GET" {
				resp, err = c.Get(probe.URL)
			} else if probe.Method == "POST" {
				resp, err = c.Post(probe.URL, probe.MimeType, strings.NewReader(probe.Body))
			} else {
				log.Println("Error: unknown method: ", probe.Method)
				continue
			}
			resp.Body.Close()
			d := time.Since(begin)
			// TODO(jcgregorio) Save the last N responses and present them in a web UI.
			if err == nil && In(resp.StatusCode, probe.Expected) {
				probe.Success.Inc(1)
			} else {
				probe.Failure.Inc(1)
			}
			probe.Latency.Update(d)
		}
	}
}
