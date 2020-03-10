package gcssource

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
)

const (
	// testFile is a real file that we can read.
	testFile = "gs://skia-infra/testdata/perf-files-gcssource-test.json"
)

var (
	// instanceConfig used by all tests.
	instanceConfig = &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: "test-project-" + strconv.FormatInt(rand.Int63(), 10),
				Topic:   "test-topic-" + strconv.FormatInt(rand.Int63(), 10),
				Sources: []string{"gs://skia-infra/testdata/"},
			},
		},
	}
)

func setupPubSubClient(t *testing.T, cfg *config.InstanceConfig) *pubsub.Client {
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

	pubsubClient, err := pubsub.NewClient(ctx, cfg.IngestionConfig.SourceConfig.Project)
	require.NoError(t, err)

	// Create the topic if it doesn't exist.
	topic := pubsubClient.Topic(cfg.IngestionConfig.SourceConfig.Topic)
	exists, err := topic.Exists(ctx)
	require.NoError(t, err)

	if !exists {
		topic, err = pubsubClient.CreateTopic(ctx, cfg.IngestionConfig.SourceConfig.Topic)
		require.NoError(t, err)
	}

	return pubsubClient
}

func sendPubSubMessages(ctx context.Context, t *testing.T, pubsubClient *pubsub.Client, instanceConfig *config.InstanceConfig) {
	topic := pubsubClient.Topic(instanceConfig.IngestionConfig.SourceConfig.Topic)
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

func TestStart_ReceiveOneFileFilterOneFile(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	pubsubClient := setupPubSubClient(t, instanceConfig)

	// Send two events, but only one that is valid.
	sendPubSubMessages(ctx, t, pubsubClient, instanceConfig)

	// Create source and call Start.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)
	ch, err := source.Start()
	require.NoError(t, err)

	// Load the one file sendPubSubMessages should have sent.
	file := <-ch
	assert.Equal(t, testFile, file.Name)
	b, err := ioutil.ReadAll(file.Contents)
	assert.NoError(t, err)
	assert.NoError(t, file.Contents.Close())
	assert.Equal(t, "{\n  \"status\": \"Success\"\n}\n", string(b))
}

func TestStart_SecondCallToStartFails(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	pubsubClient := setupPubSubClient(t, instanceConfig)

	// Create source and call Start.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)

	// A second call to Start should fail.
	_, err = source.Start()
	require.Error(t, err)
}

func TestReceiveSingleEvent_Success(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	_ = setupPubSubClient(t, instanceConfig)

	// Create source.
	source, err := New(ctx, instanceConfig, true)

	// Swap out channel for one we control.
	ch := make(chan file.File, 1)
	source.fileChannel = ch
	require.NoError(t, err)

	// Create an encoded message that points to a real GCS file.
	b, err := json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "testdata/perf-files-gcssource-test.json",
	})
	require.NoError(t, err)
	msg := &pubsub.Message{
		Data: b,
	}

	assert.True(t, source.receiveSingleEvent(ctx, msg))

	// Confirm the correct file.File comes out of the channel.
	file := <-ch
	assert.Equal(t, testFile, file.Name)
	b, err = ioutil.ReadAll(file.Contents)
	assert.NoError(t, err)
	assert.NoError(t, file.Contents.Close())
	assert.Equal(t, "{\n  \"status\": \"Success\"\n}\n", string(b))
}

func TestReceiveSingleEvent_FileDoesntExist(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	_ = setupPubSubClient(t, instanceConfig)

	// Create source.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)

	// Create an encoded message that points to a non-existent GCS file.
	b, err := json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "testdata/some-file-that-doesnt-exist.json",
	})
	require.NoError(t, err)
	msg := &pubsub.Message{
		Data: b,
	}
	assert.False(t, source.receiveSingleEvent(ctx, msg))
}

func TestReceiveSingleEvent_InvalidJSONInMessage(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	_ = setupPubSubClient(t, instanceConfig)

	// Create source.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)

	// Send a message that isn't valid JSON.
	msg := &pubsub.Message{
		Data: []byte("this isn't valid json"),
	}
	assert.True(t, source.receiveSingleEvent(ctx, msg))
}
