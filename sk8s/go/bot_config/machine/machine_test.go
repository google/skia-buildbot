// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/bot_config/swarming"
	"google.golang.org/api/option"
)

func setupConfig(t *testing.T) (context.Context, *pubsub.Topic, config.InstanceConfig) {
	require.NotEmpty(t, os.Getenv("FIRESTORE_EMULATOR_HOST"), "This test requires the firestore emulator.")
	unittest.RequiresPubSubEmulator(t)

	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	instanceConfig := config.InstanceConfig{
		Source: config.Source{
			Project: "test-project",
			Topic:   fmt.Sprintf("events-%d", rand.Int63()),
		},
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%d", rand.Int63()),
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

	return ctx, topic, instanceConfig
}

func TestStart(t *testing.T) {
	unittest.ManualTest(t)
	ctx, _, instanceConfig := setupConfig(t)

	oldVar := os.Getenv(swarming.SwarmingBotIDEnvVar)
	os.Setenv(swarming.SwarmingBotIDEnvVar, "my-test-bot-001")
	defer func() {
		os.Setenv(swarming.SwarmingBotIDEnvVar, oldVar)
	}()

	m, err := New(ctx, true, instanceConfig)
	require.NoError(t, err)

	assert.Equal(t, "my-test-bot-001", m.machineID)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.Start(ctx)
}
