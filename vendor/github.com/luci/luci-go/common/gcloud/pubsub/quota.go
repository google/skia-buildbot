// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pubsub

import (
	"time"

	"cloud.google.com/go/pubsub"
)

// Cloud PubSub quota is documented here:
// https://cloud.google.com/pubsub/quotas
const (
	// MaxPublishSize is the maximum size, in bytes, of the published message
	// (10 MB).
	//
	// See: https://cloud.google.com/pubsub/publisher
	MaxPublishSize = 10 * 1024 * 1024

	// MaxPublishBatchSize is the maximum PubSub batch size.
	MaxPublishBatchSize = pubsub.MaxPublishBatchSize

	// MaxProjectMessagesPerSecond is the maximum number of requests per second,
	// across the entire project.
	MaxProjectMessagesPerSecond = 10000

	// MaxSubscriptionPullSize is the maximum number of subscription records that
	// can be pulled at a time.
	MaxSubscriptionPullSize = pubsub.DefaultMaxPrefetch

	// MaxACKDeadline is the maximum acknowledgement deadline that can be applied
	// to a leased subscription Message.
	MaxACKDeadline = 600 * time.Second
)
