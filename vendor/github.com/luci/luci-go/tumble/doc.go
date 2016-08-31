// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package tumble is a distributed multi-stage transaction processor for
// appengine.
//
// What it is
//
// Tumble allows you to make multi-entity-group transaction chains, even when
// you need to affect more than the number of entities allowed by appengine
// (currently capped at 25 entity groups). These chains can be transactionally
// started from a single entity group, and will process 'in the background'.
// Tumble guarantees that once a transaction chain starts, it will eventually
// complete, though it makes no guarantees of how long that might take.
//
// This can be used for doing very large-scale fan-out, and also for large-scale
// fan-in.
//
// How it works
//
// An app using tumble declares one or more Mutation object. These objects
// are responsible for enacting a single link in the transaction chain, and
// may affect entities within a single entity group. Mutations must be
// idempotent (they will occasionally be run more than once). Mutations
// primarially implement a RollForward method which transactionally manipulates
// an entity, and then returns zero or more Mutations (which may be for other
// entities). Tumble's task queues and/or cron job (see Setup), will eventually
// pick up these new Mutations and process them, possibly introducing more
// Mutations, etc.
//
// When the app wants to begin a transaction chain, it uses
// tumble.EnterTransaction, allows the app to transactionally manipulate the
// starting entity, and also return one or more Mutation objects. If
// the transaction is successful, EnterTransaction will also fire off any
// necessary taskqueue tasks to process the new mutations in the background.
//
// When the transaction is committed, it's committed along with all the
// Mutations it produced. Either they're all committed successfully (and so
// the tumble transaction chain is started), or none of them are committed.
//
// Required Setup
//
// There are a couple prerequisites for using tumble.
//
// 1. You must register the tumble routes in your appengine app. You can do this
// like:
//
//   import (
//     "net/http"
//
//     "github.com/julienschmidt/httprouter"
//     "github.com/luci/luci-go/appengine/gaemiddleware"
//     "github.com/luci/luci-go/tumble"
//   )
//
//   var tumbleService = tumble.DefaultConfig()
//
//   def init() {
//     router := httprouter.New()
//     tumbleService.InstallHandlers(router, gaemiddleware.BaseProd())
//     http.Handle("/", router)
//   }
//
// Don't forget to add these to app.yaml (and/or dispatch.yaml). Additionally,
// make sure to add them with `login: admin`, as they should never be accessed
// from non-backend processes.
//
// 2. You must add the following index to your index.yaml:
//
//   - kind: tumble.Mutation
//     properties:
//     - name: ExpandedShard
//     - name: TargetRoot
//
// 2a. If you enable DelayedMutations in your configuration, you must also add
//   - kind: tumble.Mutation
//     properties:
//     - name: TargetRoot
//     - name: ProcessAfter
//
// 3. You must add a new taskqueue for tumble (example parameters):
//
//   - name: tumble  # NOTE: name must match the name in the tumble.Config.
//     rate: 32/s
//     bucket_size: 32
//     retry_parameters:
//       task_age_limit: 2m   # aggressive task age pruning is desirable
//       min_backoff_seconds: 2
//       max_backoff_seconds: 6
//       max_doublings: 7     # tops out at 2**(6 - 1) * 2 == 128 sec
//
// 4. All Mutation implementations must be registered at init() time using
// tumble.Register((*MyMutation)(nil)).
//
// 5. You must remember to add tumbleService to all of your handlers'
// contexts with tumble.Use(ctx, tumbleService). This last step is not
// necessary if you use all of the default configuration for tumble.
//
// Optional Setup
//
// You may choose to add a new cron entry. This prevents work from slipping
// through the cracks. If your app has constant tumble throughput and good key
// distribution, this is not necessary.
//
//   - description: tumble fire_all_tasks invocation
//     url: /internal/tumble/fire_all_tasks  # NOTE: must match tumble.Config.FireAllTasksURL()
//     schedule: every 5 minutes             # maximium task latency you can tolerate.
package tumble
