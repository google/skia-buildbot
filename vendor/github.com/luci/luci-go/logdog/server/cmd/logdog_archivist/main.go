// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/gcloud/gs"
	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/types"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/server/archivist"
	"github.com/luci/luci-go/logdog/server/service"

	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

var (
	errInvalidConfig = errors.New("invalid configuration")

	// tsTaskProcessingTime measures the amount of time spent processing a single
	// task.
	//
	// The "consumed" field is true if the underlying task was consumed and
	// false if it was not.
	tsTaskProcessingTime = metric.NewCumulativeDistribution("logdog/archivist/task_processing_time_ms",
		"The amount of time (in milliseconds) that a single task takes to process.",
		types.MetricMetadata{Units: types.Milliseconds},
		distribution.DefaultBucketer,
		field.Bool("consumed"))
)

const (
	// subscriptionErrorDelay is the amount of time to sleep after a subscription
	// iterator returns a non-terminal error.
	subscriptionErrorDelay = 10 * time.Second
)

// application is the Archivist application state.
type application struct {
	service.Service
}

// run is the main execution function.
func (a *application) runArchivist(c context.Context) error {
	cfg := a.Config()

	coordCfg, acfg := cfg.GetCoordinator(), cfg.GetArchivist()
	switch {
	case coordCfg == nil:
		fallthrough

	case acfg == nil:
		return errors.New("missing required config: archivist")
	case acfg.GsStagingBucket == "":
		return errors.New("missing required config: archivist.gs_staging_bucket")
	}

	// Initialize Pub/Sub client.
	//
	// We will initialize both an authenticated Client instance and an
	// authenticated Context, since we need the latter for raw ACK deadline
	// updates.
	taskSub := gcps.Subscription(acfg.Subscription)
	if err := taskSub.Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"value":      taskSub,
		}.Errorf(c, "Task subscription did not validate.")
		return errors.New("invalid task subscription name")
	}
	psProject, psSubscriptionName := taskSub.Split()

	psAuth, err := a.Authenticator(c, func(o *auth.Options) {
		o.Scopes = gcps.SubscriberScopes
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get Pub/Sub authenticator.")
		return err
	}

	// Pub/Sub: TokenSource => Client
	psClient, err := pubsub.NewClient(c, psProject, option.WithTokenSource(psAuth.TokenSource()))
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to create Pub/Sub client.")
		return err
	}
	sub := psClient.Subscription(psSubscriptionName)

	// Initialize our Storage.
	st, err := a.IntermediateStorage(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get storage instance.")
		return err
	}
	defer st.Close()

	// Initialize our Google Storage client.
	gsClient, err := a.GSClient(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get Google Storage client.")
		return err
	}
	defer gsClient.Close()

	ar := archivist.Archivist{
		Service:        a.Coordinator(),
		SettingsLoader: a.GetSettingsLoader(acfg),
		Storage:        st,
		GSClient:       gsClient,
	}

	tasks := int(acfg.Tasks)
	if tasks <= 0 {
		tasks = 1
	}

	log.Fields{
		"subscription": taskSub,
		"tasks":        tasks,
	}.Infof(c, "Pulling tasks from Pub/Sub subscription.")
	it, err := sub.Pull(c, pubsub.MaxExtension(gcps.MaxACKDeadline), pubsub.MaxPrefetch(tasks))
	if err != nil {
		log.Fields{
			log.ErrorKey:   err,
			"subscription": taskSub,
		}.Errorf(c, "Failed to create Pub/Sub subscription iterator.")
		return err
	}
	defer it.Stop()

	// Application shutdown will now operate by stopping the Iterator.
	a.SetShutdownFunc(it.Stop)

	// Loop, pulling messages from our iterator and dispatching them.
	parallel.Ignore(parallel.Run(tasks, func(taskC chan<- func() error) {
		for {
			msg, err := it.Next()
			switch err {
			case nil:
				c := log.SetFields(c, log.Fields{
					"messageID": msg.ID,
					"ackID":     msg.AckID,
				})

				// Dispatch an archive handler for this message.
				taskC <- func() error {
					// ACK (or not) the message based on whether our task was consumed.
					deleteTask := false
					defer func() {
						msg.Done(deleteTask)
					}()

					// Time how long task processing takes for metrics.
					startTime := clock.Now(c)
					defer func() {
						duration := clock.Now(c).Sub(startTime)

						if deleteTask {
							log.Fields{
								"duration": duration,
							}.Infof(c, "Task successfully processed; deleting.")
						} else {
							log.Fields{
								"duration": duration,
							}.Infof(c, "Task processing incomplete. Not deleting.")
						}

						// Add to our processing time metric.
						tsTaskProcessingTime.Add(c, duration.Seconds()*1000, deleteTask)
					}()

					task, err := makePubSubArchivistTask(psSubscriptionName, msg)
					if err != nil {
						log.WithError(err).Errorf(c, "Failed to unmarshal archive task from message.")
						deleteTask = true
						return nil
					}

					ar.ArchiveTask(c, task)
					deleteTask = task.consumed

					return nil
				}

			case pubsub.Done, context.Canceled, context.DeadlineExceeded:
				log.Infof(c, "Subscription iterator is finished.")
				return

			default:
				log.WithError(err).Warningf(c, "Subscription iterator returned error. Sleeping...")
				clock.Sleep(c, subscriptionErrorDelay)
				continue
			}
		}
	}))

	log.Debugf(c, "Archivist finished.")
	return nil
}

// GetSettingsLoader is an archivist.SettingsLoader implementation that merges
// global and project-specific settings.
//
// The resulting settings object will be verified by the Archivist.
func (a *application) GetSettingsLoader(acfg *svcconfig.Archivist) archivist.SettingsLoader {
	serviceID := a.ServiceID()

	return func(c context.Context, proj config.ProjectName) (*archivist.Settings, error) {
		// Fold in our project-specific configuration, if valid.
		pcfg, err := a.ProjectConfig(c, proj)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"project":    proj,
			}.Errorf(c, "Failed to fetch project configuration.")
			return nil, err
		}

		indexParam := func(get func(ic *svcconfig.ArchiveIndexConfig) int32) int {
			if ic := pcfg.ArchiveIndexConfig; ic != nil {
				if v := get(ic); v > 0 {
					return int(v)
				}
			}

			if ic := acfg.ArchiveIndexConfig; ic != nil {
				if v := get(ic); v > 0 {
					return int(v)
				}
			}

			return 0
		}

		// Load our base settings.
		//
		// Archival bases are:
		// Staging: gs://<services:gs_staging_bucket>/<project-id>/...
		// Archive: gs://<project:archive_gs_bucket>/<project-id>/...
		st := archivist.Settings{
			GSBase:        gs.MakePath(pcfg.ArchiveGsBucket, "").Concat(serviceID),
			GSStagingBase: gs.MakePath(acfg.GsStagingBucket, "").Concat(serviceID),

			IndexStreamRange: indexParam(func(ic *svcconfig.ArchiveIndexConfig) int32 { return ic.StreamRange }),
			IndexPrefixRange: indexParam(func(ic *svcconfig.ArchiveIndexConfig) int32 { return ic.PrefixRange }),
			IndexByteRange:   indexParam(func(ic *svcconfig.ArchiveIndexConfig) int32 { return ic.ByteRange }),
			AlwaysRender:     (acfg.RenderAllStreams || pcfg.RenderAllStreams),
		}

		// Fold project settings into loaded ones.
		return &st, nil
	}
}

// Entry point.
func main() {
	a := application{
		Service: service.Service{
			Name: "archivist",
		},
	}
	a.Run(context.Background(), a.runArchivist)
}
