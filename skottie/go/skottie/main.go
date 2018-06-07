package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	FPS = 60
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	bucket       = flag.String("bucket", "skottie-render", "The bucket to store lottie files and rendered webp files.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	skottieTool  = flag.String("skottie_tool", "", "Absolute path to the skottie_tool executable.")
)

type Server struct {
	client *storage.Client
	bucket string
}

func New() (*Server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	ts, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token source: %s", err)
	}
	client := auth.ClientFromTokenSource(ts)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}

	return &Server{
		bucket: *bucket,
		client: storageClient,
	}, nil
}

// createWebm runs ffmpeg over the images in the given dir.
func createWebm(ctx context.Context, dir string) error {
	// ffmpeg -r $FPS -pattern_type glob -i '*.png' -c:v libvpx-vp9 -lossless 1 output.webm
	name := "ffmpeg"
	args := []string{
		"-r", fmt.Sprintf("%d", FPS),
		"-pattern_type", "glob", "-i", "*.png",
		"-c:v", "libvpx-vp9",
		"-lossless", "1",
		"lottie.webm",
	}
	output := &bytes.Buffer{}
	runCmd := &exec.Command{
		Name:      name,
		Args:      args,
		Dir:       dir,
		LogStderr: true,
		Stdout:    output,
	}
	if err := exec.Run(ctx, runCmd); err != nil {
		return fmt.Errorf("ffmpeg failed %#v: %s", *runCmd, err)
	}

	return nil
}

func (srv *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Extract json file
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		httputils.ReportError(w, r, err, "Error handling form data.")
		return
	}
	f, _, err := r.FormFile("post_lottie")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't find file.")
		return
	}
	source, err := ioutil.TempDir("", "source")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't create temp space.")
		return
	}
	dest, err := ioutil.TempDir("", "dest")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't create temp space.")
		return
	}
	defer util.RemoveAll(source)
	defer util.RemoveAll(dest)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		httputils.ReportError(w, r, err, "Can't read file.")
		return
	}
	sourceFullPath := filepath.Join(source, "lottie.json")
	if err := ioutil.WriteFile(sourceFullPath, b, 0644); err != nil {
		httputils.ReportError(w, r, err, "Can't write file.")
		return
	}
	// Run through skottie_tool
	toolResults, err := exec.RunSimple(r.Context(), fmt.Sprintf("%s --input %q --writePath %q", *skottieTool, sourceFullPath, dest))
	if err != nil {
		sklog.Warningf("Failed running: %q", toolResults)
		httputils.ReportError(w, r, err, "Failed running skottie_tool.")
		return
	}
	// Run results of that through ffmpeg to create webm.
	if err := createWebm(r.Context(), dest); err != nil {
		httputils.ReportError(w, r, err, "Failed building webm.")
		return
	}
	// Calculate md5 of file+opts.
	// If existing file then redirect to display page.
	// Store json file w/o opts in GCS.
	// Store resulting webm in GCS at md5 (along with skia build hash)
	// Redirect to display page that takes arg of md5 hash.
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"skottie",
		common.PrometheusOpt(promPort),
	)

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/_/upload", srv.uploadHandler)
	r.PathPrefix("/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	// TODO csrf
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = iap.None(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
