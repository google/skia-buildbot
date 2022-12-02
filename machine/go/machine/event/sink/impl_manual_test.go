package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

func setupPubSubClient(t *testing.T) (context.Context, *pubsub.Client, *pubsub.Subscription, config.InstanceConfig) {
	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	instanceConfig := config.InstanceConfig{
		Source: config.Source{
			Project: "test-project",
			Topic:   fmt.Sprintf("sink-%d", rand.Int63()),
		},
	}

	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
	require.NoError(t, err)
	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.Source.Project, option.WithTokenSource(ts))
	require.NoError(t, err)

	// Create the topic.
	topic := pubsubClient.Topic(instanceConfig.Source.Topic)
	ok, err := topic.Exists(ctx)
	require.NoError(t, err)
	if !ok {
		topic, err = pubsubClient.CreateTopic(ctx, instanceConfig.Source.Topic)
	}
	topic.Stop()
	assert.NoError(t, err)

	sub, err := pubsubClient.CreateSubscription(ctx, instanceConfig.Source.Topic, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	return ctx, pubsubClient, sub, instanceConfig
}

func TestSink(t *testing.T) {
	gcp_emulator.RequirePubSub(t)
	ctx, _, sub, instanceConfig := setupPubSubClient(t)

	// Create new sink.
	s, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)

	// Create event to send.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android: machine.Android{
			GetProp: `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`,
		},
		Host: machine.Host{
			Name: "my-machine-id",
		},
	}

	// Send the event.
	err = s.Send(ctx, event)
	require.NoError(t, err)

	// Confirm that the event was sent correctly.
	called := false
	cancellableCtx, cancel := context.WithCancel(ctx)
	err = sub.Receive(cancellableCtx, func(ctx context.Context, m *pubsub.Message) {
		// cancel so sub.Receive returns.
		cancel()
		called = true
		m.Ack()
		var receivedEvent machine.Event
		err := json.Unmarshal(m.Data, &receivedEvent)
		require.NoError(t, err)
		assert.Equal(t, receivedEvent, event)

	})
	require.NoError(t, err)
	assert.True(t, called)
}
