package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	// FPS is the frame rate to play and encode animations.
	FPS = 30

	// BUCKET is the Cloud Storage bucket we store files in.
	BUCKET = "skottie-renderer"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	skottieTool  = flag.String("skottie_tool", "", "Absolute path to the skottie_tool executable.")
)

// Server is the state of the server.
type Server struct {
	bucket    *storage.BucketHandle
	templates *template.Template
}

func New() (*Server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	ts, err := auth.NewDefaultTokenSource(*local, storage.ScopeFullControl)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token source: %s", err)
	}
	client := auth.ClientFromTokenSource(ts)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}

	srv := &Server{
		bucket: storageClient.Bucket(BUCKET),
	}
	srv.loadTemplates()
	return srv, nil
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
		filepath.Join(*resourcesDir, "display.html"),
	))
}

// createWebm runs ffmpeg over the images in the given dir.
func createWebm(ctx context.Context, dir string) error {
	// ffmpeg -r $FPS -pattern_type glob -i '*.png' -c:v libvpx-vp9 -lossless 1 lottie.webm
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
func (srv *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *Server) displayHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		srv.loadTemplates()
	}
	vars := mux.Vars(r)
	context := map[string]string{
		"Hash": vars["hash"],
	}
	if err := srv.templates.ExecuteTemplate(w, "display.html", context); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *Server) webmHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "video/webm")
	hash := mux.Vars(r)["hash"]
	path := strings.Join([]string{hash, "lottie.webm"}, "/")
	reader, err := srv.bucket.Object(path).NewReader(r.Context())
	if err != nil {
		httputils.ReportError(w, r, err, "Can't load file from GCS")
		return
	}
	if _, err = io.Copy(w, reader); err != nil {
		httputils.ReportError(w, r, err, "Failed to write webm file.")
		return
	}
}

func (srv *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract json file.
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		httputils.ReportError(w, r, err, "Error handling form data.")
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't find file.")
		return
	}
	source, err := ioutil.TempDir("", "source")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't create temp space.")
		return
	}
	defer util.RemoveAll(source)
	dest, err := ioutil.TempDir("", "dest")
	if err != nil {
		httputils.ReportError(w, r, err, "Can't create temp space.")
		return
	}
	defer util.RemoveAll(dest)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		httputils.ReportError(w, r, err, "Can't read file.")
		return
	}
	sourceFullPath := filepath.Join(source, "lottie.json")
	destFullPath := filepath.Join(dest, "lottie.webm")
	if err := ioutil.WriteFile(sourceFullPath, b, 0644); err != nil {
		httputils.ReportError(w, r, err, "Can't write file.")
		return
	}
	// Run through skottie_tool.
	toolResults, err := exec.RunSimple(ctx, fmt.Sprintf("%s --input %s --writePath %s --fps %d", *skottieTool, sourceFullPath, dest, FPS))
	if err != nil {
		sklog.Warningf("Failed running: %q", toolResults)
		httputils.ReportError(w, r, err, "Failed running skottie_tool.")
		return
	}
	// Run results of that through ffmpeg to create webm.
	if err := createWebm(ctx, dest); err != nil {
		httputils.ReportError(w, r, err, "Failed building webm.")
		return
	}

	// Calculate md5 of file.
	// TODO(jcgregorio) include options in md5 calculation once they're added to the UI.
	h := md5.New()
	if _, err = h.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed calculating hash.")
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	// Write JSON file.
	path := strings.Join([]string{hash, "lottie.json"}, "/")
	wr := srv.bucket.Object(path).NewWriter(ctx)
	defer util.Close(wr)
	wr.ObjectAttrs.ContentEncoding = "application/json"
	if _, err := wr.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed writing JSON to GCS.")
		return
	}

	// Write webm file.
	path = strings.Join([]string{hash, "lottie.webm"}, "/")
	wr = srv.bucket.Object(path).NewWriter(ctx)
	defer util.Close(wr)
	wr.ObjectAttrs.ContentEncoding = "video/webm"
	f, err = os.Open(destFullPath)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed opening webm.")
		return
	}
	_, err = io.Copy(wr, f)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed writing webm to GCS.")
		return
	}

	// Redirect to display page that takes arg of md5 hash.
	http.Redirect(w, r, fmt.Sprintf("/display/%s", hash), http.StatusTemporaryRedirect)
}

func main() {
	common.InitWithMust(
		"skottie",
		common.PrometheusOpt(promPort),
	)

	if *skottieTool == "" {
		sklog.Fatal("The --skottie_tool flag is required.")
	}

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/display/{hash:[0-9A-Za-z]+}", srv.displayHandler)

	r.HandleFunc("/_/i/{hash:[0-9A-Za-z]+}", srv.webmHandler)
	r.HandleFunc("/_/upload", srv.uploadHandler)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = iap.None(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
