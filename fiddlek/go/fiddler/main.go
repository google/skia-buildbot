// Compiles a fiddle and then runs the fiddle. The output of both processes is
// combined into a single JSON output.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
	"golang.org/x/sync/errgroup"
)

const (
	// FPS is the Frames Per Second when generating an animation.
	FPS = 60
)

// flags
var (
	apoptosis  = flag.Duration("apoptosis", 5*time.Minute, "How long a pod should live after starting a run.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	fiddleRoot = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	checkout   = flag.String("checkout", "", "Directory where Skia is checked out.")
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
)

var (
	mutex        sync.Mutex
	currentState types.State = types.IDLE
	version      string
)

func setStateStart() error {
	mutex.Lock()
	defer mutex.Unlock()
	if currentState != types.IDLE {
		return fmt.Errorf("Fiddle already being run.")
	}
	currentState = types.WRITING
	return nil
}

func setState(s types.State) {
	mutex.Lock()
	defer mutex.Unlock()
	currentState = s
}

func getState() types.State {
	mutex.Lock()
	defer mutex.Unlock()
	return currentState
}

func serializeOutput(w io.Writer, res *types.Result) {
	w = limitwriter.New(w, types.MAX_JSON_SIZE)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		sklog.Errorf("Failed to encode: %s", err)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := &types.FiddlerMainResponse{
		State:   getState(),
		Version: version,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Warningf("Failed to write response: %s", err)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer util.Close(r.Body)
	if setStateStart() != nil {
		http.Error(w, "Currently running a fiddle.", http.StatusTooManyRequests)
		return
	}
	go func() {
		// Fail out of the container after a single run.
		<-time.Tick(*apoptosis)
		os.Exit(0)
	}()
	var request types.FiddleContext

	res := &types.Result{
		Compile: types.Compile{},
		Execute: types.Execute{
			Errors: "",
			Output: types.Output{},
		},
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		res.Execute.Errors = fmt.Sprintf("Invalid JSON Request: %s", err)
		serializeOutput(w, res)
		return
	}

	// Compile draw.cpp into 'fiddle'.
	if err := ioutil.WriteFile(filepath.Join(*checkout, "tools", "fiddle", "draw.cpp"), []byte(request.Code), 0644); err != nil {
		res.Execute.Errors = fmt.Sprintf("Failed to write draw.cpp: %s", err)
		serializeOutput(w, res)
		return
	}

	setState(types.COMPILING)

	// TODO(jcgregorio) Put a timeout here.
	buildResults, err := exec.RunCwd(ctx, *checkout, filepath.Join(*fiddleRoot, "depot_tools", "ninja"), "-C", "out/Static")
	if err != nil {
		res.Compile.Errors = err.Error()
		res.Compile.Output = buildResults
		serializeOutput(w, res)
		return
	}

	// We stay in the RUNNING state forever.
	setState(types.RUNNING)

	if request.Options.Duration == 0 {
		oneStep(ctx, *checkout, res, 0.0, request.Options.Duration)
		serializeOutput(w, res)
	} else {

		var g errgroup.Group
		// If this is an animated fiddle then:
		//   - Create tmpdir to store PNGs.
		//   - Loop over the following code to generate each frame of the animation.
		//   -   Pass the duration and frame via cmd line flags to fiddle.
		//   -   Decode and write PNGs (CPU+GPU) to their temp location.
		//   - Run ffmpeg over the resulting PNG's to generate the webm files.
		//   - Clean up tmp file.
		//   - Encode resulting webm files as base64 strings and return in JSON.
		numFrames := int(FPS * (request.Options.Duration))
		tmpDir, err := ioutil.TempDir("", "animation")
		defer util.RemoveAll(tmpDir)
		if err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to create tmp dir for storing animation PNGs: %s", err)
			serializeOutput(w, res)
			return
		}

		// frameCh is a channel of all the frames we want rendered, which feeds the
		// pool of Go routines that will do the actual rendering.
		frameCh := make(chan int)

		// mutex protects glinfo.
		var mutex sync.Mutex
		// glinfo is the info about the GL version for the GPU.
		glinfo := ""

		// Start a pool of workers to do the rendering.
		for i := 0; i <= 5; i++ {
			g.Go(func() error {
				// Each Go func should have its own 'res'.
				res := &types.Result{
					Compile: types.Compile{},
					Execute: types.Execute{
						Errors: "",
						Output: types.Output{},
					},
				}
				for frameIndex := range frameCh {
					sklog.Infof("Parallel render: %d", frameIndex)
					frame := float64(frameIndex) / float64(numFrames)
					oneStep(ctx, *checkout, res, frame, request.Options.Duration)
					// Check for errors.
					if res.Execute.Errors != "" {
						return fmt.Errorf("Failed to render: %s", res.Execute.Errors)
					}
					// Extract CPU and GPU pngs to a tmp directory.
					if err := extractPNG(res.Execute.Output.Raster, res, frameIndex, "CPU", tmpDir); err != nil {
						return err
					}
					if err := extractPNG(res.Execute.Output.Gpu, res, frameIndex, "GPU", tmpDir); err != nil {
						return err
					}
					mutex.Lock()
					if glinfo == "" {
						glinfo = res.Execute.Output.GLInfo
					}
					mutex.Unlock()
				}
				return nil
			})
		}

		// Feed all the frame indices to the channel.
		for i := 0; i <= numFrames; i++ {
			frameCh <- i
		}
		close(frameCh)

		// Wait for all the work to be done.
		if err := g.Wait(); err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to encode video: %s", err)
			serializeOutput(w, res)
			return
		}
		// Run ffmpeg for CPU and GPU.
		if err := createWebm(ctx, "CPU", tmpDir); err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to encode video: %s", err)
			serializeOutput(w, res)
			return
		}
		res.Execute.Output.AnimatedRaster = encodeWebm("CPU", tmpDir, res)

		if err := createWebm(ctx, "GPU", tmpDir); err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to encode video: %s", err)
			serializeOutput(w, res)
			return
		}
		res.Execute.Output.AnimatedGpu = encodeWebm("GPU", tmpDir, res)
		res.Execute.Output.Raster = ""
		res.Execute.Output.Gpu = ""
		res.Execute.Output.GLInfo = glinfo

		serializeOutput(w, res)
	}

	go func() {
		// Fail out of the container after the results have been sent.
		<-time.Tick(3 * time.Second)
		os.Exit(0)
	}()
}

