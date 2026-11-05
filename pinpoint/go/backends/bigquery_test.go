// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backends

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBigQueryClient_ValidProject_Client(t *testing.T) {
	ctx := context.Background()
	client, err := NewBigQueryClient(ctx, "chromeperf")
	require.NoError(t, err)
	assert.NotNil(t, client)
}
