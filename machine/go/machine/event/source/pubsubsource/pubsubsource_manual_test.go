// Package pubsubsource implements source.Source using Google Cloud PubSub.
package pubsubsource

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
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

func setupPubSubClient(t *testing.T) (context.Context, *pubsub.Client, config.InstanceConfig) {
	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	instanceConfig := config.InstanceConfig{
		Source: config.Source{
			Project: "test-project",
			Topic:   fmt.Sprintf("events-%d", rand.Int63()),
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

	return ctx, pubsubClient, instanceConfig
}

func sendPubSubMessages(ctx context.Context, t *testing.T, pubsubClient *pubsub.Client, instanceConfig config.InstanceConfig) {
	topic := pubsubClient.Topic(instanceConfig.Source.Topic)

	msg := &pubsub.Message{
		Data: []byte("This isn't valid JSON."),
	}
	res := topic.Publish(ctx, msg)

	// Wait for the message to be sent.
	_, err := res.Get(ctx)
	require.NoError(t, err)

	// Now publish a good message.
	b, err := json.Marshal(machine.Event{
		Host: machine.Host{
			Name: "skia-rpi2-rack4-shelf1-001",
		},
	})
	require.NoError(t, err)

	msg = &pubsub.Message{
		Data: b,
	}
	res = topic.Publish(ctx, msg)

	// Wait for the message to be sent.
	_, err = res.Get(ctx)
	require.NoError(t, err)
	topic.Stop()
}

func TestStart(t *testing.T) {
	unittest.RequiresPubSubEmulator(t)

	ctx, pubsubClient, instanceConfig := setupPubSubClient(t)
	ctx, cancel := context.WithCancel(ctx)
	// Create source and call Start.
	source, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)
	ch, err := source.Start(ctx)
	require.NoError(t, err)

	sendPubSubMessages(ctx, t, pubsubClient, instanceConfig)

	// Load the one file sendPubSubMessages should have sent.
	event := <-ch

	// Now cancel the context and wait for channel to close.
	cancel()
	for range ch {
	}

	assert.Equal(t, "skia-rpi2-rack4-shelf1-001", event.Host.Name)
	assert.Equal(t, int64(2), source.eventsReceivedCounter.Get())
	assert.Equal(t, int64(1), source.eventsFailedToParseCounter.Get())
	assert.NoError(t, pubsubClient.Close())
}

func TestStart_SecondCallToStartFails(t *testing.T) {
	unittest.RequiresPubSubEmulator(t)

	ctx, pubsubClient, instanceConfig := setupPubSubClient(t)
	// Create source and call Start.
	source, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)
	_, err = source.Start(ctx)
	require.NoError(t, err)

	_, err = source.Start(ctx)
	require.Error(t, err)
	assert.NoError(t, pubsubClient.Close())
}
