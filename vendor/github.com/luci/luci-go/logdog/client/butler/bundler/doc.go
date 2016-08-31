// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package bundler is responsible for efficiently transforming aggregate stream
// data into Butler messages for export.
//
// A process will instantiate a single Bundler instance. The Bundler manages an
// elastic set of Stream instances, each of which contains state for a single
// log Stream.
//
// Each Stream instance will have sequential stream binary data appended to it
// via Append, which it will collect and organize for export as a series of
// ButlerLogBundle_Entry protobufs. Streams operate independently and buffer
// data until it is consumed by their Bundler instance. If a Stream's buffer is
// full, the Stream will block on appending data, which will, in turn, block its
// data source.
//
// The Bundler owns the various Stream instances. When its Next() method is
// called, it will sort through the stream instances to prepare an
// optimally-sized ButlerLogBundle protobuf for export. The construction of this
// bundle may block pending data, and may be subject to various data urgency
// requests.
//
// The Bundler acknowledges the following constraints:
//   - Data enqueued into a Stream should be exported within a specific period
//     of time from its introduction
//   - The exported ButlerLogBundle protobuf must not exceed a maximum bundle
//     size constraint.
//   - Stream data may be added during the bundling process, and should be
//     acknowledged if possible.
//
// When a Stream is finished, its Close method should be called. This alerts the
// Stream that it will receive no more data, causing it to export a terminal
// ButlerLogBundle and unregister from the Bundler.
//
// The Bundler may block via its CloseAndFinish() method until all Streams are
// drained and cleared.
package bundler
