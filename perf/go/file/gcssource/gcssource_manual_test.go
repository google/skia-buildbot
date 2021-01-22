package gcssource

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"google.golang.org/api/option"
)

const (
	// testFile is a real file that we can read.
	testFile = "gs://skia-infra/testdata/perf-files-gcssource-test.json"
)

func setupPubSubClient(t *testing.T) (*pubsub.Client, *config.InstanceConfig) {
	unittest.RequiresPubSubEmulator(t)
	ctx := context.Background()

	rand.Seed(time.Now().Unix())
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: "skia-public",
				Topic:   "perf-file-source-test",
				Sources: []string{"gs://skia-infra/testdata/"},
			},
		},
	}

	ts, err := auth.NewDefaultTokenSource(true, pubsub.ScopePubSub)
	require.NoError(t, err)
	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	require.NoError(t, err)

	// Create the topic.
	topic := pubsubClient.Topic(instanceConfig.IngestionConfig.SourceConfig.Topic)
	ok, err := topic.Exists(ctx)
	require.NoError(t, err)
	if !ok {
		topic, err = pubsubClient.CreateTopic(ctx, instanceConfig.IngestionConfig.SourceConfig.Topic)
	}
	topic.Stop()
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

func sendTwoGoodPubSubMessages(ctx context.Context, t *testing.T, pubsubClient *pubsub.Client, instanceConfig *config.InstanceConfig) {
	topic := pubsubClient.Topic(instanceConfig.IngestionConfig.SourceConfig.Topic)
	// Publish two messages that looks like the arrival of a file.
	b, err := json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "testdata/tx_log/perf-files-gcssource-test.json",
	})
	require.NoError(t, err)

	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)

	// Wait for the message to be sent.
	_, err = res.Get(ctx)
	require.NoError(t, err)

	// Now publish the second message.
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

func TestStart_ReceiveOneFileFilterOneFileViaSources(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	pubsubClient, instanceConfig := setupPubSubClient(t)

	// Send two events, but only one that is valid.
	sendPubSubMessages(ctx, t, pubsubClient, instanceConfig)

	// Create source and call Start.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)
	ch, err := source.Start(ctx)
	require.NoError(t, err)

	// Load the one file sendPubSubMessages should have sent.
	file := <-ch
	assert.Equal(t, testFile, file.Name)
	b, err := ioutil.ReadAll(file.Contents)
	assert.NoError(t, err)
	assert.NoError(t, file.Contents.Close())
	assert.Equal(t, "{\n  \"status\": \"Success\"\n}\n", string(b))
}

func TestStart_ReceiveOneFileFilterOneFileViaFilter(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	// Set up test.
	pubsubClient, instanceConfig := setupPubSubClient(t)

	// Reject names that contain tx_log.
	instanceConfig.IngestionConfig.SourceConfig.RejectIfNameMatches = "/tx_log/"

	// Send two events, but only one that is valid.
	sendPubSubMessages(ctx, t, pubsubClient, instanceConfig)

	// Create source and call Start.
	source, err := New(ctx, instanceConfig, true)
	require.NoError(t, err)
	ch, err := source.Start(ctx)
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
	_, err = source.Start(ctx)
	require.NoError(t, err)

	// A second call to Start should fail.
	_, err = source.Start(ctx)
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
