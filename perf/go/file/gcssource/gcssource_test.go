package gcssource

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

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

func setupPubSubClient(t *testing.T) (*pubsub.Client, *config.InstanceConfig) {
	ctx := context.Background()

	rand.Seed(time.Now().Unix())
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: "prj-" + strconv.FormatInt(rand.Int63()%10000, 10),
				Topic:   "top-" + strconv.FormatInt(rand.Int63()%10000, 10),
				Sources: []string{"gs://skia-infra/testdata/"},
			},
		},
	}

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

	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project)
	require.NoError(t, err)

	// Create the topic.
	_, err = pubsubClient.CreateTopic(ctx, instanceConfig.IngestionConfig.SourceConfig.Topic)
	assert.NoError(t, err)

	return pubsubClient, instanceConfig
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
	pubsubClient, instanceConfig := setupPubSubClient(t)

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
	_, instanceConfig := setupPubSubClient(t)

	// Create source and call Start.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)
	_, err = source.Start()
	require.NoError(t, err)

	// A second call to Start should fail.
	_, err = source.Start()
	require.Error(t, err)
}

func TestReceiveSingleEvent_Success(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	_, instanceConfig := setupPubSubClient(t)

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
	_, instanceConfig := setupPubSubClient(t)

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
	_, instanceConfig := setupPubSubClient(t)

	// Create source.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)

	// Send a message that isn't valid JSON.
	msg := &pubsub.Message{
		Data: []byte("this isn't valid json"),
	}
	assert.True(t, source.receiveSingleEvent(ctx, msg))
}
