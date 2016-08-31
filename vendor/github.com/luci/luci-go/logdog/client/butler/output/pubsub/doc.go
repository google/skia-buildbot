// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package pubsub implements the "pubsub" Output.
//
// The "pubsub" (Google Cloud Pub/Sub) Output publishes ButlerLogBundle
// protobufs to a Google Cloud Pub/Sub topic using the protocol defined in:
//   github.com/luci/luci-go/common/logdog/butlerproto
package pubsub
