package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
)

const (
	// gcsBucket is the Cloud Storage bucket we store files in.
	gcsBucket         = "skottie-renderer"
	gcsBucketInternal = "skottie-renderer-internal"

	// These sizes are in bytes.
	maxFilenameSize = 5 * 1024
	maxJSONSize     = 10 * 1024 * 1024
	maxZipSize      = 200 * 1024 * 1024

	maxZipFiles = 5000
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	lockedDown   = flag.Bool("locked_down", false, "Restricted to only @google.com accounts.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var (
	canUploadZips = false
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

	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		return nil, err
	}

	ts, err := auth.NewDefaultTokenSource(*local, storage.ScopeFullControl)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}

	if *lockedDown {
		allow := allowed.NewAllowedFromList([]string{"google.com"})
		login.SimpleInitWithAllow(*port, *local, nil, nil, allow)
	}

	bucket := gcsBucket
	if *lockedDown {
		bucket = gcsBucketInternal
	}

	srv := &Server{
		bucket: storageClient.Bucket(bucket),
	}
	srv.loadTemplates()
	return srv, nil
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
		filepath.Join(*resourcesDir, "drive.html"),
		filepath.Join(*resourcesDir, "embed.html"),
	))
}
func (srv *Server) templateHandler(filename string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Set the HTML to expire at the same time as the JS and WASM, otherwise the HTML
		// (and by extension, the JS with its cachbuster hash) might outlive the WASM
		// and then the two will skew
		w.Header().Set("Cache-Control", "max-age=60")
		if *local {
			srv.loadTemplates()
		}
		if err := srv.templates.ExecuteTemplate(w, filename, nil); err != nil {
			sklog.Errorf("Failed to expand template %s: %s", filename, err)
		}
	}
}

func resourceHandler(resourcesDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		// Use a shorter cache live to limit the risk of canvaskit.js (in indexbundle.js)
		// from drifting away from the version of canvaskit.wasm. Ideally, the skottie
		// will roll at ToT (~35 commits per day), so living for a minute should
		// reduce the risk of JS/WASM being out of sync.
		w.Header().Add("Cache-Control", "max-age=60")
		fileServer.ServeHTTP(w, r)
	}
}

func (srv *Server) verificationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, err := w.Write([]byte("google-site-verification: google99d1f93c6755806b.html"))
	if err != nil {
		httputils.ReportError(w, err, "Failed to write.", http.StatusInternalServerError)
	}
}

