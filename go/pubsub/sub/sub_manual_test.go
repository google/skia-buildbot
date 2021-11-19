package sub

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	project            = "test-project"
	mySubscriptionName = "my-subscription-name"
)

func TestNewWithSubNameProvider(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresPubSubEmulator(t)

	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	topicName := fmt.Sprintf("events-%d", rand.Int63())

	const numGoroutines = 5
	sub, err := NewWithSubNameProvider(ctx, true, project, topicName, NewConstNameProvider(mySubscriptionName), numGoroutines)
	require.NoError(t, err)

	assert.Equal(t, numGoroutines, sub.ReceiveSettings.NumGoroutines)
	assert.Equal(t, numGoroutines*batchSize, sub.ReceiveSettings.MaxOutstandingMessages)
	assert.Contains(t, sub.String(), mySubscriptionName)
	cfg, err := sub.Config(ctx)
	assert.NoError(t, err)
	assert.Contains(t, cfg.Topic.ID(), topicName)
	assert.NoError(t, sub.Delete(ctx))
}

func TestNewConstNameProvider(t *testing.T) {
	unittest.SmallTest(t)

	name, err := NewConstNameProvider(mySubscriptionName).SubName()
	assert.NoError(t, err)
	assert.Equal(t, mySubscriptionName, name)
}

func TestNewRoundRobinNameProvider_LocalIsTrue_SubNameUsesHostName(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := fmt.Sprintf("events-%d", rand.Int63())

	name, err := NewRoundRobinNameProvider(true, topicName).SubName()
	assert.NoError(t, err)
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, name, fmt.Sprintf("%s-%s", topicName, hostname))
}

func TestNewRoundRobinNameProvider_LocalIsFalse_SubNameUsesSuffix(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := fmt.Sprintf("events-%d", rand.Int63())

	name, err := NewRoundRobinNameProvider(false, topicName).SubName()
	assert.NoError(t, err)
	assert.Equal(t, name, topicName+subscriptionSuffix)
}
