package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	// BUCKET is the Cloud Storage bucket we store files in.
	BUCKET          = "skottie-renderer"
	BUCKET_INTERNAL = "skottie-renderer-internal"

	MAX_FILENAME_SIZE = 5 * 1024
	MAX_JSON_SIZE     = 10 * 1024 * 1024
	MAX_ZIP_SIZE      = 20 * 1024 * 1024
	MAX_ZIP_FILES     = 100
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	lockedDown   = flag.Bool("locked_down", false, "Restricted to only @google.com accounts.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	skottieTool  = flag.String("skottie_tool", "", "[deprecated/unused]Absolute path to the skottie_tool executable.")
	versionFile  = flag.String("version_file", "[deprecated/unused]/etc/skia-prod/VERSION", "The full path of the Skia VERSION file.")
)

var (
	invalidRequestErr = errors.New("")
	canUploadZips     = false
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
		sklog.Fatal(err)
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
		login.InitWithAllow(*port, *local, nil, nil, allow)
	}

	bucket := BUCKET
	if *lockedDown {
		bucket = BUCKET_INTERNAL
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
		filepath.Join(*resourcesDir, "tos.html"),
		filepath.Join(*resourcesDir, "embed.html"),
	))
}
func (srv *Server) templateHandler(filename string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if *local {
			srv.loadTemplates()
		}
		if err := srv.templates.ExecuteTemplate(w, filename, nil); err != nil {
			sklog.Errorf("Failed to expand template %s: %s", filename, err)
		}
	}
}

func (srv *Server) verificationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, err := w.Write([]byte("google-site-verification: google99d1f93c6755806b.html"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to write.")
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
		httputils.ReportError(w, r, err, "Failed to write JSON file.")
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
		httputils.ReportError(w, r, err, "Failed to write binary file.")
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
		httputils.ReportError(w, r, err, "Error decoding JSON.")
		return
	}
	// Check for maliciously sized input on any field we upload to GCS
	if len(req.AssetsFilename) > MAX_ZIP_SIZE || len(req.Filename) > MAX_FILENAME_SIZE {
		httputils.ReportError(w, r, nil, "Input file(s) too big")
		return
	}

	// Calculate md5 of UploadRequest (lottie contents and file name)
	h := md5.New()
	b, err := json.Marshal(req)
	if err != nil {
		httputils.ReportError(w, r, err, "Can't re-encode request.")
		return
	}
	if _, err = h.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed calculating hash.")
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	if strings.HasSuffix(req.Filename, ".json") {
		if err := srv.createFromJSON(&req, hash, ctx); err != nil {
			httputils.ReportError(w, r, err, "Failed handing input of JSON.")
			return
		}
	} else if canUploadZips && strings.HasSuffix(req.Filename, ".zip") {
		if err := srv.createFromZip(&req, hash, ctx); err != nil {
			httputils.ReportError(w, r, err, "Failed handing input of JSON.")
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

func (srv *Server) createFromJSON(req *UploadRequest, hash string, ctx context.Context) error {
	b, err := json.Marshal(req.Lottie)
	if err != nil {
		return skerr.Fmt("Can't re-encode lottie file: %s", err)
	}
	if len(b) > MAX_JSON_SIZE {
		return skerr.Fmt("Lottie JSON is too big (%d bytes): %s", len(b), err)
	}

	if canUploadZips && req.AssetsZip != "" {
		if err := srv.uploadAssetsZip(hash, req.AssetsZip, ctx); err != nil {
			return skerr.Fmt("Could not process asset folder: %s", err)
		}
	}

	// We don't need to store the zip contents or filename.
	req.AssetsZip = ""
	req.AssetsFilename = ""
	return srv.uploadState(req, hash, ctx)
}

func (srv *Server) uploadState(req *UploadRequest, hash string, ctx context.Context) error {
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

func (srv *Server) createFromZip(req *UploadRequest, hash string, ctx context.Context) error {
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

	if jsonFile.UncompressedSize64 > MAX_JSON_SIZE {
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

	if err := srv.uploadState(req, hash, ctx); err != nil {
		return skerr.Fmt("Could not upload lottie.json state: %s", err)
	}

	eg, newCtx := errgroup.WithContext(ctx)
	// Upload everything else as an asset
	for _, f := range zr.File {
		if f != nil {
			pieces := strings.Split(f.Name, "/")
			strippedName := pieces[len(pieces)-1]
			if len(strippedName) < 1 {
				// Ignore directory listing
				continue
			}
			if strings.HasSuffix(strippedName, ".json") {
				// We already uploaded this
				continue
			}
			if !validFileName.MatchString(strippedName) {
				sklog.Infof("Ignoring potentially maliciously-named file %q", f.Name)
				continue
			}
			// Make a local variable to get the file into the closure correctly.
			tf := f
			eg.Go(func() error {
				dest := strings.Join([]string{hash, "assets", strippedName}, "/")
				sklog.Infof("Uploading %s from zip file to %s", strippedName, dest)
				if err := srv.writeZipFileToGCS(tf, dest, "application/octet-stream", newCtx); err != nil {
					return skerr.Fmt("Failed while uploading asset %s to %s: %s", tf.Name, dest, err)
				}
				return nil
			})
		}
	}
	return eg.Wait()
}

func (srv *Server) writeZipFileToGCS(f *zip.File, dest, encoding string, ctx context.Context) error {
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

var topJSONFile = regexp.MustCompile(`^(?P<prefix>.*?)(?P<name>[^/]+\.json)$`)
var validFileName = regexp.MustCompile(`^[A-Za-z0-9\._\-]+$`)

func (srv *Server) uploadAssetsZip(lottieHash, b64Zip string, ctx context.Context) error {
	zr, err := readBase64Zip(b64Zip)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if f != nil {
			if !validFileName.MatchString(f.Name) {
				sklog.Warningf("Saw potentially malicious filename in zip file: %q", f.Name)
				continue
			}
			fr, err := f.Open()
			if err != nil {
				return skerr.Fmt("Could not open zipped file %s: %s", f.Name, err)
			}
			defer util.Close(fr)
			sklog.Infof("See %s in zip file, should upload it", f.Name)
			path := strings.Join([]string{lottieHash, "assets", f.Name}, "/")
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
	if len(b64Zip) > MAX_ZIP_SIZE {
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

	if len(zr.File) > MAX_ZIP_FILES {
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
	r.HandleFunc("/tos", srv.templateHandler("tos.html"))
	r.HandleFunc("/google99d1f93c6755806b.html", srv.verificationHandler)
	r.HandleFunc("/{hash:[0-9A-Za-z]*}", srv.templateHandler("index.html"))
	r.HandleFunc("/e/{hash:[0-9A-Za-z]*}", srv.templateHandler("embed.html"))

	r.HandleFunc("/_/j/{hash:[0-9A-Za-z]+}", srv.jsonHandler).Methods("GET")
	r.HandleFunc(`/_/a/{hash:[0-9A-Za-z]+}/{name:[A-Za-z0-9\._\-]+}`, srv.assetsHandler).Methods("GET")
	r.HandleFunc("/_/upload", srv.uploadHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.CorsHandler(httputils.MakeResourceHandler(*resourcesDir))))).Methods("GET")

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		if *lockedDown {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
		}
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
