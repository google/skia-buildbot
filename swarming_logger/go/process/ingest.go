package process

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/sync/syncmap"
)

/*
	Download Swarming logs and push into Google Storage.
*/

const (
	GS_BUCKET = "skia-swarming-logs"
	// Logs indexed by {prefix}/YYYY/MM/DD/HH/{basename}
	GS_PATH       = "%s/%04d/%02d/%02d/%02d/%s"
	GS_PREFIX_RAW = "raw"

	TIME_CHUNK  = 24 * time.Hour
	NUM_WORKERS = 20
)

// Ingest a single Task's log.
func ingestLog(gs *storage.Client, s swarming.ApiClient, t *types.Task) error {
	gsPath := fmt.Sprintf(GS_PATH, GS_PREFIX_RAW, t.Created.Year(), t.Created.Month(), t.Created.Day(), t.Created.Hour(), t.SwarmingTaskId+".log")
	gsObj := gs.Bucket(GS_BUCKET).Object(gsPath)

	// Skip already-ingested tasks.
	if _, err := gsObj.Attrs(context.Background()); err == nil {
		// The object exists; skip it.
		return nil
	} else if err != storage.ErrObjectNotExist {
		// According to the docs, the API should return ErrObjectNotExist
		// when the object doesn't exist. However, we're getting standard
		// 404s here, so we just have to assume that an error means that
		// the object isn't there.
	}

	// Obtain the Swarming task log.
	stdout, err := s.SwarmingService().Task.Stdout(t.SwarmingTaskId).Do()
	if err != nil {
		return err
	}

	// Write the logs to GS.
	return gcs.WriteObj(gsObj, []byte(stdout.Output))
}

// Ingest logs for all completed tasks within the given time chunk.
func ingestLogsChunk(taskDb db.TaskReader, gcs *storage.Client, s swarming.ApiClient, start, end time.Time, ingested *syncmap.Map) (map[string]error, error) {
	tasks, err := taskDb.GetTasksFromDateRange(start, end, "")
	if err != nil {
		return nil, err
	}
	sklog.Infof("Ingesting logs for %d tasks created between %s and %s.", len(tasks), start, end)
	queue := make(chan *types.Task)
	go func() {
		for _, t := range tasks {
			if !t.Done() || t.SwarmingTaskId == "" {
				continue
			}
			queue <- t
		}
		close(queue)
	}()

	var wg sync.WaitGroup
	var failMtx sync.Mutex
	fails := map[string]error{}
	for i := 0; i < NUM_WORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range queue {
				if _, ok := ingested.Load(t.Id); ok {
					continue
				}
				if err := ingestLog(gcs, s, t); err != nil {
					failMtx.Lock()
					fails[t.Id] = err
					failMtx.Unlock()
					sklog.Errorf("Failed to ingest task: %s", err)
				} else {
					ingested.Store(t.Id, true)
					time.AfterFunc(50*time.Hour, func() { // Slightly longer than the ingestion window.
						ingested.Delete(t.Id)
					})
				}
			}
		}()
	}
	wg.Wait()

	return fails, nil
}

// Ingest logs for all completed tasks within a given time period.
func IngestLogs(taskDb db.TaskReader, gcs *storage.Client, s swarming.ApiClient, start, end time.Time, ingested *syncmap.Map) error {
	failed := map[string]error{}
	if err := util.IterTimeChunks(start, end, TIME_CHUNK, func(chunkStart, chunkEnd time.Time) error {
		fails, err := ingestLogsChunk(taskDb, gcs, s, chunkStart, chunkEnd, ingested)
		if err != nil {
			return err
		}
		for k, v := range fails {
			failed[k] = v
		}
		// Don't stop early for ingestion errors.
		return nil
	}); err != nil {
		return err
	}
	if len(failed) > 0 {
		failMsg := fmt.Sprintf("Failed to ingest logs for %d tasks.\n", len(failed))
		for k, v := range failed {
			failMsg += fmt.Sprintf("  %s: %s\n", k, v)
		}
		return errors.New(failMsg)
	}
	return nil
}

// Ingest logs for completed tasks periodically.
func IngestLogsPeriodically(ctx context.Context, taskDb db.TaskReader, gcs *storage.Client, s swarming.ApiClient) {
	lv := metrics2.NewLiveness("last_successful_swarming_task_gcs_ingestion")
	ingested := &syncmap.Map{}
	go util.RepeatCtx(time.Hour, ctx, func() {
		end := time.Now()
		start := end.Add(-48 * time.Hour) // Use a big window, because we're lazy.
		if err := IngestLogs(taskDb, gcs, s, start, end, ingested); err != nil {
			sklog.Errorf("Failed to ingest Swarming logs to GCS: %s", err)
		} else {
			lv.Reset()
		}
	})
}

// Ingest all logs since the "beginning of time". This should only ever be run
// once, after which IngestLogsPeriodically should be used.
func InitialIngestLogs(taskDb db.TaskReader, gcs *storage.Client, s swarming.ApiClient) error {
	start := time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().Add(-24 * time.Hour)
	return IngestLogs(taskDb, gcs, s, start, end, &syncmap.Map{})
}
