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
	testFile    = "gs://skia-infra/testdata/perf-files-gcssource-test.json"
)

func setupPubSub(t *testing.T, cfg *config.InstanceConfig) {
	ctx := context.Background()
	pubSubClient, err := pubsub.NewClient(ctx, projectName)
	require.NoError(t, err)

	b, err := json.Marshal(pubSubEvent{
		Bucket: "skia-infra",
		Name:   "testdata/perf-files-gcssource-test.json",
	})
	require.NoError(t, err)

	topic := pubSubClient.Topic(topicName)
	exists, err := topic.Exists(ctx)
	require.NoError(t, err)

	if !exists {
		topic, err = pubSubClient.CreateTopic(ctx, topicName)
		require.NoError(t, err)
	}

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

	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)
	_, err = res.Get(ctx)
	assert.NoError(t, err)

}

func TestStart(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

	cfg := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: projectName,
				Topic:   topicName,
			},
		},
	}

	setupPubSub(t, cfg)
	source := New(cfg, true)
	ch, err := source.Start(ctx)
	require.NoError(t, err)

	file := <-ch
	assert.Equal(t, testFile, file.Name)
	b, err := ioutil.ReadAll(file.Contents)
	assert.NoError(t, err)
	assert.Equal(t, "{\n  \"status\": \"Success\"\n}\n", string(b))
}
