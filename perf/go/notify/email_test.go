// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
)

func TestEmailTransportSendNewRegression_EmailIsMissing_ReturnsError(t *testing.T) {
	e, err := NewEmailTransport()
	require.NoError(t, err)
	_, err = e.SendNewRegression(context.Background(), &alerts.Alert{}, "", "")
	require.Contains(t, err.Error(), "No email address")

}

func TestEmailTransportSendRegressionMissing_EmailIsMissing_ReturnsError(t *testing.T) {
	e, err := NewEmailTransport()
	require.NoError(t, err)
	err = e.SendRegressionMissing(context.Background(), "", &alerts.Alert{}, "", "")
	require.Contains(t, err.Error(), "No email address")

}
