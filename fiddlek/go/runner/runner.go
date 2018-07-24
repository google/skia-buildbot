// Functions for the last mile of a fiddle, i.e. formatting the draw.cpp and
// then compiling and executing the code by dispatching a request to a fiddler.
package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.skia.org/infra/fiddlek/go/linenumbers"
	"go.skia.org/infra/fiddlek/go/types"
)

const (
	// PREFIX is a format string for the code that makes it compilable.
	PREFIX = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = %s; // Either a string, or 0.
  return DrawOptions(%d, %d, true, true, %v, %v, %v, %v, %v, path, %s, %d, %d, %d, %s);
}

%s
`
)

var (
	LOCALRUN_URL = "http://localhost:8000/run"
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
		return "GrMipMapped::kYes"
	} else {
		return "GrMipMapped::kNo"
	}
}

type Runner struct {
	sourceDir  string
	client     *http.Client
	fastClient *http.Client
	local      bool
	clientset  *kubernetes.Clientset
	rand       *rand.Rand

	mutex       sync.Mutex // mutex protects skiaGitHash.
	skiaGitHash string
}

func New(local bool, sourceDir string) (*Runner, error) {
	ret := &Runner{
		sourceDir:  sourceDir,
		client:     httputils.NewTimeoutClient(),
		fastClient: httputils.NewFastTimeoutClient(),
		local:      local,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if !local {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to get in-cluster config: %s", err)
		}
		sklog.Infof("Auth username: %s", config.Username)
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("Failed to get in-cluster clientset: %s", err)
		}
		ret.clientset = clientset
	}
	return ret, nil
}

// prepCodeToCompile adds the line numbers and the right prefix code
// to the fiddle so it compiles and links correctly.
//
//    code - The code to compile.
//    opts - The user's options about how to run that code.
//
// Returns the prepped code.
func (r *Runner) prepCodeToCompile(code string, opts *types.Options) string {
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
	return fmt.Sprintf(PREFIX, sourceImage, opts.Width, opts.Height, pdf, skp, opts.SRGB, opts.F16, opts.TextOnly, source_mipmap, offscreen_width, offscreen_height, opts.OffScreenSampleCount, offscreen_mipmap, code)
}

// ValidateOptions validates that the options make sense.
func (r *Runner) ValidateOptions(opts *types.Options) error {
	if opts.Animated {
		if opts.Duration <= 0 {
			return fmt.Errorf("Animation duration must be > 0.")
		}
		if opts.Duration > 60 {
			return fmt.Errorf("Animation duration must be < 60s.")
		}
	} else {
		opts.Duration = 0
	}
	if opts.OffScreen {
		if opts.OffScreenMipMap {
			if opts.OffScreenTexturable == false {
				return fmt.Errorf("OffScreenMipMap can only be true if OffScreenTexturable is true.")
			}
		}
		if opts.OffScreenWidth <= 0 || opts.OffScreenHeight <= 0 {
			return fmt.Errorf("OffScreen Width and Height must be > 0.")
		}
		if opts.OffScreenSampleCount < 0 {
			return fmt.Errorf("OffScreen SampleCount must be >= 0.")
		}
	}
	return nil
}

func (r *Runner) singleRun(url string, body io.Reader) (*types.Result, error) {
	var output bytes.Buffer
	resp, err := r.client.Post(url, "application/json", body)
	if err != nil {
		return nil, failedToSendErr
	}
	defer util.Close(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, alreadyRunningFiddleErr
	}
	_, err = io.Copy(limitwriter.New(&output, types.MAX_JSON_SIZE), resp.Body)
	sklog.Infof("Got response: %q", output.String()[:20])
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %s", err)
	}
	// Parse the output into types.Result.
	res := &types.Result{}
	if err := json.Unmarshal(output.Bytes(), res); err != nil {
		sklog.Errorf("Received erroneous output: %q", output.String()[:20])
		return nil, fmt.Errorf("Failed to decode results from run: %s, %q", err, output.String())
	}
	if strings.HasPrefix(res.Execute.Errors, "Invalid JSON Request") {
		return nil, failedToSendErr
	}
	return res, nil
}

// shuffle is copied from the Go 1.10 library until we can switch to Go 1.10.
func shuffle(r *rand.Rand, n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

	// Fisher-Yates shuffle: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	// Shuffle really ought not be called with n that doesn't fit in 32 bits.
	// Not only will it take a very long time, but with 2³¹! possible permutations,
	// there's no way that any PRNG can have a big enough internal state to
	// generate even a minuscule percentage of the possible permutations.
	// Nevertheless, the right API signature accepts an int n, so handle it as best we can.
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(r.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(r.Int31n(int32(i + 1)))
		swap(i, j)
	}
}

// Run executes fiddle_run and then parses the JSON output into types.Results.
//
//    local - Boolean, true if we are running locally.
func (r *Runner) Run(local bool, req *types.FiddleContext) (*types.Result, error) {
	if len(req.Code) > types.MAX_CODE_SIZE {
		return nil, fmt.Errorf("Code size is too large.")
	}
	reqToSend := *req
	reqToSend.Code = r.prepCodeToCompile(req.Code, &req.Options)
	runTotal.Inc(1)
	sklog.Infof("Sending: %q", reqToSend.Code)

	b, err := json.Marshal(reqToSend)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %s", err)
	}
	body := bytes.NewReader(b)

	// If not local then use the k8s api to pick an open fiddler pod to send
	// the request to. Send a GET / to each one until you find an idle instance.
	if local {
		return r.singleRun(LOCALRUN_URL, body)
	} else {
		// Try to run the fiddle on an open pod. If all pods are busy then
		// wait a bit and try again.
		for tries := 0; tries < 6; tries++ {
			fiddlerList, err := r.clientset.CoreV1().Pods("default").List(metav1.ListOptions{
				LabelSelector: "app=fiddler",
			})
			if err != nil {
				return nil, fmt.Errorf("Could not list fiddler pods: %s", err)
			}
			pods := fiddlerList.Items
			// The kubernetes API returns the list of pods in sorted order. Shuffle
			// them in order to avoid pummeling the first pod with tons of traffic.
			shuffle(r.rand, len(pods), func(i, j int) {
				pods[i], pods[j] = pods[j], pods[i]
			})
			// Loop over all the pods looking for an open one.
			for i, p := range pods {
				sklog.Infof("Found pod %d: %s", i, p.Name)
				rootURL := fmt.Sprintf("http://%s:8000", p.Status.PodIP)
				resp, err := r.fastClient.Get(rootURL)
				if err != nil {
					sklog.Infof("Failed to request fiddler status: %s", err)
					continue
				}
				defer util.Close(resp.Body)

				var fiddlerResp types.FiddlerMainResponse
				if err := json.NewDecoder(resp.Body).Decode(&fiddlerResp); err != nil {
					sklog.Warningf("Failed to read status: %s", err)
					continue
				}
				if fiddlerResp.State == types.IDLE {
					// Run the fiddle in the open pod.
					ret, err := r.singleRun(rootURL+"/run", body)
					if err == alreadyRunningFiddleErr || err == failedToSendErr {
						continue
					} else {
						return ret, err
					}
				}
			}
			// Let the pods run and see of any new ones open up.
			time.Sleep((1 << uint64(tries)) * time.Second)
		}
		runExhaustion.Inc(1)
		return nil, fmt.Errorf("Failed to find an available server to run the fiddle.")
	}
}

func (r *Runner) podIPs() []string {
	if r.local {
		return []string{"127.0.0.1"}
	} else {
		pods, err := r.clientset.CoreV1().Pods("default").List(metav1.ListOptions{
			LabelSelector: "app=fiddler",
		})
		if err != nil {
			sklog.Errorf("Could not list fiddler pods: %s", err)
			return []string{}
		}
		ret := make([]string, 0, len(pods.Items))
		for _, p := range pods.Items {
			ret = append(ret, p.Status.PodIP)
		}
		return ret
	}
}

func (r *Runner) metricsSingleStep() {
	idleCount := 0
	// What versions of skia are all the fiddlers running.
	versions := map[string]int{}
	ips := r.podIPs()
	for _, address := range ips {
		rootURL := fmt.Sprintf("http://%s:8000", address)
		resp, err := r.client.Get(rootURL)
		if err != nil {
			sklog.Infof("Failed to request fiddler status: %s", err)
			continue
		}
		defer util.Close(resp.Body)
		if resp.Header.Get("Content-Type") == "application/json" {
			var fiddlerResp types.FiddlerMainResponse
			if err := json.NewDecoder(resp.Body).Decode(&fiddlerResp); err != nil {
				sklog.Warningf("Failed to read status: %s", err)
				continue
			}
			if fiddlerResp.State == types.IDLE {
				idleCount += 1
			}
			versions[fiddlerResp.Version] += 1
		} else {
			state, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				sklog.Warningf("Failed to read status: %s", err)
				continue
			}
			if types.State(state) == types.IDLE {
				idleCount += 1
			}
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
	r.metricsSingleStep()
	for _ = range time.Tick(15 * time.Second) {
		r.metricsSingleStep()
	}
}

func (r *Runner) Version() string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.skiaGitHash
}
