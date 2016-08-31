// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/server/router"
	"golang.org/x/net/context"
)

const transientHTTPHeader = "X-LUCI-Tumble-Transient"

// Service is an instance of a Tumble service. It installs its handlers into an
// HTTP router and services Tumble request tasks.
type Service struct {
	// Namespaces is a function that returns the datastore namespaces that Tumble
	// will poll.
	//
	// If nil, Tumble will be executed against all namespaces registered in the
	// datastore.
	Namespaces func(context.Context) ([]string, error)
}

// InstallHandlers installs http handlers.
//
// 'base' is usually gaemiddleware.BaseProd(), but can also be its derivative
// if something else it needed in the context.
func (s *Service) InstallHandlers(r *router.Router, base router.MiddlewareChain) {
	// GET so that this can be invoked from cron
	r.GET(fireAllTasksURL, base.Extend(gaemiddleware.RequireCron), s.FireAllTasksHandler)
	r.POST(processShardPattern, base.Extend(gaemiddleware.RequireTaskQueue(baseName)),
		s.ProcessShardHandler)
}

// FireAllTasksHandler is an HTTP handler that expects `logging` and `luci/gae`
// services to be installed into the context.
//
// FireAllTasksHandler verifies that it was called within an Appengine Cron
// request, and then invokes the FireAllTasks function.
func (s *Service) FireAllTasksHandler(c *router.Context) {
	if err := s.FireAllTasks(c.Context); err != nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(c.Writer, "fire_all_tasks failed: %s", err)
	} else {
		c.Writer.Write([]byte("ok"))
	}
}

// FireAllTasks searches for work in all namespaces, and fires off a process
// task for any shards it finds that have at least one Mutation present to
// ensure that no work languishes forever. This may not be needed in
// a constantly-loaded system with good tumble key distribution.
func (s *Service) FireAllTasks(c context.Context) error {
	cfg := getConfig(c)
	shards := make(map[taskShard]struct{}, cfg.NumShards)
	shardsLock := &sync.RWMutex{}

	nspaces, err := s.getNamespaces(c, cfg)
	if err != nil {
		return err
	}

	err = parallel.WorkPool(cfg.NumGoroutines, func(ch chan<- func() error) {
		// Since shards are cross-namespace, missingShards represents the total
		// maximum set of shards that this cron job can trigger. Once we find work
		// that needs to be done on a particular shard, that shard is removed from
		// this map, and we no longer probe the rest of the namespaces for it.
		missingShards := make(map[taskShard]struct{}, cfg.NumShards)
		for i := uint64(0); i < cfg.NumShards; i++ {
			missingShards[taskShard{i, minTS}] = struct{}{}
		}

		for _, ns := range nspaces {
			c := c
			ds := datastore.Get(c)
			if ns != "" {
				c = info.Get(c).MustNamespace(ns)
				ds = datastore.Get(c)
			}

			for shrd := range missingShards {
				shardsLock.RLock()
				_, has := shards[shrd]
				shardsLock.RUnlock()
				if has {
					delete(missingShards, shrd)
					if len(missingShards) == 0 {
						return
					}
					continue
				}

				c := c
				ds := ds
				shrd := shrd
				ch <- func() error {
					shardsLock.RLock()
					_, has := shards[shrd]
					shardsLock.RUnlock()
					if has {
						return nil
					}

					amt, err := ds.Count(processShardQuery(c, cfg, shrd.shard).Limit(1))
					if err != nil {
						logging.Fields{
							logging.ErrorKey: err,
							"shard":          shrd.shard,
						}.Errorf(c, "error in query")
						return err
					}
					if amt == 0 {
						return nil
					}
					logging.Fields{
						"shard": shrd.shard,
					}.Infof(c, "triggering")

					shardsLock.Lock()
					shards[shrd] = struct{}{}
					shardsLock.Unlock()
					return nil
				}
			}
		}
	})
	if err != nil {
		logging.WithError(err).Errorf(c, "got errors while scanning")
		if len(shards) == 0 {
			return err
		}
	}

	if !fireTasks(c, cfg, shards) {
		err = errors.New("unable to fire tasks")
	}

	return err
}

func (s *Service) getNamespaces(c context.Context, cfg *Config) (namespaces []string, err error) {
	// Get the set of namespaces to handle.
	if cfg.Namespaced {
		nsFn := s.Namespaces
		if nsFn == nil {
			nsFn = getDatastoreNamespaces
		}
		namespaces, err = nsFn(c)
		if err != nil {
			logging.WithError(err).Errorf(c, "Failed to enumerate namespaces.")
			return
		}
	} else {
		// Namespacing is disabled, use a single empty string. Process will
		// interpret this as a signal to not use namesapces.
		namespaces = []string{""}
	}
	return
}

// ProcessShardHandler is an HTTP handler that expects `logging` and `luci/gae`
// services to be installed into the context.
//
// ProcessShardHandler verifies that its being run as a taskqueue task and that
// the following parameters exist and are well-formed:
//   * timestamp: decimal-encoded UNIX/UTC timestamp in seconds.
//   * shard_id: decimal-encoded shard identifier.
//
// ProcessShardHandler then invokes ProcessShard with the parsed parameters.
func (s *Service) ProcessShardHandler(ctx *router.Context) {
	c, rw, p := ctx.Context, ctx.Writer, ctx.Params

	tstampStr := p.ByName("timestamp")
	sidStr := p.ByName("shard_id")

	tstamp, err := strconv.ParseInt(tstampStr, 10, 64)
	if err != nil {
		logging.Errorf(c, "bad timestamp %q", tstampStr)
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, "bad timestamp")
		return
	}

	sid, err := strconv.ParseUint(sidStr, 10, 64)
	if err != nil {
		logging.Errorf(c, "bad shardID %q", tstampStr)
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, "bad shardID")
		return
	}

	cfg := getConfig(c)

	// Get the set of namespaces to handle.
	namespaces, err := s.getNamespaces(c, cfg)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = processShard(c, cfg, namespaces, time.Unix(tstamp, 0).UTC(), sid)
	if err != nil {
		logging.Errorf(c, "failure! %s", err)

		if errors.IsTransient(err) {
			rw.Header().Add(transientHTTPHeader, "true")
		}
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "error: %s", err)
	} else {
		rw.Write([]byte("ok"))
	}
}

// getDatastoreNamespaces returns a list of all of the namespaces in the
// datastore.
//
// This is done by issuing a datastore query for kind "__namespace__". The
// resulting keys will have IDs for the namespaces, namely:
//	- The default namespace will have integer ID 1. We ignore this because if
//	  we're using tumble with namespaces, we don't process the default
//	  namespace.
//
//	- Other namespaces will have string IDs.
func getDatastoreNamespaces(c context.Context) ([]string, error) {
	q := datastore.NewQuery("__namespace__").KeysOnly(true)

	// Query our datastore for the full set of namespaces.
	var namespaceKeys []*datastore.Key
	if err := datastore.Get(c).GetAll(q, &namespaceKeys); err != nil {
		logging.WithError(err).Errorf(c, "Failed to execute namespace query.")
		return nil, err
	}

	namespaces := make([]string, 0, len(namespaceKeys))
	for _, nk := range namespaceKeys {
		if ns := nk.StringID(); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces, nil
}
