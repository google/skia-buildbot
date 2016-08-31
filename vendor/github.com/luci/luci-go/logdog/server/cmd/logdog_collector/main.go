// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/types"
	"github.com/luci/luci-go/logdog/server/collector"
	"github.com/luci/luci-go/logdog/server/collector/coordinator"
	"github.com/luci/luci-go/logdog/server/service"
	"golang.org/x/net/context"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
)

var (
	errInvalidConfig = errors.New("invalid configuration")
)

const (
	pubsubPullErrorDelay = 10 * time.Second
)

// Metrics.
var (
	// tsPubsubCount counts the number of Pub/Sub messages processed by the
	// Archivist.
	//
	// Result tracks the outcome of each message, either "success", "failure", or
	// "transient_failure".
	tsPubsubCount = metric.NewCounter("logdog/collector/subscription/count",
		"The number of Pub/Sub messages pulled.",
		types.MetricMetadata{},
		field.String("result"))

	// tsTaskProcessingTime tracks the amount of time a single subscription
	// message takes to process, in milliseconds.
	tsTaskProcessingTime = metric.NewCumulativeDistribution("logdog/collector/subscription/processing_time_ms",
		"Amount of time in milliseconds that a single Pub/Sub message takes to process.",
		types.MetricMetadata{types.Milliseconds},
		distribution.DefaultBucketer)
)

// application is the Collector application state.
type application struct {
	service.Service
}

// run is the main execution function.
func (a *application) runCollector(c context.Context) error {
	cfg := a.Config()
	ccfg := cfg.GetCollector()
	if ccfg == nil {
		return errors.New("no collector configuration")
	}

	pscfg := cfg.GetTransport().GetPubsub()
	if pscfg == nil {
		return errors.New("missing Pub/Sub configuration")
	}

	// Our Subscription must be a valid one.
	sub := gcps.NewSubscription(pscfg.Project, pscfg.Subscription)
	if err := sub.Validate(); err != nil {
		return fmt.Errorf("invalid Pub/Sub subscription %q: %v", sub, err)
	}

	// New PubSub instance with the authenticated client.
	psAuth, err := a.Authenticator(c, func(o *auth.Options) {
		o.Scopes = gcps.SubscriberScopes
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Pub/Sub token source.")
		return err
	}

	psClient, err := pubsub.NewClient(c, pscfg.Project, option.WithTokenSource(psAuth.TokenSource()))
	if err != nil {
		log.Fields{
			log.ErrorKey:   err,
			"subscription": sub,
		}.Errorf(c, "Failed to create Pub/Sub client.")
		return err
	}

	psSub := psClient.Subscription(pscfg.Subscription)
	exists, err := psSub.Exists(c)
	if err != nil {
		log.Fields{
			log.ErrorKey:   err,
			"subscription": sub,
		}.Errorf(c, "Could not confirm Pub/Sub subscription.")
		return errInvalidConfig
	}
	if !exists {
		log.Fields{
			"subscription": sub,
		}.Errorf(c, "Subscription does not exist.")
		return errInvalidConfig
	}
	log.Fields{
		"subscription": sub,
	}.Infof(c, "Successfully validated Pub/Sub subscription.")

	st, err := a.IntermediateStorage(c)
	if err != nil {
		return err
	}
	defer st.Close()

	// Initialize our Collector service object using a caching Coordinator
	// interface.
	coord := coordinator.NewCoordinator(a.Coordinator())
	coord = coordinator.NewCache(coord, int(ccfg.StateCacheSize), ccfg.StateCacheExpiration.Duration())

	coll := collector.Collector{
		Coordinator:       coord,
		Storage:           st,
		MaxMessageWorkers: int(ccfg.MaxMessageWorkers),
	}
	defer coll.Close()

	// Execute our main subscription pull loop. It will run until the supplied
	// Context is cancelled.
	psIterator, err := psSub.Pull(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Pub/Sub iterator.")
		return err
	}
	defer psIterator.Stop()

	// Application shutdown will now operate by cancelling the Collector's
	// shutdown Context.
	a.SetShutdownFunc(psIterator.Stop)

	parallel.Ignore(parallel.Run(int(ccfg.MaxConcurrentMessages), func(taskC chan<- func() error) {
		// Loop until shut down.
		for {
			msg, err := psIterator.Next()
			switch err {
			case nil:
				taskC <- func() error {
					c := log.SetField(c, "messageID", msg.ID)
					msg.Done(a.processMessage(c, &coll, msg))
					return nil
				}

			case pubsub.Done, context.Canceled, context.DeadlineExceeded:
				return

			default:
				log.Fields{
					log.ErrorKey: err,
					"delay":      pubsubPullErrorDelay,
				}.Errorf(c, "Failed to fetch Pub/Sub message, retry after delay...")
				clock.Sleep(c, pubsubPullErrorDelay)
			}
		}
	}))

	log.Debugf(c, "Collector finished.")
	return nil
}

// processMessage returns true if the message should be ACK'd (deleted from
// Pub/Sub) or false if the message should not be ACK'd.
func (a *application) processMessage(c context.Context, coll *collector.Collector, msg *pubsub.Message) bool {
	log.Fields{
		"ackID": msg.AckID,
		"size":  len(msg.Data),
	}.Infof(c, "Received Pub/Sub Message.")

	startTime := clock.Now(c)
	err := coll.Process(c, msg.Data)
	duration := clock.Now(c).Sub(startTime)

	// We track processing time in milliseconds.
	tsTaskProcessingTime.Add(c, duration.Seconds()*1000)

	switch {
	case errors.IsTransient(err):
		// Do not consume
		log.Fields{
			log.ErrorKey: err,
			"duration":   duration,
		}.Warningf(c, "TRANSIENT error ingesting Pub/Sub message.")
		tsPubsubCount.Add(c, 1, "transient_failure")
		return false

	case err == nil:
		log.Fields{
			"ackID":    msg.AckID,
			"size":     len(msg.Data),
			"duration": duration,
		}.Infof(c, "Message successfully processed; ACKing.")
		tsPubsubCount.Add(c, 1, "success")
		return true

	default:
		log.Fields{
			log.ErrorKey: err,
			"ackID":      msg.AckID,
			"size":       len(msg.Data),
			"duration":   duration,
		}.Errorf(c, "Non-transient error ingesting Pub/Sub message; ACKing.")
		tsPubsubCount.Add(c, 1, "failure")
		return true
	}
}

// Entry point.
func main() {
	a := application{
		Service: service.Service{
			Name: "collector",
		},
	}
	a.Run(context.Background(), a.runCollector)
}
