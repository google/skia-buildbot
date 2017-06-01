// Compiles a fiddle and then runs the fiddle. The output of both processes is
// combined into a single JSON output.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/skia-dev/glog"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/fiddle/go/config"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

const (
	// FPS is the Frames Per Second when generating an animation.
	FPS = 60
)

// flags
var (
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	fiddleRoot = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	checkout   = flag.String("checkout", "", "Directory where Skia is checked out.")
	gitHash    = flag.String("git_hash", "", "The version of Skia code to run against.")
	duration   = flag.Float64("duration", 0.0, "If an animation, the duration of the animation. 0 for no animation.")
)

func serializeOutput(res *types.Result) {
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(res); err != nil {
		fmt.Printf("Failed to encode: %s", err)
	}
}

func main() {
	common.Init()
	res := &types.Result{
		Compile: types.Compile{},
		Execute: types.Execute{
			Errors: "",
			Output: types.Output{},
		},
	}
	if *fiddleRoot == "" {
		res.Errors = "fiddle_run: The --fiddle_root flag is required."
	}
	if *gitHash == "" {
		res.Errors = "fiddle_run: The --git_hash flag is required."
	}
	if *checkout == "" {
		res.Errors = "fiddle_run: The --checkout flag is required."
	}

	depotTools := filepath.Join(*fiddleRoot, "depot_tools")

	// Set limits on this process and all its children.

	// Limit total CPU seconds.
	rLimit := &syscall.Rlimit{
		Cur: 30,
		Max: 30,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, rLimit); err != nil {
		fmt.Println("Error Setting Rlimit ", err)
	}
	// Do not emit core dumps.
	rLimit = &syscall.Rlimit{
		Cur: 0,
		Max: 0,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, rLimit); err != nil {
		fmt.Println("Error Setting Rlimit ", err)
	}

	// Re-run GN since the directory changes under overlayfs.
	if err := buildskia.GNGen(*checkout, depotTools, "Release", config.GN_FLAGS); err != nil {
		glog.Errorf("gn gen failed: %s", err)
		res.Compile.Errors = err.Error()
		serializeOutput(res)
		return
	}

	// Compile draw.cpp into 'fiddle'.
	if output, err := buildskia.GNNinjaBuild(*checkout, depotTools, "Release", "fiddle", true); err != nil {
		res.Compile.Errors = err.Error()
		res.Compile.Output = output
		serializeOutput(res)
		return
	}

	if *duration == 0 {
		oneStep(*checkout, res, 0.0)
		serializeOutput(res)
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
		numFrames := int(FPS * (*duration))
		tmpDir, err := ioutil.TempDir("", "animation")
		if err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to create tmp dir for storing animation PNGs: %s", err)
			serializeOutput(res)
			return
		}

		// frameCh is a channel of all the frames we want rendered, which feeds the
		// pool of Go routines that will do the actual rendering.
		frameCh := make(chan int)

		// Start a pool of workers to do the rendering.
		for i := 0; i <= runtime.NumCPU(); i++ {
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
					oneStep(*checkout, res, frame)
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
			serializeOutput(res)
			return
		}
		// Run ffmpeg for CPU and GPU.
		if err := createWebm("CPU", tmpDir); err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to encode video: %s", err)
			serializeOutput(res)
			return
		}
		res.Execute.Output.AnimatedRaster = encodeWebm("CPU", tmpDir, res)

		if err := createWebm("GPU", tmpDir); err != nil {
			res.Execute.Errors = fmt.Sprintf("Failed to encode video: %s", err)
			serializeOutput(res)
			return
		}
		res.Execute.Output.AnimatedGpu = encodeWebm("GPU", tmpDir, res)
		res.Execute.Output.Raster = ""
		res.Execute.Output.Gpu = ""

		serializeOutput(res)
	}
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

// createWebm runs ffmpeg over the images in the given dir.
func createWebm(prefix, tmpDir string) error {
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
		Name:      name,
		Args:      args,
		Dir:       tmpDir,
		LogStderr: true,
		Stdout:    output,
	}
	if err := exec.Run(runCmd); err != nil {
		return fmt.Errorf("ffmpeg failed %#v: %s", *runCmd, err)
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

// Now that we've built fiddle we want to run it as:
//
//    $ bin/fiddle_secwrap out/fiddle
//
// in the container, or as
//
//    $ out/Release/fiddle
//
// if running locally.
func oneStep(checkout string, res *types.Result, frame float64) {
	name := path.Join(*fiddleRoot, "bin", "fiddle_secwrap")
	args := []string{path.Join(checkout, "skia", "out", "Release", "fiddle")}
	if *local {
		name = path.Join(checkout, "skia", "out", "Release", "fiddle")
		args = []string{}
	}
	args = append(args, "--duration", fmt.Sprintf("%f", *duration), "--frame", fmt.Sprintf("%f", frame))

	stderr := bytes.Buffer{}
	stdout := bytes.Buffer{}
	runCmd := &exec.Command{
		Name:        name,
		Args:        args,
		Dir:         *fiddleRoot,
		InheritPath: true,
		Env:         []string{"LD_LIBRARY_PATH=" + config.EGL_LIB_PATH},
		InheritEnv:  true,
		Stdout:      &stdout,
		Stderr:      &stderr,
		Timeout:     20 * time.Second,
	}
	if err := exec.Run(runCmd); err != nil {
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
