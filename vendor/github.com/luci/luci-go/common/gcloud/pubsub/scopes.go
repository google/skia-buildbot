// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pubsub

import (
	"cloud.google.com/go/pubsub"
)

var (
	// PublisherScopes is the set of OAuth2 scopes needed for a publisher to
	// publish messages.
	PublisherScopes = []string{
		pubsub.ScopePubSub,
	}

	// SubscriberScopes is the set of OAuth2 scopes needed for a subscriber to
	// pull and acknowledge messages.
	SubscriberScopes = []string{
		pubsub.ScopePubSub,
	}
)
