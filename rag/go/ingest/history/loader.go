// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package history

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/rag/go/filereaders/zip"
	"go.skia.org/infra/rag/go/topicstore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// LoadInMemoryStoreFromGCS loads topic data from GCS into the provided in-memory store.
func LoadInMemoryStoreFromGCS(ctx context.Context, store topicstore.TopicStore, bucketName, indexDate, defaultRepoName string, dimensionality int) error {
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly)
	if err != nil {
		return skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	client, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return skerr.Wrap(err)
	}
	defer client.Close()

	prefix := fmt.Sprintf("embeddings/%s/", indexDate)
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return skerr.Wrap(err)
	}
	localCacheDir := filepath.Join(cacheDir, "rag_index_cache")

	// List all objects with the prefix in the bucket.
	sklog.Infof("Listing objects in bucket %s with prefix %s", bucketName, prefix)
	it := client.Bucket(bucketName).Objects(ctx, &storage.Query{Prefix: prefix})

	// Create an ingester that will write to the store.
	ingester := New(store, dimensionality, defaultRepoName)

	var allAttrs []*storage.ObjectAttrs
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return skerr.Wrap(err)
		}
		if strings.HasSuffix(attrs.Name, "topics.zip") {
			allAttrs = append(allAttrs, attrs)
		}
	}

	if len(allAttrs) == 0 {
		sklog.Warningf("No topics.zip files found in bucket %s with prefix %s", bucketName, prefix)
		return nil
	}

	err = util.ChunkIterParallelPool(ctx, len(allAttrs), 1, 10, func(ctx context.Context, startIdx, endIdx int) error {
		attrs := allAttrs[startIdx]
		parts := strings.Split(attrs.Name, "/")
		if len(parts) < 2 {
			return nil
		}
		// The parent directory of topics.zip is assumed to be the repository name.
		repoNameCandidate := parts[len(parts)-2]
		repoName := repoNameCandidate
		// Simple check if it's a hash, similar to pubsub_source.go
		if len(repoNameCandidate) == 40 {
			repoName = "" // fallback to default or use candidate if not hex
		}

		localPath := filepath.Join(localCacheDir, attrs.Name)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return skerr.Wrap(err)
		}

		// Download the file if it doesn't exist locally.
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			sklog.Infof("Downloading %s from GCS", attrs.Name)
			reader, err := client.Bucket(bucketName).Object(attrs.Name).NewReader(ctx)
			if err != nil {
				return skerr.Wrap(err)
			}
			defer reader.Close()

			outFile, err := os.Create(localPath)
			if err != nil {
				return skerr.Wrap(err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, reader); err != nil {
				return skerr.Wrap(err)
			}
		} else {
			sklog.Infof("Using cached file %s", localPath)
		}

		// Extract the zip file.
		content, err := os.ReadFile(localPath)
		if err != nil {
			return skerr.Wrap(err)
		}

		tempExtractDir, err := os.MkdirTemp("", "extracted-")
		if err != nil {
			return skerr.Wrap(err)
		}
		defer util.RemoveAll(tempExtractDir)

		if err := zip.ExtractZipData(content, tempExtractDir); err != nil {
			return skerr.Wrap(err)
		}

		topicsDirPath := filepath.Join(tempExtractDir, "topics")
		embeddingsFilePath := filepath.Join(tempExtractDir, "embeddings.npy")
		indexPickleFilePath := filepath.Join(tempExtractDir, "index.pkl")

		// Ingest topics from the extracted files into the store.
		sklog.Infof("Ingesting topics for repo %s from %s", repoName, attrs.Name)
		if err := ingester.IngestTopics(ctx, topicsDirPath, embeddingsFilePath, indexPickleFilePath, repoName); err != nil {
			return skerr.Wrap(err)
		}
		sklog.Infof("Successfully ingested topics from %s", attrs.Name)
		return nil
	})

	return skerr.Wrap(err)
}