func (srv *Server) jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	hash := mux.Vars(r)["hash"]
	path := strings.Join([]string{hash, "lottie.json"}, "/")
	reader, err := srv.bucket.Object(path).NewReader(r.Context())
	if err != nil {
		sklog.Warningf("Can't load JSON file %s from GCS: %s", path, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err = io.Copy(w, reader); err != nil {
		httputils.ReportError(w, err, "Failed to write JSON file.", http.StatusInternalServerError)
		return
	}
}

// assetsHandler expects a URL as follows:
// [endpoint]/[hash]/[name]
// It then looks in the GCS location:
// gs://[bucket]/[hash]/assets/[name]
func (srv *Server) assetsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	hash := mux.Vars(r)["hash"]
	name := mux.Vars(r)["name"]
	path := strings.Join([]string{hash, "assets", name}, "/")
	reader, err := srv.bucket.Object(path).NewReader(r.Context())
	if err != nil {
		sklog.Warningf("Can't load asset %s from GCS: %s", path, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err = io.Copy(w, reader); err != nil {
		httputils.ReportError(w, err, "Failed to write binary file.", http.StatusInternalServerError)
		return
	}
}

type UploadRequest struct {
	Lottie   interface{} `json:"lottie"` // the parsed JSON
	Filename string      `json:"filename"`
	// AssetsZip is a base64 encoded dataURL of the assets folder
	// or a base64 encoded dataURL of a zip produced by lottiefiles.com.
	// It starts with "data:application/zip;base64,"
	AssetsZip string `json:"assetsZip"`
	// AssetsFilename is the human-friendly filename for the optional
	// assetsZip. It is only used to generate the hash and is stripped
	// out upon storage. We remove the name of the zip because if
	// a user loads the page fresh, they won't have the zip folder
	//  contents and we want to indicate they should
	// re-attach them if they re-upload the animation.
	AssetsFilename string `json:"assetsFilename"`
}

type UploadResponse struct {
	Hash   string      `json:"hash"`
	Lottie interface{} `json:"lottie"` // the parsed JSON
}

func (srv *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract json file.
	defer util.Close(r.Body)
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Error decoding JSON.", http.StatusInternalServerError)
		return
	}
	// Check for maliciously sized input on any field we upload to GCS
	if len(req.AssetsFilename) > maxZipSize || len(req.Filename) > maxFilenameSize {
		httputils.ReportError(w, nil, "Input file(s) too big", http.StatusInternalServerError)
		return
	}

	// Calculate md5 of UploadRequest (lottie contents and file name)
	h := md5.New()
	b, err := json.Marshal(req)
	if err != nil {
		httputils.ReportError(w, err, "Can't re-encode request.", http.StatusInternalServerError)
		return
	}
	if _, err = h.Write(b); err != nil {
		httputils.ReportError(w, err, "Failed calculating hash.", http.StatusInternalServerError)
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	sklog.Infof("Processing input with hash %s", hash)

	if strings.HasSuffix(req.Filename, ".json") {
		if err := srv.createFromJSON(ctx, &req, hash); err != nil {
			httputils.ReportError(w, err, "Failed handing input of JSON.", http.StatusInternalServerError)
			return
		}
	} else if canUploadZips && strings.HasSuffix(req.Filename, ".zip") {
		if err := srv.createFromZip(ctx, &req, hash); err != nil {
			httputils.ReportError(w, err, "Failed handing input of JSON.", http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		msg := "Only .json files allowed"
		if canUploadZips {
			msg = "Only .json and .zip files allowed"
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			sklog.Errorf("Failed to write error response: %s", err)
		}
		return
	}

	resp := UploadResponse{
		Hash:   hash,
		Lottie: req.Lottie,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func (srv *Server) createFromJSON(ctx context.Context, req *UploadRequest, hash string) error {
	b, err := json.Marshal(req.Lottie)
	if err != nil {
		return skerr.Fmt("Can't re-encode lottie file: %s", err)
	}
	if len(b) > maxJSONSize {
		return skerr.Fmt("Lottie JSON is too big (%d bytes): %s", len(b), err)
	}

	if canUploadZips && req.AssetsZip != "" {
		if err := srv.uploadAssetsZip(ctx, hash, req.AssetsZip); err != nil {
			return skerr.Fmt("Could not process asset folder: %s", err)
		}
	}

	// We don't need to store the zip contents or filename.
	req.AssetsZip = ""
	req.AssetsFilename = ""
	return srv.uploadState(ctx, req, hash)
}

func (srv *Server) uploadState(ctx context.Context, req *UploadRequest, hash string) error {
	// Write JSON file, containing the state (filename, lottie, etc)
	bytesToUpload, err := json.Marshal(req)
	if err != nil {
		return skerr.Fmt("Can't re-encode request: %s", err)
	}

	path := strings.Join([]string{hash, "lottie.json"}, "/")
	obj := srv.bucket.Object(path)
	wr := obj.NewWriter(ctx)
	wr.ObjectAttrs.ContentEncoding = "application/json"
	if _, err := wr.Write(bytesToUpload); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS: %s", err)
	}
	if err := wr.Close(); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS on close: %s", err)
	}
	return nil
}

func (srv *Server) createFromZip(ctx context.Context, req *UploadRequest, hash string) error {
	zr, err := readBase64Zip(req.AssetsZip)
	if err != nil {
		return err
	}

	var jsonFile *zip.File

	// Example zip contents from lottiefiles.com looks like:
	// Dogrun/Dogrun.aep
	// Dogrun/dogrun.json
	// Dogrun/images/
	// Dogrun/images/img_0.png
	// ...
	// We seek out the json file and then upload every other file as an
	// asset, throwing away any directory structure:
	// Dogrun/images/img_0.png -> /assets/img_0.png

	for _, f := range zr.File {
		if match := topJSONFile.FindStringSubmatch(f.Name); match != nil {
			// match 1 is prefix, match 2 is filename
			jsonFile = f
			break
		}
	}
	if jsonFile == nil {
		return skerr.Fmt("Could not find json file")
	}

	if jsonFile.UncompressedSize64 > maxJSONSize {
		return skerr.Fmt("Lottie JSON is too big (%d bytes): %s", jsonFile.UncompressedSize64, err)
	}

	fr, err := jsonFile.Open()
	if err != nil {
		return skerr.Fmt("Could not unzip lottie.json: %s", err)
	}

	lottieBytes, err := ioutil.ReadAll(fr)
	if err := json.Unmarshal(lottieBytes, &req.Lottie); err != nil {
		return skerr.Fmt("lottie.json was invalid JSON: %s", err)
	}

	// We don't need to store the zip contents
	req.AssetsZip = ""
	// Remove the name of the folder because if a user loads the page fresh, they
	// won't have the zip folder contents and we want to indicate they should
	// re-attach them if they re-upload the animation.
	req.AssetsFilename = ""

	if err := srv.uploadState(ctx, req, hash); err != nil {
		return skerr.Fmt("Could not upload lottie.json state: %s", err)
	}

	eg, newCtx := errgroup.WithContext(ctx)
	// Upload everything else as an asset
	for _, f := range zr.File {
		if f != nil {
			strippedName := getFileName(f.Name)
			if strippedName == "" || strings.HasSuffix(strippedName, ".json") {
				// We already uploaded this
				continue
			}
			// Make a local variable to get the file into the closure correctly.
			tf := f
			eg.Go(func() error {
				dest := strings.Join([]string{hash, "assets", strippedName}, "/")
				sklog.Infof("Uploading %s from zip file to %s", strippedName, dest)
				if err := srv.writeZipFileToGCS(newCtx, tf, dest, "application/octet-stream"); err != nil {
					return skerr.Fmt("Failed while uploading asset %s to %s: %s", tf.Name, dest, err)
				}
				return nil
			})
		}
	}
	return eg.Wait()
}

func (srv *Server) writeZipFileToGCS(ctx context.Context, f *zip.File, dest, encoding string) error {
	fr, err := f.Open()
	if err != nil {
		return skerr.Fmt("Failed reading out of zip file when uploading %s: %s", dest, err)
	}
	defer util.Close(fr)
	obj := srv.bucket.Object(dest)
	wr := obj.NewWriter(ctx)
	wr.ObjectAttrs.ContentEncoding = encoding
	if _, err := io.Copy(wr, fr); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS: %s", err)
	}
	if err := wr.Close(); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS on close: %s", err)
	}
	return nil
}

// getFileName takes an entry in a zip file and returns the basename
// for example, "images/foo.png" will be translated into "foo.png"
// If the given entry is invalid, empty string is returned.
func getFileName(zipName string) string {
	if strings.HasPrefix(zipName, "__MACOSX") {
		// skip this unhelpful folder
		return ""
	}
	pieces := strings.Split(zipName, "/")
	strippedName := pieces[len(pieces)-1]
	if len(strippedName) < 1 {
		// Ignore directory listing
		return ""
	}

	if !validFileName.MatchString(strippedName) {
		sklog.Infof("Ignoring potentially maliciously-named file %q", zipName)
		return ""
	}
	return strippedName
}

var topJSONFile = regexp.MustCompile(`^(?P<prefix>.*?)(?P<name>[^/]+\.json)$`)
var validFileName = regexp.MustCompile(`^[A-Za-z0-9._\-]+$`)

func (srv *Server) uploadAssetsZip(ctx context.Context, lottieHash, b64Zip string) error {
	zr, err := readBase64Zip(b64Zip)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if f != nil {
			strippedName := getFileName(f.Name)
			if strippedName == "" {
				// Skip invalid file
				continue
			}
			fr, err := f.Open()
			if err != nil {
				return skerr.Fmt("Could not open zipped file %s: %s", f.Name, err)
			}
			defer util.Close(fr)
			sklog.Infof("See %s [%s] in zip file, should upload it", strippedName, f.Name)
			path := strings.Join([]string{lottieHash, "assets", strippedName}, "/")
			obj := srv.bucket.Object(path)
			wr := obj.NewWriter(ctx)

			wr.ObjectAttrs.ContentEncoding = "application/octet-stream"
			if _, err := io.Copy(wr, fr); err != nil {
				return skerr.Fmt("Could not write %s to GCS %s: %s", f.Name, path, err)
			}
			if err := wr.Close(); err != nil {
				return skerr.Fmt("Could not write %s to GCS on close %s: %s", f.Name, path, err)
			}
		}
	}
	return nil
}

const BASE64_ZIP_PREFIX = "data:application/zip;base64,"

func readBase64Zip(b64Zip string) (*zip.Reader, error) {
	if strings.HasPrefix(b64Zip, BASE64_ZIP_PREFIX) {
		b64Zip = strings.TrimPrefix(b64Zip, BASE64_ZIP_PREFIX)
	} else {
		return nil, skerr.Fmt("Not a base64 encoded zip")
	}
	if len(b64Zip) > maxZipSize {
		return nil, skerr.Fmt(".zip too big (%d bytes)", len(b64Zip))
	}
	data, err := base64.StdEncoding.DecodeString(b64Zip)
	if err != nil {
		return nil, skerr.Fmt("Could not decode base64 string %s", err)
	}

	zb := bytes.NewReader(data)

	zr, err := zip.NewReader(zb, int64(len(data)))
	if err != nil {
		return nil, skerr.Fmt("Could not unzip bytes %s", err)
	}

	if len(zr.File) > maxZipFiles {
		return nil, skerr.Fmt(".zip has too many files (%d)", len(zr.File))
	}
	return zr, nil
}

func main() {
	common.InitWithMust(
		"skottie",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	if *lockedDown && *local {
		sklog.Fatalf("Can't be run as both --locked_down and --local.")
	}
	canUploadZips = *lockedDown || *local

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/drive", srv.templateHandler("drive.html"))
	r.HandleFunc("/google99d1f93c6755806b.html", srv.verificationHandler)
	r.HandleFunc("/{hash:[0-9A-Za-z]*}", srv.templateHandler("index.html")).Methods("GET")
	r.HandleFunc("/e/{hash:[0-9A-Za-z]*}", srv.templateHandler("embed.html")).Methods("GET")

	r.HandleFunc("/_/j/{hash:[0-9A-Za-z]+}", srv.jsonHandler).Methods("GET")
	r.HandleFunc(`/_/a/{hash:[0-9A-Za-z]+}/{name:[A-Za-z0-9\._\-]+}`, srv.assetsHandler).Methods("GET")
	r.HandleFunc("/_/upload", srv.uploadHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.CorsHandler(resourceHandler(*resourcesDir))))).Methods("GET")

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.CrossOriginResourcePolicy(h)
	if !*local {
		if *lockedDown {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
		}
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
