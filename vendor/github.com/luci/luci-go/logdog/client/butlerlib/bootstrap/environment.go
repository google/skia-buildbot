// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bootstrap

// Environment variable names
const (
	// EnvStreamServerPath is the path to the Butler's stream server endpoint.
	//
	// This can be used by applications to initiate a new Butler stream with an
	// existing Butler stream server process. If a subprocess is launched with a
	// stream server configuration, it will propagate this path to its child
	// processes.
	EnvStreamServerPath = "LOGDOG_STREAM_SERVER_PATH"

	// EnvStreamProject is the environment variable set to the configured stream
	// project name.
	EnvStreamProject = "LOGDOG_STREAM_PROJECT"

	// EnvStreamPrefix is the environment variable set to the configured
	// stream name prefix.
	EnvStreamPrefix = "LOGDOG_STREAM_PREFIX"
)
