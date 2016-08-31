// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package config is a library to access the luci-config service.
//
// There are three backends for the interface presented in the interface.go.
//
// One backend talks directly to the luci-config service, another reads
// from the local filesystem, and the third one reads from memory struct.
//
// Usually, you should use the remote backend in production, the filesystem
// backend when developing, and the memory backend from unit tests.
package config
