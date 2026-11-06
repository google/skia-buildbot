// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/storage"
	"github.com/rs/cors"
	"go.skia.org/infra/go/bazel"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
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

	// Keep in sync with //bazel/gcs_mirror.bzl.
	supportedSuffixes = []string{".tar.gz", ".tgz", ".tar.xz", ".deb", ".zip"}
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	gcsPath := fmt.Sprintf("gs://%s/%s", *bucket, path)

	obj := gcsClient.Bucket(*bucket).Object(path)
	reader, err := obj.NewReader(r.Context())
	if skerr.Unwrap(err) == storage.ErrObjectNotExist {
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
		"cdn",
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	if *bucket == "" {
		sklog.Fatal("--bucket is required.")
	}
	*bucket = strings.TrimPrefix(*bucket, "gs://")

	ctx := context.Background()
	// Note: storage.NewClient() will use Application Default Credentials by
	// default if option.WithHTTPClient is not provided, but doing so results in
	// the caller needing to have the serviceusage.services.use permission for
	// the project in question, which our developer accounts do not seem to
	// have by default.
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadWrite)
	if err != nil {
		sklog.Fatal(err)
	}
	gcsHttpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gcsClient, err = storage.NewClient(ctx, option.WithScopes(storage.ScopeReadWrite), storage.WithJSONReads(), option.WithHTTPClient(gcsHttpClient))
	if err != nil {
		sklog.Fatal(err)
	}

	gitilesRepo, err := gitiles.NewRepo(ctx, common.REPO_SKIA_INFRA)
	if err != nil {
		sklog.Fatal(err)
	}
	backupHttpClient := httputils.DefaultClientConfig().With2xxAnd3xx().Client()

	// Run file backup in a separate goroutine to avoid downtime if it fails.
	go util.RepeatCtx(ctx, 15*time.Minute, func(ctx context.Context) {
		if err := ensureBackups(ctx, gitilesRepo, backupHttpClient); err != nil {
			sklog.Errorf("Failed backups: %s", err)
		}
	})

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	h := httputils.LoggingRequestResponse(http.HandlerFunc(storageHandler))
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			Debug:          true,
		}).Handler(h)
		h = httputils.HealthzAndHTTPS(h)
	}
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, h))
}

func gcsMirrorObjectPath(url, sha256 string) string {
	var ext string
	for _, suffix := range supportedSuffixes {
		if strings.HasSuffix(url, suffix) {
			ext = suffix
			break
		}
	}
	return fmt.Sprintf("bazel/%s%s", sha256, ext)
}

func ensureBackups(ctx context.Context, repo gitiles.GitilesRepo, httpClient *http.Client) error {
	sklog.Infof("Backing up URLs from MODULE.bazel...")
	contents, err := repo.ReadFile(ctx, "MODULE.bazel")
	if err != nil {
		return skerr.Wrap(err)
	}
	deps, err := bazel.ParseDeps(string(contents))
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, dep := range deps {
		if dep.GetFunction() != "gcs_mirror.http_archive" && dep.GetFunction() != "gcs_mirror.http_file" {
			continue
		}
		singleDep, ok := dep.(*bazel.SingleDependency)
		if !ok {
			return skerr.Fmt("expected SingleDependency for %s (%s)", dep.GetID(), dep.GetFunction())
		}
		if err := maybeBackupURL(ctx, httpClient, singleDep.Version, singleDep.SHA256); err != nil {
			return skerr.Wrapf(err, "failed to back up %q (%s)", singleDep.ID, singleDep.Version)
		}
	}
	sklog.Infof("Done backing up URLs from MODULE.bazel.")
	return nil
}

func maybeBackupURL(ctx context.Context, httpClient *http.Client, url, expectSha256 string) error {
	// Per storage API docs, if the context is cancelled before Writer.Close()
	// is called, the writes are not saved. We'll use this to ensure that the
	// object doesn't end up existing if the digest doesn't match the
	// expectation.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	objectPath := gcsMirrorObjectPath(url, expectSha256)
	obj := gcsClient.Bucket(*bucket).Object(objectPath)
	if _, err := obj.Attrs(ctx); err == nil {
		// The object already exists; nothing to do.
		return nil
	} else if skerr.Unwrap(err) != storage.ErrObjectNotExist {
		return skerr.Wrapf(err, "failed checking attributes of %s", objectPath)
	}

	// We'll write the object while computing the digest.
	const bufSize = 1024 * 1024
	objWriter := obj.NewWriter(ctx)
	objWriter.ChunkSize = bufSize
	hash := sha256.New()
	w := io.MultiWriter(objWriter, hash)

	// Read the URL.
	resp, err := httpClient.Get(url)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer resp.Body.Close()

	// Perform the copy.
	buf := make([]byte, bufSize)
	if _, err := io.CopyBuffer(w, resp.Body, buf); err != nil {
		return skerr.Wrap(err)
	}

	// Ensure that the digest matches the expectation.
	actualSha256 := fmt.Sprintf("%x", hash.Sum(nil))
	if actualSha256 != expectSha256 {
		return skerr.Fmt("incorrect sha256 digest for %q: expected %s but got %s", url, expectSha256, actualSha256)
	}
	sklog.Infof("Backed up %s to %s", url, objectPath)

	return skerr.Wrap(objWriter.Close())
}
