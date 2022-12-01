// Package pubsubsource implements source.Source using Google Cloud PubSub.
package pubsubsource

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/event/source"
	"go.skia.org/infra/machine/go/machineserver/config"
)

const (
	machineEventChannelSize = 100

	// maxParallelReceives is the number of Go routines we want to run.
	maxParallelReceives = 10

	// subscriptionSuffix is the name we append to a topic name to build a
	// subscription name.
	subscriptionSuffix = "-prod"
)

// Source implements source.Source.
type Source struct {
	sub                        *pubsub.Subscription
	started                    bool // Start should only be called once.
	eventsReceivedCounter      metrics2.Counter
	eventsFailedToParseCounter metrics2.Counter
}

// New returns a new *Source.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*Source, error) {
	sub, err := sub.New(ctx, local, instanceConfig.Source.Project, instanceConfig.Source.Topic, maxParallelReceives)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	sklog.Infof("pubsub Source started for topic: %q", instanceConfig.Source.Topic)

	return &Source{
		sub:                        sub,
		eventsReceivedCounter:      metrics2.GetCounter(source.ReceiveSuccessMetricName, map[string]string{"type": "pubsub"}),
		eventsFailedToParseCounter: metrics2.GetCounter(source.ReceiveFailureMetricName, map[string]string{"type": "pubsub"}),
	}, nil
}

// Start implements source.Source.
func (s *Source) Start(ctx context.Context) (<-chan machine.Event, error) {
	if s.started {
		return nil, skerr.Fmt("Start can only be called once.")
	}
	s.started = true
	ch := make(chan machine.Event, machineEventChannelSize)
	go func() {
		for {
			if ctx.Err() != nil {
				sklog.Errorf("pubsub source closing!: %s", ctx.Err())
				close(ch)
				return
			}
			err := s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				s.eventsReceivedCounter.Inc(1)
				msg.Ack()
				var event machine.Event
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					sklog.Errorf("Received invalid pubsub event data: %s", err)
					s.eventsFailedToParseCounter.Inc(1)
					return
				}
				ch <- event
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()
	return ch, nil
}

// Afirm that we implement the interface.
var _ source.Source = (*Source)(nil)
