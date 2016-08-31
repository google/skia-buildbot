// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/clock"
	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	"github.com/luci/luci-go/common/iotools"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/monitor"
	"github.com/luci/luci-go/common/tsmon/store"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"
)

// State holds the configuration of the tsmon library for GAE.
//
// Define it as a global variable and inject it in the request contexts using
// State.Middleware().
//
// It will initialize itself from the tsmon state in the passed context on
// a first use, mutating it along the way. Assumes caller is consistently using
// contexts configured with exact same tsmon state (in a vast majority of cases
// it would be global tsmon state that corresponds to context.Background, but
// unit tests may provide its own state).
//
// Will panic if it detects that caller has changed tsmon state in the context
// between the requests.
type State struct {
	lock sync.RWMutex

	state        *tsmon.State
	lastSettings tsmonSettings

	flushingNow bool
	lastFlushed time.Time

	// testingMonitor is mocked monitor used in unit tests.
	testingMonitor monitor.Monitor
	// testingSettings if not nil are used in unit tests.
	testingSettings *tsmonSettings
}

// responseWriter wraps a given http.ResponseWriter, records its
// status code and response size.
type responseWriter struct {
	http.ResponseWriter
	writer iotools.CountingWriter
	status int
}

func (rw *responseWriter) Write(buf []byte) (int, error) { return rw.writer.Write(buf) }

func (rw *responseWriter) Size() int64 { return rw.writer.Count }

func (rw *responseWriter) Status() int { return rw.status }

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func newResponseWriter(rw http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: rw,
		writer:         iotools.CountingWriter{Writer: rw},
		status:         http.StatusOK,
	}
}

// Middleware is a middleware that must be inserted into the middleware
// chain to enable tsmon metrics to be send on AppEngine.
func (s *State) Middleware(c *router.Context, next router.Handler) {
	state, settings := s.checkSettings(c.Context)
	if settings.Enabled {
		started := clock.Now(c.Context)
		userAgent, ok := c.Request.Header["User-Agent"]
		if !ok || len(userAgent) == 0 {
			userAgent = []string{"Unknown"}
		}
		ctx := c.Context
		contentLength := c.Request.ContentLength
		nrw := newResponseWriter(c.Writer)
		c.Writer = nrw
		defer func() {
			dur := clock.Now(ctx).Sub(started)
			metric.UpdateServerMetrics(ctx, "/", nrw.Status(), dur,
				contentLength, nrw.Size(), userAgent[0])
		}()
		next(c)
		s.flushIfNeeded(ctx, state, settings)
	} else {
		next(c)
	}
}

// checkSettings fetches tsmon settings and initializes, reinitializes or
// deinitializes tsmon, as needed.
//
// Returns current tsmon state and settings. Panics if the context is using
// unexpected tsmon state.
func (s *State) checkSettings(c context.Context) (*tsmon.State, *tsmonSettings) {
	state := tsmon.GetState(c)

	var settings tsmonSettings
	if s.testingSettings != nil {
		settings = *s.testingSettings
	} else {
		settings = fetchCachedSettings(c)
	}

	// Read the values used when handling previous request. In most cases they
	// are identical to current ones and we can skip grabbing a heavier write
	// lock.
	s.lock.RLock()
	if s.state == state && s.lastSettings == settings {
		s.lock.RUnlock()
		return state, &settings
	}
	s.lock.RUnlock()

	// 'settings' or 'state' has changed. Reinitialize tsmon as needed under
	// the write lock.
	s.lock.Lock()
	defer s.lock.Unlock()

	// First call to 'checkSettings' ever?
	if s.state == nil {
		s.state = state
		s.state.M = monitor.NewNilMonitor() // doFlush uses its own monitor
	} else if state != s.state {
		panic("tsmon state in the context was unexpectedly changed between requests")
	}

	switch {
	case !bool(s.lastSettings.Enabled) && bool(settings.Enabled):
		s.enableTsMon(c)
	case bool(s.lastSettings.Enabled) && !bool(settings.Enabled):
		s.disableTsMon(c)
	}
	s.lastSettings = settings

	return state, &settings
}

// enableTsMon puts in-memory metrics store in the context's tsmon state.
//
// Called with 's.lock' locked.
func (s *State) enableTsMon(c context.Context) {
	i := info.Get(c)
	s.state.SetStore(store.NewInMemory(&target.Task{
		DataCenter:  proto.String(targetDataCenter),
		ServiceName: proto.String(i.AppID()),
		JobName:     proto.String(i.ModuleName()),
		HostName:    proto.String(strings.SplitN(i.VersionID(), ".", 2)[0]),
		TaskNum:     proto.Int32(-1),
	}))

	// Request the flush to be executed ASAP, so it registers (or updates)
	// 'Instance' entity in the datastore.
	s.lastFlushed = time.Time{}
}

// disableTsMon puts nil metrics store in the context's tsmon state.
//
// Called with 's.lock' locked.
func (s *State) disableTsMon(c context.Context) {
	s.state.SetStore(store.NewNilStore())
}

