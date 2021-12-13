// Proberk is an HTTP prober that periodically sends out HTTP requests to specified
// endpoints and reports if the returned results match the expectations. The results
// of the probe, including latency, are recorded in metrics2.
package main

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/proberk/go/types"
)

// flags
var (
	config          = flag.String("config", "probersk.json5", "Prober config filename.")
	expectationsDir = flag.String("expectations_dir", "expectations", "Directory with expectations files.")
	local           = flag.Bool("local", false, "True if running locally (as opposed to in production).")
	promPort        = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	runEvery        = flag.Duration("run_every", 1*time.Minute, "How often to run the probes.")
	validate        = flag.Bool("validate", false, "Validate the config file and then exit.")
)

var (
	// responseTesters is a mapping of names to functions that test response bodies.
	responseTesters = map[string]types.ResponseTester{
		"nonZeroContentLength": nonZeroContentLength,
		"skfiddleJSONBad":      skfiddleJSONBad,
		"skfiddleJSONGood":     skfiddleJSONGood,
		"validJSON":            validJSON,
		"gobPublicReposGood":   gobPublicReposGood,
	}

	// The hash of the config file contents when the app started.
	startHash = ""
)

const (
	dialTimeout             = 5 * time.Second
	requestTimeout          = 30 * time.Second
	defaultSSLValidDuration = 10 * 24 * time.Hour // cert should be valid for at least 10 days
)

func readConfigFile(ctx context.Context, filename string) (types.Probes, error) {
	p, err := types.LoadFromJSONFile(ctx, filename)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read config file: %s", filename)
	}

	allProbes := types.Probes{}
	var errs []string
	for k, v := range p {
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
		return nil, skerr.Fmt("%s", strings.Join(errs, "\n  "))
	}
	return allProbes, nil
}

