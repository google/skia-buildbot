// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/taskqueue"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/rand/cryptorand"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/settings"
	"golang.org/x/net/context"
)

// Testing is a high-level testing object for testing applications that use
// tumble.
type Testing struct {
	Service
}

// UpdateSettings changes the tumble settings in the context to match cfg.
//
// If cfg == nil, this resets the settings to their default values.
func (t *Testing) UpdateSettings(c context.Context, cfg *Config) {
	if cfg == nil {
		dflt := defaultConfig
		dflt.DustSettleTimeout = 0
		cfg = &dflt
	}
	settings.Set(c, baseName, cfg, "tumble.Testing", "for testing")
}

// GetConfig retrieves the current tumble settings
func (t *Testing) GetConfig(c context.Context) *Config {
	return getConfig(c)
}

// Context generates a correctly configured context with:
//   * luci/gae/impl/memory
//   * luci/luci-go/common/clock/testclock
//   * luci/luci-go/common/logging/memlogger
//   * luci/luci-go/server/settings (MemoryStorage)
//
// It also correctly configures the "tumble.Mutation" indexes and taskqueue
// named in this Testing config.
func (t *Testing) Context() context.Context {
	ctx := memory.Use(memlogger.Use(context.Background()))
	ctx, _ = testclock.UseTime(ctx, testclock.TestTimeUTC.Round(time.Millisecond))
	ctx = settings.Use(ctx, settings.New(&settings.MemoryStorage{}))
	ctx = cryptorand.MockForTest(ctx, 765589025) // as chosen by fair dice roll
	t.UpdateSettings(ctx, nil)

	taskqueue.Get(ctx).Testable().CreateQueue(baseName)

	ds := datastore.Get(ctx)
	ds.Testable().AddIndexes(&datastore.IndexDefinition{
		Kind: "tumble.Mutation",
		SortBy: []datastore.IndexColumn{
			{Property: "ExpandedShard"},
			{Property: "TargetRoot"},
		},
	})
	ds.Testable().Consistent(true)

	return ctx
}

// EnableDelayedMutations turns on delayed mutations for this context.
func (t *Testing) EnableDelayedMutations(c context.Context) {
	cfg := t.GetConfig(c)
	if !cfg.DelayedMutations {
		cfg.DelayedMutations = true
		datastore.Get(c).Testable().AddIndexes(&datastore.IndexDefinition{
			Kind: "tumble.Mutation",
			SortBy: []datastore.IndexColumn{
				{Property: "TargetRoot"},
				{Property: "ProcessAfter"},
			},
		})
		t.UpdateSettings(c, cfg)
	}
}

// Iterate makes a single iteration of the tumble service worker, and returns
// the number of shards that were processed.
//
// It will skip all work items if the test clock hasn't advanced in time
// enough.
func (t *Testing) Iterate(c context.Context) int {
	tq := taskqueue.Get(c)
	clk := clock.Get(c).(testclock.TestClock)
	logging.Debugf(c, "tumble.Testing.Iterate: time(%d|%s)", timestamp(clk.Now().Unix()), clk.Now().UTC())

	ret := 0
	tsks := tq.Testable().GetScheduledTasks()[baseName]
	logging.Debugf(c, "got tasks: %v", tsks)
	for _, tsk := range tsks {
		logging.Debugf(c, "found task: %v", tsk)
		if tsk.ETA.After(clk.Now().UTC()) {
			logging.Infof(c, "skipping task: ETA(%s): %s", tsk.ETA, tsk.Path)
			continue
		}
		toks := strings.Split(tsk.Path, "/")

		// Process the shard until a success or hard failure.
		retryHTTP(c, func(rec *httptest.ResponseRecorder) {
			t.ProcessShardHandler(&router.Context{
				Context: c,
				Writer:  rec,
				Request: &http.Request{
					Header: http.Header{"X-AppEngine-QueueName": []string{baseName}},
				},
				Params: httprouter.Params{
					{Key: "shard_id", Value: toks[4]},
					{Key: "timestamp", Value: toks[6]},
				},
			})
		})

		if err := tq.Delete(tsk, baseName); err != nil {
			panic(fmt.Errorf("Deleting task failed: %s", err))
		}
		ret++
	}
	return ret
}

// FireAllTasks will force all tumble shards to run in the future.
func (t *Testing) FireAllTasks(c context.Context) {
	retryHTTP(c, func(rec *httptest.ResponseRecorder) {
		// Fire all tasks until a success or hard failure.
		t.FireAllTasksHandler(&router.Context{
			Context: c,
			Writer:  rec,
			Request: &http.Request{
				Header: http.Header{"X-Appengine-Cron": []string{"true"}},
			},
		})
	})
}

// AdvanceTime advances the test clock enough so that Iterate will be able to
// pick up tasks in the task queue.
func (t *Testing) AdvanceTime(c context.Context) {
	clk := clock.Get(c).(testclock.TestClock)
	cfg := t.GetConfig(c)
	toAdd := time.Duration(cfg.TemporalMinDelay) + time.Duration(cfg.TemporalRoundFactor) + time.Second
	logging.Infof(c, "adding %s to %s", toAdd, clk.Now().UTC())
	clk.Add(toAdd)
}

// Drain will run a loop, advancing time and iterating through tumble mutations
// until tumble's queue is empty. It returns the total number of processed
// shards.
func (t *Testing) Drain(c context.Context) int {
	ret := 0
	for {
		t.AdvanceTime(c)
		processed := t.Iterate(c)
		if processed == 0 {
			break
		}
		ret += processed
	}
	return ret
}

// ResetLog resets the current memory logger to the empty state.
func (t *Testing) ResetLog(c context.Context) {
	logging.Get(c).(*memlogger.MemLogger).Reset()
}

// DumpLog dumps the current memory logger to stdout to help with debugging.
func (t *Testing) DumpLog(c context.Context) {
	logging.Get(c).(*memlogger.MemLogger).Dump(os.Stdout)
}

// retryHTTP will record an HTTP request and handle its response.
//
// It will return if the response indicated success, retry the request if the
// response indicated a transient failure, or panic if the response indicated a
// hard failure.
func retryHTTP(c context.Context, reqFn func(rec *httptest.ResponseRecorder)) {
	for {
		rec := httptest.NewRecorder()
		reqFn(rec)

		switch rec.Code {
		case http.StatusOK, http.StatusNoContent:
			return

		case http.StatusInternalServerError:
			bodyStr := rec.Body.String()
			err := fmt.Errorf("internal server error: %s", bodyStr)
			if rec.Header().Get(transientHTTPHeader) == "" {
				lmsg := logging.Get(c).(*memlogger.MemLogger).Messages()
				panic(fmt.Errorf("HTTP non-transient error: %s: %#v", err, lmsg))
			}
			logging.WithError(err).Warningf(c, "Transient error encountered, retrying.")

		default:
			panic(fmt.Errorf("HTTP error %d (%s): %s", rec.Code, http.StatusText(rec.Code), rec.Body.String()))
		}
	}
}
