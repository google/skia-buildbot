// Functions for the last mile of a fiddle, i.e. formatting the draw.cpp and
// then compiling and executing the code by dispatching a request to a fiddler.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"go.skia.org/infra/fiddlek/go/linenumbers"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// cppPrefix is a format string for the code that makes it compilable.
	cppPrefix = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = %s; // Either a string, or 0.
  return DrawOptions(%d, %d, true, true, %v, %v, %v, %v, %v, path, %s, %d, %d, %d, %s);
}

%s
`
	// numRetries is the number of time to try to find a pod to run the fiddle
	// on before giving up.
	numRetries = 8

	// localrunURL stands in for a fiddler pods URL when running locally.
	defaultLocalRunURL = "http://localhost:8000/run"
)

var (
	runTotal      = metrics2.GetCounter("run_total", nil)
	runFailures   = metrics2.GetCounter("run_failures", nil)
	runExhaustion = metrics2.GetCounter("run_exhaustion", nil)
	podsTotal     = metrics2.GetInt64Metric("pods_total", nil)
	podsIdle      = metrics2.GetInt64Metric("pods_idle", nil)

	alreadyRunningFiddleErr = errors.New("Fiddle already running.")
	failedToSendErr         = errors.New("Failed to send request to fiddler.")
)

func toGrMipMapped(b bool) string {
	if b {
		return "skgpu::Mipmapped::kYes"
	} else {
		return "skgpu::Mipmapped::kNo"
	}
}

type Runner struct {
	sourceDir string
	local     bool
	clientset *kubernetes.Clientset
	rand      *rand.Rand

	mutex        sync.Mutex // mutex protects the members below.
	skiaGitHash  string
	fiddlerIPs   []string
	k8sNamespace string
}

func New(local bool, sourceDir, k8sNamespace string) (*Runner, error) {
	ret := &Runner{
		sourceDir:    sourceDir,
		local:        local,
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		fiddlerIPs:   []string{},
		k8sNamespace: k8sNamespace,
	}
	if !local {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to get in-cluster config")
		}
		sklog.Infof("Auth username: %s", config.Username)
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to get in-cluster clientset")
		}
		ret.clientset = clientset
	}
	return ret, nil
}

// Start loads the fiddler IPs and then starts a goroutine to update the IPs in the background.
func (r *Runner) Start(ctx context.Context) error {
	// Start the IP refresher.
	if err := r.fiddlerIPsOneStep(ctx); err != nil {
		return skerr.Wrapf(err, "Failed initial population of fiddlerIPs")
	}
	go r.fiddlerIPsRefresher(ctx)
	return nil
}

// fiddlerIPsOneStep refreshes a list of fiddler pod IP addresses just once.
func (r *Runner) fiddlerIPsOneStep(ctx context.Context) error {
	var ips []string
	if r.local {
		ips = []string{"127.0.0.1"}
	} else {
		pods, err := r.clientset.CoreV1().Pods(r.k8sNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=fiddler",
		})
		if err != nil {
			return skerr.Wrapf(err, "Could not list fiddler pods")
		}
		ips = make([]string, 0, len(pods.Items))
		for _, p := range pods.Items {
			// Note that the PodIP can be the empty string. Who knew?
			if p.Status.PodIP != "" {
				ips = append(ips, p.Status.PodIP)
			}
		}
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.fiddlerIPs = ips
	return nil
}

// fiddlerIPsRefresher will repeatedly refresh the list of fiddler pod IP addresses
// until the passed in context is cancelled.
func (r *Runner) fiddlerIPsRefresher(ctx context.Context) {
	fiddlerIPLiveness := metrics2.NewLiveness("fiddler_ips")
	util.RepeatCtx(ctx, 5*time.Second, func(ctx context.Context) {
		if err := r.fiddlerIPsOneStep(ctx); err != nil {
			sklog.Warningf("Failed to refresh fiddler IPs: %s", err)
		} else {
			fiddlerIPLiveness.Reset()
		}
	})
}

// prepCodeToCompile adds the line numbers and the right prefix code
// to the fiddle so it compiles and links correctly.
//
//	code - The code to compile.
//	opts - The user's options about how to run that code.
//
// Returns the prepped code.
func (r *Runner) prepCodeToCompile(code string, opts types.Options) string {
	code = linenumbers.LineNumbers(code)
	sourceImage := "0"
	if opts.Source != 0 {
		filename := fmt.Sprintf("%d.png", opts.Source)
		sourceImage = fmt.Sprintf("%q", filepath.Join(r.sourceDir, filename))
	}
	pdf := true
	skp := true
	if opts.Animated {
		pdf = false
		skp = false
	}
	offscreen_mipmap := toGrMipMapped(opts.OffScreenMipMap)
	source_mipmap := toGrMipMapped(opts.SourceMipMap)
	offscreen_width := opts.OffScreenWidth
	if offscreen_width == 0 {
		offscreen_width = 64
	}
	offscreen_height := opts.OffScreenHeight
	if offscreen_height == 0 {
		offscreen_height = 64
	}
	return fmt.Sprintf(cppPrefix, sourceImage, opts.Width, opts.Height, pdf, skp, opts.SRGB, opts.F16, opts.TextOnly, source_mipmap, offscreen_width, offscreen_height, opts.OffScreenSampleCount, offscreen_mipmap, code)
}

// ValidateOptions validates that the options make sense.
func ValidateOptions(opts types.Options) error {
	if opts.Animated {
		if opts.Duration <= 0 {
			return skerr.Fmt("Animation duration must be > 0.")
		}
		if opts.Duration > 60 {
			return skerr.Fmt("Animation duration must be < 60s.")
		}
	} else {
		opts.Duration = 0
	}
	if opts.OffScreen {
		if opts.OffScreenMipMap {
			if opts.OffScreenTexturable == false {
				return skerr.Fmt("OffScreenMipMap can only be true if OffScreenTexturable is true.")
			}
		}
		if opts.OffScreenWidth <= 0 || opts.OffScreenHeight <= 0 {
			return skerr.Fmt("OffScreen Width and Height must be > 0.")
		}
		if opts.OffScreenSampleCount < 0 {
			return skerr.Fmt("OffScreen SampleCount must be >= 0.")
		}
	}
	return nil
}

func (r *Runner) singleRun(ctx context.Context, url string, body io.Reader) (*types.Result, error) {
	ctx, span := trace.StartSpan(ctx, "run.singleRun")
	defer span.End()

	//	client := httputils.NewTimeoutClient()
	client := &http.Client{Transport: &ochttp.Transport{}}
	var output bytes.Buffer

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		sklog.Errorf("Failed to create POST request: %s", err)
		return nil, failedToSendErr
	}

	// Pods come and go, so don't keep the connection alive.
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		sklog.Errorf("Failed to POST to %q: %s", url, err)
		return nil, failedToSendErr
	}
	defer util.Close(resp.Body)
	span.Annotate([]trace.Attribute{
		trace.Int64Attribute("status code", int64(resp.StatusCode)),
	}, "fiddler response")

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, alreadyRunningFiddleErr
	}
	n, err := io.Copy(limitwriter.New(&output, types.MAX_JSON_SIZE), resp.Body)
	if n == types.MAX_JSON_SIZE {
		return nil, skerr.Fmt("Response too large, truncated at %d bytes.", n)
	}
	truncOutput := util.Truncate(output.String(), 20)
	sklog.Infof("Got response: %q", truncOutput)
	if err != nil {
		return nil, skerr.Fmt("Failed to read response: %s", err)
	}
	// Parse the output into types.Result.
	res := &types.Result{}
	if err := json.Unmarshal(output.Bytes(), res); err != nil {
		sklog.Errorf("Received erroneous output: %q", truncOutput)
		return nil, skerr.Fmt("Failed to decode results from run at %q: %s, %q", url, err, truncOutput)
	}
	if strings.HasPrefix(res.Execute.Errors, "Invalid JSON Request") {
		sklog.Errorf("Failed to send valid JSON: res.Execute.Errors : %s", err)
		return nil, failedToSendErr
	}
	return res, nil
}

// Run executes fiddle_run and then parses the JSON output into types.Results.
//
//	local - Boolean, true if we are running locally.
func (r *Runner) Run(ctx context.Context, local bool, req *types.FiddleContext) (*types.Result, error) {
	ctx, span := trace.StartSpan(ctx, "run.Run")
	defer span.End()

	if len(req.Code) > types.MAX_CODE_SIZE {
		return nil, skerr.Fmt("Code size is too large.")
	}
	reqToSend := *req
	reqToSend.Code = r.prepCodeToCompile(req.Code, req.Options)
	runTotal.Inc(1)
	sklog.Infof("%q Sending: %q", req.Hash, reqToSend.Code)

	b, err := json.Marshal(reqToSend)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to encode request")
	}

	// If not local then use the k8s api to pick an open fiddler pod to send
	// the request to. Send a GET / to each one until you find an idle instance.
	if local {
		return r.singleRun(ctx, getLocalURL(ctx), bytes.NewReader(b))
	} else {
		// Try to run the fiddle on an open pod. If all pods are busy then
		// wait a bit and try again.
		for tries := 0; tries < numRetries; tries++ {
			ips := r.randPodIPs()
			for _, p := range ips {
				rootURL := fmt.Sprintf("http://%s:8000", p)
				sklog.Infof("%q Trying: %q", req.Hash, rootURL)
				// Run the fiddle in the open pod.
				ret, err := r.singleRun(ctx, rootURL+"/run", bytes.NewReader(b))
				if err == alreadyRunningFiddleErr || err == failedToSendErr {
					sklog.Warningf("%q Couldn't run on pod: %s", req.Hash, err)
					continue
				} else {
					return ret, skerr.Wrapf(err, "Problem running fiddle %s", req.Hash)
				}
			}
			// Let the pods run and see of any new ones open up.
			time.Sleep((1 << uint64(tries)) * time.Millisecond)
		}
		runExhaustion.Inc(1)
		return nil, skerr.Fmt("%q Failed to find an available server to run the fiddle.", req.Hash)
	}
}

func (r *Runner) podIPs() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return append([]string(nil), r.fiddlerIPs...)
}

// randPodIPs returns 10 random pods IPs.
//
// Only return a subset, not the full list, as this may lead to a thundering
// herd problem. I.e. A large number of incoming requests will hit the backends
// and cause requests to fail, so requests have to look further down the list
// of pods, increasing traffic further.
func (r *Runner) randPodIPs() []string {
	ret := []string{}
	ips := r.podIPs()
	n := len(ips)
	for i := 0; i < 10; i++ {
		ret = append(ret, ips[rand.Intn(n)])
	}
	return ret
}

func singlePodVersion(client *http.Client, address string) (string, types.State, bool) {
	rootURL := fmt.Sprintf("http://%s:8000", address)
	req, err := http.NewRequest("GET", rootURL, nil)
	if err != nil {
		sklog.Infof("Failed to create request for fiddler status: %s", err)
		return "", types.ERROR, false
	}
	// Pods come and go, so don't keep the connection alive.
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		sklog.Infof("Failed to request fiddler status: %s", err)
		return "", types.ERROR, false
	}
	defer util.Close(resp.Body)
	var fiddlerResp types.FiddlerMainResponse
	if err := json.NewDecoder(resp.Body).Decode(&fiddlerResp); err != nil {
		sklog.Warningf("Failed to read status body: %s", err)
		return "", types.ERROR, false
	}
	if fiddlerResp.State == types.IDLE {
		return fiddlerResp.Version, fiddlerResp.State, true
	}
	return "", fiddlerResp.State, false
}

func (r *Runner) metricsSingleStep() {
	idleCount := 0
	// What versions of skia are all the fiddlers running.
	versions := map[string]int{}
	ips := r.podIPs()
	fastClient := httputils.NewFastTimeoutClient()
	for _, st := range types.AllStates {
		metrics2.GetCounter("fiddler_pod_state", map[string]string{"state": string(st)}).Reset()
	}
	for _, address := range ips {
		if ver, st, ok := singlePodVersion(fastClient, address); ok {
			idleCount += 1
			versions[ver] += 1
			metrics2.GetCounter("fiddler_pod_state", map[string]string{"state": string(st)}).Inc(1)
		} else {
			metrics2.GetCounter("fiddler_pod_state", map[string]string{"state": string(st)}).Inc(1)
		}
	}
	// Report the version that appears the most. Usually there will only be one
	// hash, but we might run this in the middle of a fiddler rollout.
	max := 0
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for k, v := range versions {
		if v > max {
			max = v
			r.skiaGitHash = k
		}
	}
	podsIdle.Update(int64(idleCount))
	podsTotal.Update(int64(len(ips)))
}

// Metrics captures metrics on the state of all the fiddler pods.
func (r *Runner) Metrics() {
	metricsLiveness := metrics2.NewLiveness("metrics")
	r.metricsSingleStep()
	for range time.Tick(30 * time.Second) {
		r.metricsSingleStep()
		metricsLiveness.Reset()
	}
}

func (r *Runner) Version() string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.skiaGitHash
}

type contextKeyType string

const localRunKey contextKeyType = "overwriteLocalRunURL"

func withLocalRunURL(ctx context.Context, localURL string) context.Context {
	return context.WithValue(ctx, localRunKey, localURL)
}

func getLocalURL(ctx context.Context) string {
	if u := ctx.Value(localRunKey); u != nil {
		switch v := u.(type) {
		case string:
			return v
		default:
			panic(fmt.Sprintf("Unknown value for localRunKey: %v", v))
		}
	}
	return defaultLocalRunURL
}
