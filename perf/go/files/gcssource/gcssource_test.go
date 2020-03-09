package gcssource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
)

const (
	projectName = "test-project"
	topicName   = "test-file-source"
	// TODO(jcgregorio) Make this a public file so we can run the test on the tree.
	testFile = "gs://skia-infra/testdata/perf-files-gcssource-test.json"
)

func setupPubSub(t *testing.T, cfg *config.InstanceConfig) {
	ctx := context.Background()

	// This test presumes the pubsub emulator is running.
	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		require.Fail(t, `Running tests that require a running Cloud PubSub emulator.

Run

	"gcloud beta emulators pubsub start --project=test-project --host-port=localhost:8085"

and then run

  $(gcloud beta emulators pubsub env-init)

to set the environment variables. When done running tests you can unset the env variables:

  $(gcloud beta emulators pubsub env-unset)

`)

	}

	pubSubClient, err := pubsub.NewClient(ctx, projectName)
	require.NoError(t, err)

	// Create the topic if it doesn't exist.
	topic := pubSubClient.Topic(topicName)
	exists, err := topic.Exists(ctx)
	require.NoError(t, err)

	if !exists {
		topic, err = pubSubClient.CreateTopic(ctx, topicName)
		require.NoError(t, err)
	}

	// Create the subscription if it doesn't exists. For some reason we need to
	// do this here when running against the emulator.
	hostname, err := os.Hostname()
	require.NoError(t, err)
	subName := fmt.Sprintf("%s-%s", cfg.IngestionConfig.SourceConfig.Topic, hostname)
	sub := pubSubClient.Subscription(subName)
	exists, err = sub.Exists(ctx)
	require.NoError(t, err)
	if !exists {
		_, err = pubSubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: pubSubClient.Topic(topicName),
		})
		require.NoError(t, err)
	}

	// Publish two messages that looks like the arrival of a file, the first
	// doesn't match Sources and so shouldn't get through.
	b, err := json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "some-random-location/somefile.json",
	})
	require.NoError(t, err)

	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)

	// Wait for the message to be sent.
	_, err = res.Get(ctx)
	require.NoError(t, err)

	// Now publish a good message.
	b, err = json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "testdata/perf-files-gcssource-test.json",
	})
	require.NoError(t, err)

	msg = &pubsub.Message{
		Data: b,
	}
	res = topic.Publish(ctx, msg)

	// Wait for the message to be sent.
	_, err = res.Get(ctx)
	require.NoError(t, err)
}

func TestStart(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	cfg := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: projectName,
				Topic:   topicName,
				Sources: []string{"gs://skia-infra/testdata/"},
			},
		},
	}

	setupPubSub(t, cfg)
	source, err := New(ctx, cfg, true)
	require.NoError(t, err)
	ch, err := source.Start()
	require.NoError(t, err)

	file := <-ch
	assert.Equal(t, testFile, file.Name)
	b, err := ioutil.ReadAll(file.Contents)
	assert.NoError(t, err)
	assert.NoError(t, file.Contents.Close())
	assert.Equal(t, "{\n  \"status\": \"Success\"\n}\n", string(b))

	// A second call to Start should fail.
	_, err = source.Start()
	require.Error(t, err)
}
