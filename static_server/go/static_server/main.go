// This executable is a very simple web server that serves static HTML/CSS/JS content from GCS.
// The GCS content can be created via another process, e.g. from a CI job. There is a file that
// controls which directory should be the root of the served content, called the latest_file.
// Whatever process updates new content to GCS should update the latest file after the content
// is updated.
//
// THIS SERVER DOES NO AUTHENTICATION. If the content being served is sensitive, then it should
// be only used on an internal website.
package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	serverReadTimeout  = 5 * time.Minute
	serverWriteTimeout = 5 * time.Minute
)

func main() {
	var (
		gcsBucket   = flag.String("gcs_bucket", "", "The GCS bucket from which to serve static content")
		latestFile  = flag.String("latest_file", "", "The path to a file in the gcs bucket that points to the latest content to serve")
		refreshRate = flag.Duration("refresh_rate", time.Minute, "How often to re-poll and update latest_file")
		port        = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
		promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	)
	flag.Parse()

	if *gcsBucket == "" || *latestFile == "" {
		sklog.Fatalf("You must set gcs_bucket and latest_file")
	}

	common.InitWithMust(
		"generic-k8s-app",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	s, err := newServer(*gcsBucket, *latestFile, *refreshRate)
	if err != nil {
		sklog.Fatalf("Could not initialize server: %s", err)
	}
	server := &http.Server{
		Addr:           *port,
		Handler:        s,
		ReadTimeout:    serverReadTimeout,
		WriteTimeout:   serverWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	sklog.Fatal(server.ListenAndServe())
}

type staticGCSServer struct {
	client     *gstorage.Client
	bucket     string
	latestFile string
	// pathToServe is the GCS path which should be the root of all served content.
	pathToServe string

	mutex sync.RWMutex // protects pathToServe
}

// ServeHTTP is a very simple web server backed by GCS. For all non-health checks, it tries to
// load the given file relative to the most recent pathToServe prefix in the configured GCS
// bucket. For example, if /foo/bar.css is the request, it will try to load the file
// gs://[bucket]/[pathToServe]/foo/bar.css and serve that. It provides mime types for HTML/CSS/JS
// files, otherwise some browsers will not render them properly. For a request to the root file,
// this is treated as requesting /index.html.
func (s *staticGCSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "/healthz" {
		w.WriteHeader(http.StatusOK)
		return
	}
	sklog.Infof("Request URI %s", r.RequestURI)
	// path.Clean prevents directory traversal attacks, as it re-writes /../../foo/bar into
	// /foo/bar, which prevents walking into parent directories.
	fileName := strings.TrimPrefix(path.Clean(r.RequestURI), "/")

	var gcsFilePath string
	s.mutex.RLock()
	if fileName == "" {
		gcsFilePath = s.pathToServe + "/index.html"
	} else {
		gcsFilePath = s.pathToServe + "/" + fileName
	}
	s.mutex.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	latestReader, err := s.client.Bucket(s.bucket).Object(gcsFilePath).NewReader(ctx)
	if err != nil {
		httputils.ReportError(w, skerr.Wrapf(err, "file %s", gcsFilePath), "Could not resolve file", http.StatusNotFound)
		return
	}
	xb, err := io.ReadAll(latestReader)
	if err != nil {
		httputils.ReportError(w, skerr.Wrapf(err, "file %s", gcsFilePath), "Could not read file", http.StatusInternalServerError)
		return
	}
	_ = latestReader.Close()

	if strings.HasSuffix(gcsFilePath, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(gcsFilePath, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(gcsFilePath, ".html") {
		w.Header().Set("Content-Type", "text/html")
	} else {
		// Just to be safe, assume everything with an unknown extension is plain text.
		w.Header().Set("Content-Type", "text/plain")
	}
	_, err = w.Write(xb)
	if err != nil {
		sklog.Warningf("Error while writing response for file %s: %s", gcsFilePath, err)
	}
}

// updatePathToServe attempts to load the content of the latestFile and setss that to be the
// new root of the content served.
func (s *staticGCSServer) updatePathToServe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	latestReader, err := s.client.Bucket(s.bucket).Object(s.latestFile).NewReader(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	xb, err := io.ReadAll(latestReader)
	if err != nil {
		return skerr.Wrap(err)
	}
	_ = latestReader.Close()
	latestPath := strings.TrimSpace(string(xb))
	sklog.Infof("Loaded latest path %s from gs://%s/%s", latestPath, s.bucket, s.latestFile)
	s.mutex.Lock()
	s.pathToServe = latestPath
	s.mutex.Unlock()
	return nil
}

// newServer initializes the server, loads the root of the content from latestFile, and starts
// a go routine to repeatedly update that root.
func newServer(gcsBucket, latestFile string, refreshRate time.Duration) (*staticGCSServer, error) {
	ctx := context.Background()
	client, err := gstorage.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	s := &staticGCSServer{
		client:     client,
		bucket:     gcsBucket,
		latestFile: latestFile,
	}
	err = s.updatePathToServe(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	go util.RepeatCtx(ctx, refreshRate, func(ctx context.Context) {
		err = s.updatePathToServe(ctx)
		if err != nil {
			sklog.Warningf("Error while updating the latest value: %s", err)
		}
	})
	return s, nil
}
