// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

const (
	// MaxButlerLogBundleSize is the maximum size, in bytes, of the data section
	// of a ButlerLogBundle.
	MaxButlerLogBundleSize = 10 * 1024 * 1024

	// MaxLogEntryDataSize is the maximum size, in bytes, of the data section of
	// a single log entry (1 MiB).
	MaxLogEntryDataSize = 1 * 1024 * 1024

	// MaxDatagramSize is the maximum size, in bytes, of datagram data.
	MaxDatagramSize int = MaxLogEntryDataSize
)
