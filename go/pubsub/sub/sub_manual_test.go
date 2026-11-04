// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sub

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/emulators/gcp_emulator"
)

func TestNewWithSubNameProviderAndExpirationPolicy(t *testing.T) {
	gcp_emulator.RequirePubSub(t)

	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	topicName := fmt.Sprintf("events-%d", rand.Int63())

	const numGoroutines = 5
	expirationPolicy := time.Hour * 24 * 7
	sub, err := NewWithSubNameProviderAndExpirationPolicy(ctx, project, topicName, NewConstNameProvider(mySubscriptionName), &expirationPolicy, numGoroutines)
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
