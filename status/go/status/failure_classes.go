package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	autogardener_db "go.skia.org/infra/autogardener/go/db"
	autogardener_types "go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/sync/errgroup"
)

const failureClassesWindow = 90 * 24 * time.Hour

type failureClassesRequest struct {
	TaskIDs []string `json:"taskIds"`
}

type activeFailureClass struct {
	FailureClass *autogardener_types.FailureClass `json:"failureClass"`
	TaskIDs      []string                         `json:"taskIds"`
}

type failureClassesCache struct {
	db             autogardener_db.AutoGardenerDB
	mtx            sync.RWMutex
	taskSummaries  map[string]*autogardener_types.TaskSummary
	failureClasses map[string]*autogardener_types.FailureClass
}

func newFailureClassesCache(ctx context.Context, db autogardener_db.AutoGardenerDB) *failureClassesCache {
	c := &failureClassesCache{
		db:             db,
		taskSummaries:  make(map[string]*autogardener_types.TaskSummary),
		failureClasses: make(map[string]*autogardener_types.FailureClass),
	}
	ch := db.ModifiedFailureClassesCh(ctx, failureClassesWindow)
	go func() {
		for fcs := range ch {
			c.mtx.Lock()
			for _, fc := range fcs {
				c.failureClasses[fc.Id] = fc
			}
			c.mtx.Unlock()
		}
	}()
	return c
}

func (c *failureClassesCache) getTaskSummary(ctx context.Context, taskID string) (*autogardener_types.TaskSummary, error) {
	c.mtx.RLock()
	cached, ok := c.taskSummaries[taskID]
	c.mtx.RUnlock()
	if ok {
		return cached, nil
	}

	summary, err := c.db.GetTaskSummary(ctx, taskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Only cache once FailureClassId has been assigned by autogardener.
	if summary != nil && summary.FailureClassId != "" {
		c.mtx.Lock()
		c.taskSummaries[taskID] = summary
		c.mtx.Unlock()
	}
	return summary, nil
}

func (c *failureClassesCache) getFailureClass(ctx context.Context, id string) (*autogardener_types.FailureClass, error) {
	c.mtx.RLock()
	cached, ok := c.failureClasses[id]
	c.mtx.RUnlock()
	if ok {
		return cached, nil
	}

	fc, err := c.db.GetFailureClass(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if fc != nil {
		c.mtx.Lock()
		if _, ok := c.failureClasses[id]; !ok {
			c.failureClasses[id] = fc
		}
		c.mtx.Unlock()
	}
	return fc, nil
}

func (c *failureClassesCache) getActiveFailureClasses(ctx context.Context, taskIDs []string) ([]activeFailureClass, error) {
	var mtx sync.Mutex
	tasksByClassID := make(map[string][]string)
	var eg errgroup.Group
	for _, taskID := range taskIDs {
		taskID := taskID // https://golang.org/doc/faq#closures_and_goroutines
		eg.Go(func() error {
			summary, err := c.getTaskSummary(ctx, taskID)
			if err != nil {
				return err
			}
			if summary != nil && summary.FailureClassId != "" {
				mtx.Lock()
				tasksByClassID[summary.FailureClassId] = append(tasksByClassID[summary.FailureClassId], taskID)
				mtx.Unlock()
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}

	results := make([]activeFailureClass, 0, len(tasksByClassID))
	var fcEg errgroup.Group
	for classID, classTaskIDs := range tasksByClassID {
		classID := classID           // https://golang.org/doc/faq#closures_and_goroutines
		classTaskIDs := classTaskIDs // https://golang.org/doc/faq#closures_and_goroutines
		fcEg.Go(func() error {
			fc, err := c.getFailureClass(ctx, classID)
			if err != nil {
				return err
			}
			if fc == nil {
				return nil
			}
			sortedTaskIDs := append([]string(nil), classTaskIDs...)
			sort.Strings(sortedTaskIDs)
			mtx.Lock()
			results = append(results, activeFailureClass{
				FailureClass: fc,
				TaskIDs:      sortedTaskIDs,
			})
			mtx.Unlock()
			return nil
		})
	}
	if err := fcEg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}

	sort.Slice(results, func(i, j int) bool {
		if len(results[i].TaskIDs) != len(results[j].TaskIDs) {
			return len(results[i].TaskIDs) > len(results[j].TaskIDs)
		}
		return results[i].FailureClass.Id < results[j].FailureClass.Id
	})

	return results, nil
}

func (c *failureClassesCache) Handler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")

	var req failureClassesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Invalid request body.", http.StatusBadRequest)
		return
	}

	classes, err := c.getActiveFailureClasses(r.Context(), req.TaskIDs)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve failure classes.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(classes); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}
