package sub

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNewWithSubNameProviderAndExpirationPolicy(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresPubSubEmulator(t)

	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	topicName := fmt.Sprintf("events-%d", rand.Int63())

	const numGoroutines = 5
	expirationPolicy := time.Hour * 24 * 7
	sub, err := NewWithSubNameProviderAndExpirationPolicy(ctx, true, project, topicName, NewConstNameProvider(mySubscriptionName), &expirationPolicy, numGoroutines)
	require.NoError(t, err)

	assert.Equal(t, numGoroutines, sub.ReceiveSettings.NumGoroutines)
	assert.Equal(t, numGoroutines*batchSize, sub.ReceiveSettings.MaxOutstandingMessages)
	assert.Contains(t, sub.String(), mySubscriptionName)
	cfg, err := sub.Config(ctx)
	assert.NoError(t, err)
	assert.Contains(t, cfg.Topic.ID(), topicName)
	assert.Equal(t, expirationPolicy, cfg.ExpirationPolicy)
	assert.NoError(t, sub.Delete(ctx))
}
