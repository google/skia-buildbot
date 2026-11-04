// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sink

import (
	"context"
)

// MetricName used for all successful sends by a Sink implementation.
const MetricName = "machine_change_send_success"

// Sink notifies a machine that it's Description has changed.
type Sink interface {
	// Send a machine.Description from the server to the machine.
	Send(ctx context.Context, machineID string) error
}
