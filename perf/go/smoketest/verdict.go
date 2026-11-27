// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
)

// Verdict records the smoke test outcome for one perfserver instance.
type Verdict struct {
	Instance          string        `json:"instance"`
	BaseURL           string        `json:"base_url"`
	AppVersion        string        `json:"app_version"`
	Status            string        `json:"status"`
	ConsecutivePasses int           `json:"consecutive_passes"`
	StartedAt         time.Time     `json:"started_at"`
	FinishedAt        time.Time     `json:"finished_at"`
	Checks            []CheckResult `json:"checks"`
}

// newVerdict computes the aggregate status and consecutive pass streak for one
// run. A new app_version resets the streak to 1 because passes against an
// older build do not prove the newly rolled-out build is healthy. Timestamps
// are truncated to whole seconds in UTC so jq fromdate can parse them.
func newVerdict(instance, baseURL, appVersion string, startedAt, finishedAt time.Time, checks []CheckResult, prev *Verdict) Verdict {
	allPassed := len(checks) > 0
	for _, c := range checks {
		if c.Status != statusPass {
			allPassed = false
			break
		}
	}
	status := statusFail
	streak := 0
	if allPassed {
		status = statusPass
		streak = 1
		if prev != nil && prev.Status == statusPass && prev.AppVersion != "" && prev.AppVersion == appVersion {
			streak = prev.ConsecutivePasses + 1
		}
	}
	return Verdict{
		Instance:          instance,
		BaseURL:           baseURL,
		AppVersion:        appVersion,
		Status:            status,
		ConsecutivePasses: streak,
		StartedAt:         startedAt.UTC().Truncate(time.Second),
		FinishedAt:        finishedAt.UTC().Truncate(time.Second),
		Checks:            checks,
	}
}

func verdictPath(dir, instance string) string {
	return filepath.Join(dir, instance+".json")
}

// readVerdict returns nil, nil when the file does not exist so the first run
// for an instance starts with no prior streak and the gate can report a
// missing verdict as a blocking condition rather than an I/O error.
func readVerdict(path string) (*Verdict, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "reading verdict %s", path)
	}
	var v Verdict
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, skerr.Wrapf(err, "decoding verdict %s", path)
	}
	return &v, nil
}

func writeVerdict(path string, v Verdict) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return skerr.Wrapf(err, "creating verdict directory for %s", path)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return skerr.Wrap(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return skerr.Wrapf(err, "writing verdict %s", path)
	}
	return nil
}

// verdictStore reads and writes per-instance Verdict JSON files in either a
// local directory or a GCS bucket prefix.
type verdictStore struct {
	localDir  string
	gcsClient gcs.GCSClient
	gcsPrefix string
}

func newVerdictStore(ctx context.Context, verdictDir string, readOnly bool, gcsOverride gcs.GCSClient) (*verdictStore, error) {
	if verdictDir == "" {
		return nil, skerr.Fmt("--verdict_dir is required")
	}
	if !strings.HasPrefix(verdictDir, "gs://") {
		return &verdictStore{localDir: verdictDir}, nil
	}
	trimmed := strings.TrimRight(strings.TrimPrefix(verdictDir, "gs://"), "/")
	bucket, prefix := gcs.SplitGSPath(trimmed)
	if bucket == "" {
		return nil, skerr.Fmt("invalid GCS --verdict_dir %q", verdictDir)
	}
	if gcsOverride != nil {
		return &verdictStore{gcsClient: gcsOverride, gcsPrefix: prefix}, nil
	}
	scope := storage.ScopeReadWrite
	if readOnly {
		scope = storage.ScopeReadOnly
	}
	ts, err := google.DefaultTokenSource(ctx, scope)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting GCS token source")
	}
	httpClient := newGCSHTTPClient(ts)
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Wrapf(err, "creating GCS client")
	}
	return &verdictStore{gcsClient: gcsclient.New(storageClient, bucket), gcsPrefix: prefix}, nil
}

// newGCSHTTPClient builds the HTTP client for storage.NewClient without
// With2xxOnly so GCS 404 responses reach cloud.google.com/go/storage as
// storage.ErrObjectNotExist instead of a transport error.
func newGCSHTTPClient(ts oauth2.TokenSource) *http.Client {
	return httputils.DefaultClientConfig().WithTokenSource(ts).Client()
}

func (s *verdictStore) read(ctx context.Context, instance string) (*Verdict, error) {
	if s.gcsClient == nil {
		return readVerdict(verdictPath(s.localDir, instance))
	}
	objPath := path.Join(s.gcsPrefix, instance+".json")
	b, err := s.gcsClient.GetFileContents(ctx, objPath)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "reading verdict gs://%s/%s", s.gcsClient.Bucket(), objPath)
	}
	var v Verdict
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, skerr.Wrapf(err, "decoding verdict gs://%s/%s", s.gcsClient.Bucket(), objPath)
	}
	return &v, nil
}

func (s *verdictStore) write(ctx context.Context, v Verdict) error {
	if s.gcsClient == nil {
		return writeVerdict(verdictPath(s.localDir, v.Instance), v)
	}
	objPath := path.Join(s.gcsPrefix, v.Instance+".json")
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return skerr.Wrap(err)
	}
	b = append(b, '\n')
	opts := gcs.FileWriteOptions{ContentType: "application/json"}
	if err := s.gcsClient.SetFileContents(ctx, objPath, opts, b); err != nil {
		return skerr.Wrapf(err, "writing verdict gs://%s/%s", s.gcsClient.Bucket(), objPath)
	}
	return nil
}
