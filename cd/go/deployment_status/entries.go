// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	baseURL               = "https://prom2.skia.org/api/v1/query?query="
	query                 = "count by (cluster, exported_namespace, exported_pod, exported_container, image_id) (kube_pod_container_info)"
	skiaPublicImagePrefix = "gcr.io/skia-public/"
)

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Cluster   string `json:"cluster"`
				Container string `json:"exported_container"`
				Namespace string `json:"exported_namespace"`
				Pod       string `json:"exported_pod"`
				ImageID   string `json:"image_id"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Entry represents a running container and its resolved commit metadata.
type Entry struct {
	Cluster       string `json:"cluster"`
	Namespace     string `json:"namespace"`
	Pod           string `json:"pod"`
	Container     string `json:"container"`
	Image         string `json:"image"`
	Commit        string `json:"commit"`
	CommitTime    string `json:"commit_time"`
	CommitSubject string `json:"commit_subject"`
	IsAncestor    *bool  `json:"is_ancestor,omitempty"`
}

// columnValue returns the value of the given column name for filtering.
func (e Entry) columnValue(col string) string {
	switch col {
	case "cluster":
		return e.Cluster
	case "namespace":
		return e.Namespace
	case "pod":
		return e.Pod
	case "container":
		return e.Container
	case "commit":
		return e.Commit
	case "commit_time":
		return e.CommitTime
	case "commit_subject":
		return e.CommitSubject
	case "image":
		return e.Image
	default:
		return ""
	}
}

// EntriesResponse is the response returned by QueryEntries.
type EntriesResponse struct {
	Entries       []Entry           `json:"entries"`
	Total         int               `json:"total"`
	LastUpdated   time.Time         `json:"last_updated,omitzero"`
	RefreshError  string            `json:"refresh_error,omitempty"`
	AncestorError string            `json:"ancestor_error,omitempty"`
	FilterErrors  map[string]string `json:"filter_errors,omitempty"`
}

// getCommitHashesForRepository calls c.ListInstances once for the given
// registry and repository and returns a mapping of image digest to the slice of
// git commit hashes associated with that digest via "git-<hash>" tags.
func getCommitHashesForRepository(ctx context.Context, c docker.Client, registry, repository string) (map[string][]string, error) {
	instances, err := c.ListInstances(ctx, registry, repository)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make(map[string][]string, len(instances))
	for digest, instance := range instances {
		var commitHashes []string
		for _, tag := range instance.Tags {
			if hash, ok := stages.CommitHashFromTag(tag); ok {
				commitHashes = append(commitHashes, hash)
			}
		}
		rv[digest] = commitHashes
	}
	return rv, nil
}

// getCommitHashesForImages returns a mapping of image string to the slice of
// git commit hashes associated with that image, calling c.ListInstances only
// once for each unique (registry, repository) in parallel. If any image or
// repository fails, partial results are still returned alongside the combined
// error.
func getCommitHashesForImages(ctx context.Context, c docker.Client, images []string) (map[string][]string, error) {
	type repoKey struct {
		registry   string
		repository string
	}
	type imageInfo struct {
		key    repoKey
		digest string
	}
	uniqueImages := make(map[string]struct{}, len(images))
	for _, image := range images {
		uniqueImages[image] = struct{}{}
	}

	var mtx sync.Mutex
	var errs []error
	imageInfos := make(map[string]imageInfo, len(uniqueImages))
	uniqueRepos := map[repoKey]struct{}{}

	var wg sync.WaitGroup
	for image := range uniqueImages {
		wg.Add(1)
		go func(image string) {
			defer wg.Done()
			registry, repository, tagOrDigest, err := docker.SplitImage(image)
			if err != nil {
				mtx.Lock()
				errs = append(errs, skerr.Wrapf(err, "failed to parse image %q", image))
				mtx.Unlock()
				return
			}
			digest, err := docker.GetDigestIfNeeded(ctx, c, registry, repository, tagOrDigest)
			if err != nil {
				mtx.Lock()
				errs = append(errs, skerr.Wrapf(err, "failed to get digest for image %q", image))
				mtx.Unlock()
				return
			}
			key := repoKey{registry: registry, repository: repository}
			mtx.Lock()
			imageInfos[image] = imageInfo{key: key, digest: digest}
			uniqueRepos[key] = struct{}{}
			mtx.Unlock()
		}(image)
	}
	wg.Wait()

	repoDigests := make(map[repoKey]map[string][]string, len(uniqueRepos))
	for key := range uniqueRepos {
		wg.Add(1)
		go func(key repoKey) {
			defer wg.Done()
			digestToCommits, err := getCommitHashesForRepository(ctx, c, key.registry, key.repository)
			if err != nil {
				mtx.Lock()
				errs = append(errs, skerr.Wrapf(err, "failed to list instances for %s/%s", key.registry, key.repository))
				mtx.Unlock()
				return
			}
			mtx.Lock()
			repoDigests[key] = digestToCommits
			mtx.Unlock()
		}(key)
	}
	wg.Wait()

	rv := make(map[string][]string, len(uniqueImages))
	for image, info := range imageInfos {
		if digests, ok := repoDigests[info.key]; ok {
			rv[image] = digests[info.digest]
		}
	}
	return rv, errors.Join(errs...)
}

// fetchRawEntries queries Prometheus for running containers without resolving
// GCR image tags or git commits yet.
func fetchRawEntries(ctx context.Context, c *http.Client) ([]Entry, error) {
	u := baseURL + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)

	var r promResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, skerr.Wrap(err)
	}
	entries := make([]Entry, 0, len(r.Data.Result))
	for _, result := range r.Data.Result {
		entries = append(entries, Entry{
			Cluster:   result.Metric.Cluster,
			Namespace: result.Metric.Namespace,
			Pod:       result.Metric.Pod,
			Container: result.Metric.Container,
			Image:     result.Metric.ImageID,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Cluster != entries[j].Cluster {
			return entries[i].Cluster < entries[j].Cluster
		}
		if entries[i].Namespace != entries[j].Namespace {
			return entries[i].Namespace < entries[j].Namespace
		}
		if entries[i].Pod != entries[j].Pod {
			return entries[i].Pod < entries[j].Pod
		}
		return entries[i].Container < entries[j].Container
	})
	return entries, nil
}

// populateEntryCommits resolves GCR commit tags and the most recent commit for
// each entry in entries. If some images or repositories fail to resolve, it
// still populates commit metadata for the remaining entries and returns the
// combined error.
func populateEntryCommits(ctx context.Context, d docker.Client, repos []GitRepo, entries []Entry) error {
	images := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Image, skiaPublicImagePrefix) {
			images = append(images, entry.Image)
		}
	}
	commitsByImage, commitsErr := getCommitHashesForImages(ctx, d, images)

	uniqueCommitSets := map[string][]string{}
	for _, commitHashes := range commitsByImage {
		if len(commitHashes) > 0 {
			key := strings.Join(commitHashes, ",")
			uniqueCommitSets[key] = commitHashes
		}
	}

	var commitMtx sync.Mutex
	mostRecentByKey := make(map[string]*vcsinfo.LongCommit, len(uniqueCommitSets))
	var wg sync.WaitGroup
	for key, commitHashes := range uniqueCommitSets {
		wg.Add(1)
		go func(key string, commitHashes []string) {
			defer wg.Done()
			c := getMostRecentCommit(ctx, repos, commitHashes)
			commitMtx.Lock()
			mostRecentByKey[key] = c
			commitMtx.Unlock()
		}(key, commitHashes)
	}
	wg.Wait()

	for i := range entries {
		hashes := commitsByImage[entries[i].Image]
		if len(hashes) == 0 {
			continue
		}
		if c := mostRecentByKey[strings.Join(hashes, ",")]; c != nil {
			entries[i].Commit = c.Hash
			entries[i].CommitTime = c.Timestamp.Local().Format("2006-01-02 15:04")
			entries[i].CommitSubject = c.Subject
		} else {
			entries[i].Commit = hashes[0]
		}
	}
	if commitsErr != nil {
		return skerr.Wrap(commitsErr)
	}
	return nil
}

// fetchEntries fetches all container entries from Prometheus and resolves their
// GCR and Git commit metadata. If commit metadata fails to resolve for some
// images or repositories, partial entries are still returned alongside the
// error.
func fetchEntries(ctx context.Context, c *http.Client, d docker.Client, repos []GitRepo) ([]Entry, error) {
	entries, err := fetchRawEntries(ctx, c)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := populateEntryCommits(ctx, d, repos, entries); err != nil {
		return entries, skerr.Wrap(err)
	}
	return entries, nil
}
