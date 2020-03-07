package gcssource

import (
	"context"
	"encoding/json"
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

func TestStart(t *testing.T) {
	unittest.ManualTest(t)

	cfg := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: projectName,
				Topic:   topicName,
			},
		},
	}

	ctx := context.Background()
	pubSubClient, err := pubsub.NewClient(ctx, projectName)
	require.NoError(t, err)

	source := New(cfg, true)
	ch, err := source.Start(ctx)
	require.NoError(t, err)

	b, err := json.Marshal(pubSubEvent{
		Bucket: "gs://skia-infra",
		Name:   "testdata/perf-files-gcssource-test.json",
	})
	require.NoError(t, err)

	topic, err := pubSubClient.CreateTopic(ctx, topicName)
	require.NoError(t, err)
	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)
	_, err = res.Get(ctx)
	assert.NoError(t, err)

	file := <-ch
	assert.Equal(t, testFile, file.Name)
}
