package sink

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
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

	ts, err := auth.NewDefaultTokenSource(true, pubsub.ScopePubSub)
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

func TestSink(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresPubSubEmulator(t)
	ctx, _, instanceConfig := setupPubSubClient(t)
	s, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)

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
	err = s.Send(ctx, event)
	require.NoError(t, err)
}
