// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/docker"
	docker_mocks "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func TestGetCommitHashesForImages_PartialResults(t *testing.T) {
	ctx := context.Background()
	digestGood := "sha256:" + strings.Repeat("a", 64)
	digestTagged := "sha256:" + strings.Repeat("b", 64)
	digestFailRepo := "sha256:" + strings.Repeat("c", 64)
	hash1 := strings.Repeat("1", 40)
	hash2 := strings.Repeat("2", 40)

	imgGood := "gcr.io/skia-public/app@" + digestGood
	imgTagged := "gcr.io/skia-public/app:prod"
	imgBadFormat := "gcr.io/skia-public/bad@sha256:not-64-hex"
	imgBadTag := "gcr.io/skia-public/app:missing-tag"
	imgFailRepo := "gcr.io/skia-public/deleted-repo@" + digestFailRepo

	d := docker_mocks.NewClient(t)
	d.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/app", "prod").Return(&docker.Manifest{
		Digest: digestTagged,
	}, nil).Once()
	d.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/app", "missing-tag").Return(nil, errors.New("404 manifest unknown")).Once()
	d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/app").Return(map[string]*docker.ImageInstance{
		digestGood: {
			Tags: []string{stages.GitTagPrefix + hash1, "prod", stages.GitTagPrefix + "not-a-hash"},
		},
		digestTagged: {
			Tags: []string{stages.GitTagPrefix + hash2},
		},
	}, nil).Once()
	d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/deleted-repo").Return(nil, errors.New("403 forbidden")).Once()

	got, err := getCommitHashesForImages(ctx, d, []string{imgGood, imgGood, imgTagged, imgBadFormat, imgBadTag, imgFailRepo})
	require.Error(t, err)
	require.Contains(t, err.Error(), imgBadFormat)
	require.Contains(t, err.Error(), imgBadTag)
	require.Contains(t, err.Error(), "skia-public/deleted-repo")

	require.Equal(t, []string{hash1}, got[imgGood])
	require.Equal(t, []string{hash2}, got[imgTagged])
	require.Empty(t, got[imgBadFormat])
	require.Empty(t, got[imgBadTag])
	require.Empty(t, got[imgFailRepo])
}

func makePromPayload(t *testing.T, metrics []struct {
	Cluster   string
	Namespace string
	Pod       string
	Container string
	ImageID   string
}) []byte {
	t.Helper()
	var r promResponse
	r.Status = "success"
	r.Data.ResultType = "vector"
	for _, m := range metrics {
		r.Data.Result = append(r.Data.Result, struct {
			Metric struct {
				Cluster   string `json:"cluster"`
				Container string `json:"exported_container"`
				Namespace string `json:"exported_namespace"`
				Pod       string `json:"exported_pod"`
				ImageID   string `json:"image_id"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		}{
			Metric: struct {
				Cluster   string `json:"cluster"`
				Container string `json:"exported_container"`
				Namespace string `json:"exported_namespace"`
				Pod       string `json:"exported_pod"`
				ImageID   string `json:"image_id"`
			}{
				Cluster:   m.Cluster,
				Container: m.Container,
				Namespace: m.Namespace,
				Pod:       m.Pod,
				ImageID:   m.ImageID,
			},
		})
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	return b
}

func TestFetchEntries_PartialResults(t *testing.T) {
	ctx := context.Background()
	digestOK := "sha256:" + strings.Repeat("a", 64)
	digestFail := "sha256:" + strings.Repeat("b", 64)
	hashOld := strings.Repeat("1", 40)
	hashNew := strings.Repeat("2", 40)
	ts := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)

	imgOK := "gcr.io/skia-public/app@" + digestOK
	imgFail := "gcr.io/skia-public/broken@" + digestFail
	imgExternal := "docker.io/library/redis@sha256:" + strings.Repeat("f", 64)

	promURL := baseURL + url.QueryEscape(query)
	urlMock := mockhttpclient.NewURLMock()
	urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(makePromPayload(t, []struct {
		Cluster   string
		Namespace string
		Pod       string
		Container string
		ImageID   string
	}{
		{Cluster: "b-cluster", Namespace: "default", Pod: "pod-ext", Container: "redis", ImageID: imgExternal},
		{Cluster: "a-cluster", Namespace: "default", Pod: "pod-fail", Container: "broken", ImageID: imgFail},
		{Cluster: "a-cluster", Namespace: "default", Pod: "pod-ok", Container: "app", ImageID: imgOK},
	})))

	d := docker_mocks.NewClient(t)
	d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/app").Return(map[string]*docker.ImageInstance{
		digestOK: {Tags: []string{stages.GitTagPrefix + hashOld, stages.GitTagPrefix + hashNew}},
	}, nil).Once()
	d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/broken").Return(nil, errors.New("not found")).Once()

	lcOld := makeLongCommit(hashOld, nil, ts.Add(-time.Hour), "older commit")
	lcNew := makeLongCommit(hashNew, []string{hashOld}, ts, "newer commit")
	repo := gitiles_mocks.NewGitilesRepo(t)
	repo.On("Details", testutils.AnyContext, hashOld).Return(lcOld, nil).Once()
	repo.On("LogFirstParent", testutils.AnyContext, hashOld, git.MainBranch).Return([]*vcsinfo.LongCommit{lcNew}, nil).Once()

	entries, err := fetchEntries(ctx, urlMock.Client(), d, []GitRepo{repo})
	require.Error(t, err)
	require.True(t, urlMock.Empty())
	require.Len(t, entries, 3)

	// Sorted by cluster, namespace, pod, container.
	require.Equal(t, "pod-fail", entries[0].Pod)
	require.Empty(t, entries[0].Commit)

	require.Equal(t, "pod-ok", entries[1].Pod)
	require.Equal(t, hashNew, entries[1].Commit)
	require.Equal(t, "newer commit", entries[1].CommitSubject)
	require.Equal(t, ts.Local().Format("2006-01-02 15:04"), entries[1].CommitTime)

	require.Equal(t, "pod-ext", entries[2].Pod)
	require.Empty(t, entries[2].Commit)
}
