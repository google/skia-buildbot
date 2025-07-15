package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"cloud.google.com/go/storage"
	"github.com/rs/cors"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var (
	// Flags.
	host     = flag.String("host", "localhost", "HTTP service host")
	port     = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	bucket   = flag.String("bucket", "", "GCS bucket containing the files to serve.")

	// Global GCS client.
	gcsClient *storage.Client
)

func validatePath(path string) error {
	if path == "" {
		return errors.New("path is empty")
	}
	if !utf8.ValidString(path) {
		return errors.New("path is not valid UTF-8")
	}
	if path == "." || path == ".." {
		return errors.New("'.' and '..' are not allowed in GCS paths")
	}
	return nil
}

func storageHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if err := validatePath(path); err != nil {
		sklog.Error(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	gcsPath := fmt.Sprintf("gs://%s/%s", *bucket, path)

	obj := gcsClient.Bucket(*bucket).Object(path)
	reader, err := obj.NewReader(r.Context())
	if err == storage.ErrObjectNotExist {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	} else if err != nil {
		sklog.Errorf("Error creating object reader for %s: %s", gcsPath, err)
		http.Error(w, "unknown error", http.StatusInternalServerError)
		return
	}
	defer util.Close(reader)

	w.Header().Set("Content-Type", reader.Attrs.ContentType)
	w.Header().Set("Content-Encoding", reader.Attrs.ContentEncoding)
	size, err := io.Copy(w, reader)
	if err != nil {
		sklog.Errorf("Error reading object %s: %s", gcsPath, err)
		http.Error(w, "unknown error", http.StatusInternalServerError)
		return
	}
	if size != reader.Attrs.Size {
		errMsg := fmt.Sprintf("Read incorrect number of bytes for %s. Expected %d but read %d", gcsPath, reader.Attrs.Size, size)
		sklog.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
}

func main() {
	common.InitWithMust(
		"autoroll-fe",
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	if *bucket == "" {
		sklog.Fatal("--bucket is required.")
	}
	*bucket = strings.TrimPrefix(*bucket, "gs://")

	ctx := context.Background()
	// Note: storage.NewClient() will use Application Default Credentials by
	// default if option.WithHTTPClient is not provided, but doing to results in
	// the caller needing to have the serviceusage.services.use permission for
	// the project in question, which our developer accounts do not seem to
	// have by default.
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gcsClient, err = storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly), storage.WithJSONReads(), option.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	h := httputils.LoggingRequestResponse(http.HandlerFunc(storageHandler))
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = cors.New(cors.Options{
			AllowedOrigins: []string{"*.skia.org", "*.luci.app"},
			Debug:          true,
		}).Handler(h)
		h = httputils.HealthzAndHTTPS(h)
	}
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, h))
}
