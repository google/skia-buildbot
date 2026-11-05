// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backfill

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	alertconfigmocks "go.skia.org/infra/perf/go/alerts/mock"
	"go.skia.org/infra/perf/go/regression"
)

func TestProcessBackfillMessage_InvalidJSON_ReturnsErrorAndTrue(t *testing.T) {
	l := &Listener{}
	msg := &pubsub.Message{
		Data: []byte("invalid json"),
	}
	alertID, shouldAck, err := l.processBackfillMessage(context.Background(), msg)
	require.Error(t, err)
	assert.Equal(t, int64(0), alertID)
	assert.True(t, shouldAck)
}

func TestProcessBackfillMessage_InvalidAlertID_ReturnsErrorAndTrue(t *testing.T) {
	l := &Listener{}
	msg := &pubsub.Message{
		Data: []byte(`{"alert_id":"abc"}`),
	}
	alertID, shouldAck, err := l.processBackfillMessage(context.Background(), msg)
	require.Error(t, err)
	assert.Equal(t, int64(0), alertID)
	assert.True(t, shouldAck)
}

func TestProcessBackfillMessage_AlertNotFound_ReturnsErrorAndTrue(t *testing.T) {
	cp := alertconfigmocks.NewConfigProvider(t)
	cp.On("GetAlertConfig", int64(123)).Return(nil, skerr.Fmt("not found"))
	l := &Listener{
		provider: cp,
	}
	req := regression.BackfillRequest{
		AlertID: 123,
	}
	data, _ := json.Marshal(req)
	msg := &pubsub.Message{
		Data: data,
	}
	alertID, shouldAck, err := l.processBackfillMessage(context.Background(), msg)
	require.Error(t, err)
	assert.Equal(t, int64(123), alertID)
	assert.True(t, shouldAck)
}