// in returns true if n is found in the slice of integers.
func in(n int, list []int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

// nonZeroContentLength tests whether the Content-Length value is non-zero.
func nonZeroContentLength(_ io.Reader, headers http.Header) bool {
	return headers.Get("Content-Length") != "0"
}

// validJSON tests whether the response contains valid JSON.
func validJSON(r io.Reader, _ http.Header) bool {
	var i interface{}
	return json5.NewDecoder(r).Decode(&i) == nil
}

type skfiddleResp struct {
	CompileErrors []interface{} `json:"compile_errors"`
	RuntimeError  string        `json:"runtime_error"`
}

// skfiddleJSONGood tests that the compile completed w/o error.
func skfiddleJSONGood(r io.Reader, _ http.Header) bool {
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
func skfiddleJSONBad(r io.Reader, _ http.Header) bool {
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

// gobPublicReposGood confirms the response matches the file contents stored in
// expectations/gob.json.
func gobPublicReposGood(r io.Reader, _ http.Header) bool {
	gobb, err := ioutil.ReadFile(filepath.Join(*expectationsDir, "gob.json"))
	if err != nil {
		sklog.Errorf("Failed to read probe expectation: %s", err)
		return false
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		sklog.Errorf("Failed to read probe response: %s", err)
		return false
	}
	sklog.Info("Comparing expectations.")
	ret := string(b) == string(gobb)
	if !ret {
		sklog.Warningf("GoB expectations didn't match, check for new or removed repos.")
	}
	return ret
}

func probeOneRound(cfg types.Probes, anonymousClient, authClient *http.Client) {
	var resp *http.Response
	var begin time.Time
	for name, probe := range cfg {
		c := anonymousClient
		if probe.Authenticated {
			c = authClient
		}
		for _, u := range probe.URLs {
			sklog.Infof("Probe: %s Starting fail value: %d", name, probe.Failure[u].Get())
			begin = time.Now()
			var err error
			if probe.Method == "GET" {
				resp, err = c.Get(u)
			} else if probe.Method == "HEAD" {
				resp, err = c.Head(u)
			} else if probe.Method == "POST" {
				resp, err = c.Post(u, probe.MimeType, strings.NewReader(probe.Body))
			} else if probe.Method == "SSL" {
				// SSL is a fictitious method that tests the SSL cert.
				// TODO(stephana): We should consider refactoring the prober into a
				// more generic prober beyond just HTTP related probes. SSL probing
				// would fit cleaner into such a framework.
				if err := probeSSL(probe, u); err != nil {
					sklog.Errorf("While testing %s we got SSL error: %s", u, err)
					probe.Failure[u].Update(1)
				} else {
					probe.Failure[u].Update(0)
				}
				continue
			} else {
				sklog.Errorf("Error: unknown method: %s", probe.Method)
				continue
			}
			d := time.Since(begin)
			probe.Latency[u].Update(d.Nanoseconds() / int64(time.Millisecond))
			if err != nil {
				sklog.Warningf("Failed to make request: Name: %s URL: %s Error: %s", name, u, err)
				probe.Failure[u].Update(1)
				continue
			}
			if resp != nil {
				responseTestResults := true
				if probe.ResponseTest != nil && resp.Body != nil {
					responseTestResults = probe.ResponseTest(resp.Body, resp.Header)
				}
				if resp.Body != nil {
					util.Close(resp.Body)
				}
				// TODO(jcgregorio) Save the last N responses and present them in a web UI.

				if !in(resp.StatusCode, probe.Expected) {
					sklog.Errorf("Got wrong status code: Name: %s URL: %s Got: %d Want: %v", name, u, resp.StatusCode, probe.Expected)
					probe.Failure[u].Update(1)
					continue
				}
				if !responseTestResults {
					sklog.Warningf("Response test failed: Name: %s %#v", name, probe)
					probe.Failure[u].Update(1)
					continue
				}
			}

			probe.Failure[u].Update(0)
		}
	}
}

// probeSSL inspects the SSL cert for the given URL and checks whether
// the time to expiration is below a certain number of days.
func probeSSL(probe *types.Probe, URL string) error {
	parsedURL, err := url.Parse(URL)
	targetAddr := parsedURL.Host
	if parsedURL.Port() == "" {
		targetAddr += ":443"
	}

	// TODO(stephana): Revisit whether we want to do host checking.

	// Don't verify the host, since we also want to test IP addresses and
	// this is not a generic SSL prober. We will catch a host mismatch during
	// manual testing.
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", targetAddr, conf)
	if err != nil {
		return skerr.Fmt("Error dialing %s: %s", targetAddr, err)
	}
	defer util.Close(conn)

	// Establish the connection.
	if err := conn.Handshake(); err != nil {
		return skerr.Fmt("Handshake with %s failed: %s", targetAddr, err)
	}

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return skerr.Fmt("Unable to retrieve peer certificates for %s", targetAddr)
	}

	// Validate the certificate chain
	now := time.Now()
	for _, cert := range certs {
		// Make sure the cert is valid.
		if now.Before(cert.NotBefore) {
			return skerr.Fmt("Certificate for %s is not valid yet", targetAddr)
		}

		// If the 'expected' value of the probe configuration contains a positive
		// integer we interpret it as the number of days in the future the cert
		// should be valid.
		minExpirationDelta := defaultSSLValidDuration
		if len(probe.Expected) > 0 && probe.Expected[0] > 0 {
			minExpirationDelta = time.Duration(probe.Expected[0]) * 24 * time.Hour
		}

		// Make sure the cert is not expired.
		delta := cert.NotAfter.Sub(now)
		if delta < minExpirationDelta {
			return skerr.Fmt("Certificate for %s is expired or will expire in %s.", targetAddr, human.Duration(delta))
		}
	}

	return nil
}

func getHashOfConfigFile() (string, error) {
	f, err := os.Open(*config)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to read config file %s while checking hash", *config)
	}
	defer util.Close(f)

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", skerr.Wrapf(err, "Failed to copy bytes from %s while checking hash", *config)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func main() {
	ctx := context.Background()
	common.InitWithMust(
		"probeserver",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	var err error
	startHash, err = getHashOfConfigFile()
	if err != nil {
		sklog.Fatal("Failed to calculate hash of config file: ", err)
	}
	cfg, err := readConfigFile(ctx, *config)
	if *validate {
		if err != nil {
			fmt.Printf("Validation Failed:\n  %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Validation Successful")
		os.Exit(0)
	}
	if err != nil {
		sklog.Fatal("Failed to read config file: ", err)
	}

	liveness := metrics2.NewLiveness("probes")

	// Register counters for each probe.
	for name, probe := range cfg {
		for _, u := range probe.URLs {
			probe.Failure[u] = metrics2.GetInt64Metric("prober", map[string]string{"type": "failure", "probename": name, "url": u})
			probe.Latency[u] = metrics2.GetInt64Metric("prober", map[string]string{"type": "latency", "probename": name, "url": u})
		}
	}

	// Create a client that uses our dialer with a timeout.
	anonymousClient := httputils.NewConfiguredTimeoutClient(dialTimeout, requestTimeout)
	anonymousClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatal(err)
	}
	authClient := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	authClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	probeOneRound(cfg, anonymousClient, authClient)
	for range time.Tick(*runEvery) {
		probeOneRound(cfg, anonymousClient, authClient)
		liveness.Reset()

		currentHash, err := getHashOfConfigFile()
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
