// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

// MessageIndex is a prefix or stream message index type.
type MessageIndex int64

// ContentType is a MIME-style content type.
type ContentType string

const (
	// ContentTypeText is a stream content type for text streams
	ContentTypeText ContentType = "text/plain"
	// ContentTypeBinary is a stream content type for binary streams.
	ContentTypeBinary = "application/octet-stream"

	// ContentTypeLogdogDatagram is a content type for size-prefixed datagram
	// frame stream.
	ContentTypeLogdogDatagram = "application/x-logdog-datagram"
	// ContentTypeLogdogLog is a LogDog log stream.
	ContentTypeLogdogLog = "application/x-logdog-logs"
	// ContentTypeLogdogIndex is a LogDog log index.
	ContentTypeLogdogIndex = "application/x-logdog-log-index"
)