// encodeWebm encodes the webm as base64 and adds it to the results.
func encodeWebm(prefix, tmpDir string, res *types.Result) string {
	b, err := ioutil.ReadFile(path.Join(tmpDir, fmt.Sprintf("%s.webm", prefix)))
	if err != nil {
		res.Execute.Errors = fmt.Sprintf("Failed to read resulting video: %s", err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func trunc(s string) string {
	if len(s) < 100 {
		return s
	}
	return s[:100] + "..."
}

// createWebm runs ffmpeg over the images in the given dir.
func createWebm(ctx context.Context, prefix, tmpDir string) error {
	// ffmpeg -r $FPS -pattern_type glob -i '*.png' -c:v libvpx-vp9 -lossless 1 output.webm
	name := "ffmpeg"
	args := []string{
		"-r", fmt.Sprintf("%d", FPS),
		"-pattern_type", "glob", "-i", prefix + "*.png",
		"-c:v", "libvpx-vp9",
		"-lossless", "1",
		fmt.Sprintf("%s.webm", prefix),
	}
	output := &bytes.Buffer{}
	runCmd := &exec.Command{
		Name:           name,
		Args:           args,
		Dir:            tmpDir,
		CombinedOutput: output,
	}
	if err := exec.Run(ctx, runCmd); err != nil {
		return fmt.Errorf("ffmpeg failed %#v %q: %s", *runCmd, trunc(output.String()), err)
	}

	return nil
}

// extractPNG pulls the base64 encoded PNG out of the results and writes it to the tmpDir.
func extractPNG(b64 string, res *types.Result, i int, prefix string, tmpDir string) error {
	body, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		res.Execute.Errors = fmt.Sprintf("Failed to decode frame %d of %s: %s", i, prefix, err)
		return err
	}
	if err := ioutil.WriteFile(path.Join(tmpDir, fmt.Sprintf("%s_%05d.png", prefix, i)), body, 0600); err != nil {
		res.Execute.Errors = fmt.Sprintf("Failed to write frame %d of %s as a PNG: %s", i, prefix, err)
		return err
	}
	return nil
}

func oneStep(ctx context.Context, checkout string, res *types.Result, frame float64, duration float64) {
	name := path.Join(checkout, "out", "Static", "fiddle")
	args := []string{"--duration", fmt.Sprintf("%f", duration), "--frame", fmt.Sprintf("%f", frame)}
	stderr := bytes.Buffer{}
	stdout := bytes.Buffer{}
	runCmd := &exec.Command{
		Name:        name,
		Args:        args,
		Dir:         *fiddleRoot,
		InheritPath: true,
		Env:         []string{"HOME=/tmp"},
		InheritEnv:  true,
		Stdout:      &stdout,
		Stderr:      &stderr,
		Timeout:     20 * time.Second,
	}
	if err := exec.Run(ctx, runCmd); err != nil {
		sklog.Errorf("Failed to run: %s", err)
		res.Execute.Errors = err.Error()
	}
	if res.Execute.Errors != "" && stderr.String() != "" {
		sklog.Errorf("Found stderr output: %q", stderr.String())
		res.Execute.Errors += "\n"
	}
	res.Execute.Errors += stderr.String()
	if err := json.Unmarshal(stdout.Bytes(), &res.Execute.Output); err != nil {
		if res.Execute.Errors != "" {
			res.Execute.Errors += "\n"
		}
		res.Execute.Errors += "Failed to decode JSON output from fiddle.\n"
		res.Execute.Errors += err.Error()
		res.Execute.Errors += fmt.Sprintf("\nOutput was %q", stdout.Bytes())
	}
}

func main() {
	common.InitWithMust(
		"fiddler",
	)
	if *fiddleRoot == "" {
		sklog.Fatalf("The --fiddle_root flag is required.")
	}
	if *checkout == "" {
		sklog.Fatalf("The --checkout flag is required.")
	}

	b, err := ioutil.ReadFile(filepath.Join(*checkout, "VERSION"))
	if err != nil {
		sklog.Fatalf("Failed to read Skia version: %s", err)
	}
	version = strings.TrimSpace(string(b))

	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/run", runHandler)

	sklog.Infoln("Ready to serve.")
	srv := &http.Server{
		Handler:      httputils.LoggingGzipRequestResponse(r),
		Addr:         *port,
		WriteTimeout: 120 * time.Second,
		ReadTimeout:  120 * time.Second,
	}

	sklog.Fatal(srv.ListenAndServe())
}
