// Package process does the whole process of ingesting files into a trace store.
package process

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestevents"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const databaseName = "ingest"

func setupPubSubClient(t *testing.T) (*pubsub.Client, *config.InstanceConfig) {
	gcp_emulator.RequirePubSub(t)
	ctx := context.Background()

	rand.Seed(time.Now().Unix())
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				Project: "test-project",
			},
			FileIngestionTopicName: fmt.Sprintf("some-topic-%d", rand.Uint64()),
		},
	}

	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
	require.NoError(t, err)
	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	require.NoError(t, err)

	// Create the topic.
	topic := pubsubClient.Topic(instanceConfig.IngestionConfig.FileIngestionTopicName)
	ok, err := topic.Exists(ctx)
	require.NoError(t, err)
	if !ok {
		topic, err = pubsubClient.CreateTopic(ctx, instanceConfig.IngestionConfig.FileIngestionTopicName)
	}
	topic.Stop()
	assert.NoError(t, err)

	return pubsubClient, instanceConfig
}

func TestStart_IngestDemoRepoWithSpannerTraceStore_Success(t *testing.T) {

	_ = sqltest.NewSpannerDBForTests(t, databaseName)

	// Get tmp dir to use for repo checkout.
	tmpDir, err := os.MkdirTemp("", "ingest-process")
	require.NoError(t, err)
	tmpDir = filepath.Join(tmpDir, "repo")

	instanceConfig := config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.SpannerDataStoreType,
			TileSize:         256,
			ConnectionString: fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", emulators.GetEmulatorHostEnvVar(emulators.PGAdapter), databaseName),
		},
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				SourceType: config.DirSourceType,
				Sources:    []string{filepath.Join(testutils.GetRepoRoot(t), "perf/integration/data")},
			},
		},
		GitRepoConfig: config.GitRepoConfig{
			URL: "https://github.com/skia-dev/perf-demo-repo.git",
			Dir: tmpDir,
		},
	}

	err = Start(context.Background(), true, 1, &instanceConfig)
	require.NoError(t, err)
	// The integration data set has 9 good files, 1 file with a bad commit, and
	// 1 malformed JSON file.
	assert.Equal(t, int64(11), metrics2.GetCounter("perfserver_ingest_files_received").Get())
	assert.Equal(t, int64(1), metrics2.GetCounter("perfserver_ingest_bad_githash").Get())
	assert.Equal(t, int64(1), metrics2.GetCounter("perfserver_ingest_failed_to_parse").Get())
}

func TestSendPubSubEvent_Success(t *testing.T) {
	client, instanceConfig := setupPubSubClient(t)
	ctx := context.Background()

	// Setup the data to send via pubsub.
	params := []paramtools.Params{
		{
			"arch":   "x86",
			"config": "8888",
		},
		{
			"arch":   "arm",
			"config": "565",
		},
	}
	ps := paramtools.NewReadOnlyParamSet(params...)

	// Create the subscription before the pubsub message is sent, otherwise the
	// emulator won't deliver it.
	topic := client.Topic(instanceConfig.IngestionConfig.FileIngestionTopicName)
	// Create a subscription with the same name as the topic since the name is
	// random and won't have a conflict.
	sub, err := client.CreateSubscription(ctx, instanceConfig.IngestionConfig.FileIngestionTopicName, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	require.NoError(t, err)

	// Setup to receive the pubsub message we will send.
	cctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := sub.Receive(cctx, func(_ context.Context, msg *pubsub.Message) {
			ev, err := ingestevents.DecodePubSubBody(msg.Data)
			require.NoError(t, err)
			assert.Equal(t, "somefile.json", ev.Filename)
			assert.Equal(t, ps, ev.ParamSet)
			assert.Contains(t, ev.TraceIDs, ",arch=x86,config=8888,")
			wg.Done()
		})
		require.NoError(t, err)
	}()

	// Now we can finally send the message.
	err = sendPubSubEvent(ctx, client, instanceConfig.IngestionConfig.FileIngestionTopicName, params, ps, "somefile.json")
	require.NoError(t, err)

	// Wait for one message to be delivered.
	wg.Wait()

	// Stop the receiver.
	cancel()
	topic.Stop()
}
