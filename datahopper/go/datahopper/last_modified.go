package main

import (
	"context"
	"net/http"
	"time"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	METRIC_LAST_MODIFIED = "liveness_last_file_modification_s"
)

func updateLastModifiedMetrics(ctx context.Context, client *http.Client, reposToFiles map[*gitiles.Repo][]string) error {
	now := time.Now()
	for repo, files := range reposToFiles {
		for _, file := range files {
			log, err := repo.Log(ctx, "master", gitiles.LogLimit(1), gitiles.LogPath(file))
			if err != nil {
				return skerr.Wrapf(err, "Failed loading %s", file)
			}
			if len(log) != 1 {
				return skerr.Fmt("Failed to obtain the last commit which modified %s in %s; expected 1 commit but got %d", file, repo.URL, len(log))
			}
			metrics2.GetInt64Metric(METRIC_LAST_MODIFIED, map[string]string{
				"repo": repo.URL,
				"file": file,
			}).Update(int64(now.Sub(log[0].Timestamp).Seconds()))
		}
	}
	return nil
}

func StartLastModifiedMetrics(ctx context.Context, client *http.Client, filesByRepo map[string][]string) {
	reposToFiles := make(map[*gitiles.Repo][]string, len(filesByRepo))
	for repo, filePaths := range filesByRepo {
		r := gitiles.NewRepo(repo, client)
		reposToFiles[r] = filePaths
	}

	lv := metrics2.NewLiveness("last_successful_last_modified_metrics")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		if err := updateLastModifiedMetrics(ctx, client, reposToFiles); err == nil {
			lv.Reset()
		} else {
			sklog.Errorf("Failed to update file last-modified metrics: %s", err)
		}
	})
}
