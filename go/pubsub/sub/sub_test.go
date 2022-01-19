package sub

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	project            = "test-project"
	mySubscriptionName = "my-subscription-name"
)

func TestNewConstNameProvider(t *testing.T) {
	unittest.SmallTest(t)

	name, err := NewConstNameProvider(mySubscriptionName).SubName()
	assert.NoError(t, err)
	assert.Equal(t, mySubscriptionName, name)
}

func TestNewRoundRobinNameProvider_LocalIsTrue_SubNameUsesHostName(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := "events"

	name, err := NewRoundRobinNameProvider(true, topicName).SubName()
	assert.NoError(t, err)
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, name, fmt.Sprintf("%s-%s", topicName, hostname))
}

func TestNewRoundRobinNameProvider_LocalIsFalse_SubNameUsesSuffix(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := "events"

	name, err := NewRoundRobinNameProvider(false, topicName).SubName()
	assert.NoError(t, err)
	assert.Equal(t, name, topicName+subscriptionSuffix)
}

func TestNewBroadcastNameProvider_LocalIsTrue_SubNameDoesNotUseSuffix(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := "events"

	name, err := NewBroadcastNameProvider(true, topicName).SubName()
	assert.NoError(t, err)
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, name, fmt.Sprintf("%s-%s", topicName, hostname))
}

func TestNewBroadcastNameProvider_LocalIsFalse_SubNameUsesSuffix(t *testing.T) {
	unittest.SmallTest(t)

	rand.Seed(time.Now().Unix())
	topicName := "events"

	name, err := NewBroadcastNameProvider(false, topicName).SubName()
	assert.NoError(t, err)
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, name, fmt.Sprintf("%s-%s%s", topicName, hostname, subscriptionSuffix))
}
