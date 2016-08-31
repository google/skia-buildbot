// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package frontend is DM's Google AppEngine application stub.
//
// Setup
//
// * Create a pubsub topic "projects/$APPID/topics/dm-distributor-notify"
// * Create a pubsub Push subscription:
//     Topic: "projects/$APPID/topics/dm-distributor-notify"
//     Name: "projects/$APPID/subscriptions/dm-distributor-notify"
//     Push: "https://$APPID.$APPDOMAIN/_ah/push-handlers/dm-distributor-notify"
package frontend