// flushIfNeeded periodically flushes the accumulated metrics.
//
// It skips the flush if some other goroutine is already flushing. Logs errors.
func (s *State) flushIfNeeded(c context.Context, state *tsmon.State, settings *tsmonSettings) {
	now := clock.Now(c)
	flushTime := now.Add(-time.Duration(settings.FlushIntervalSec) * time.Second)

	// Most of the time the flush is not needed and we can get away with
	// lightweight RLock.
	s.lock.RLock()
	skip := s.flushingNow || s.lastFlushed.After(flushTime)
	s.lock.RUnlock()
	if skip {
		return
	}

	// Need to flush. Update flushingNow. Redo the check under write lock.
	s.lock.Lock()
	skip = s.flushingNow || s.lastFlushed.After(flushTime)
	if !skip {
		s.flushingNow = true
	}
	s.lock.Unlock()
	if skip {
		return
	}

	// Report per-process statistic, like memory stats.
	collectProcessMetrics(c, settings)

	// Unset 'flushingNow' no matter what (even on panics). Update 'lastFlushed'
	// only on successful flush. Unsuccessful flush thus will be retried ASAP.
	success := false
	defer func() {
		s.lock.Lock()
		s.flushingNow = false
		if success {
			s.lastFlushed = now
		}
		s.lock.Unlock()
	}()

	// The flush must be fast. Limit it with by some timeout.
	c, _ = clock.WithTimeout(c, flushTimeout)
	if err := s.updateInstanceEntityAndFlush(c, state, settings); err != nil {
		logging.Errorf(c, "Failed to flush tsmon metrics: %s", err)
	} else {
		success = true
	}
}

// updateInstanceEntityAndFlush waits for instance to get assigned a task number and
// flushes the metrics.
func (s *State) updateInstanceEntityAndFlush(c context.Context, state *tsmon.State, settings *tsmonSettings) error {
	c = info.Get(c).MustNamespace(instanceNamespace)

	defTarget := state.S.DefaultTarget()
	task, ok := defTarget.(*target.Task)
	if !ok {
		return fmt.Errorf("default tsmon target is not a Task (%T): %v", defTarget, defTarget)
	}

	now := clock.Now(c)

	// Grab TaskNum assigned to the currently running instance. It is updated by
	// a dedicated cron job, see housekeepingHandler in handler.go.
	entity, err := getOrCreateInstanceEntity(c)
	if err != nil {
		return fmt.Errorf("failed to get instance entity - %s", err)
	}

	// Don't do the flush if we still haven't get a task number.
	if entity.TaskNum < 0 {
		if *task.TaskNum >= 0 {
			// We used to have a task number but we don't any more (we were inactive
			// for too long), so clear our state.
			logging.Warningf(c, "Instance %s got purged from Datastore, but is still alive. "+
				"Clearing cumulative metrics", info.Get(c).InstanceID())
			state.ResetCumulativeMetrics(c)
		}
		task.TaskNum = proto.Int32(-1)
		state.S.SetDefaultTarget(task)

		// Start complaining if we haven't been given a task number after some time.
		shouldHaveTaskNumBy := entity.LastUpdated.Add(instanceExpectedToHaveTaskNum)
		if shouldHaveTaskNumBy.Before(now) {
			logging.Warningf(c, "Instance %s is %s old with no task_num.",
				info.Get(c).InstanceID(), now.Sub(shouldHaveTaskNumBy).String())
		}

		// Non-assigned TaskNum is expected situation, pretend the flush succeeded,
		// so that next try happens after regular flush period, not right away.
		return nil
	}

	task.TaskNum = proto.Int32(int32(entity.TaskNum))
	state.S.SetDefaultTarget(task)

	// Refresh 'entity.LastUpdated'. Ignore errors here since the flush is
	// happening already even this operation fails.
	putDone := make(chan struct{})
	go func() {
		defer close(putDone)
		if err := refreshLastUpdatedTime(c, now); err != nil {
			logging.Errorf(c, "Failed to update instance entity: %s", err)
		}
	}()

	ret := s.doFlush(c, state, settings)

	<-putDone
	return ret
}

// doFlush actually sends the metrics to the monitor.
func (s *State) doFlush(c context.Context, state *tsmon.State, settings *tsmonSettings) error {
	var mon monitor.Monitor

	if s.testingMonitor != nil {
		mon = s.testingMonitor
	} else if info.Get(c).IsDevAppServer() || settings.PubsubProject == "" || settings.PubsubTopic == "" {
		mon = monitor.NewDebugMonitor("")
	} else {
		topic := gcps.NewTopic(settings.PubsubProject, settings.PubsubTopic)
		logging.Infof(c, "Sending metrics to %s", topic)

		// Create an HTTP client with the default appengine service account. The
		// client is bound to the context and inherits its deadline.
		t, err := auth.GetRPCTransport(c, auth.AsSelf, auth.WithScopes(gcps.PublisherScopes...))
		if err != nil {
			return err
		}
		if mon, err = monitor.NewPubsubMonitor(c, &http.Client{Transport: t}, topic); err != nil {
			return err
		}
	}

	if err := state.Flush(c, mon); err != nil {
		return err
	}

	state.ResetGlobalCallbackMetrics(c)
	return nil
}
