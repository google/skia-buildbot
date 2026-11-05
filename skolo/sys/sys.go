// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sys

import (
	"embed" // Enable go:embed.
)

// Sys is a filesystem with all the config files.
//
//go:embed *.json5
var Sys embed.FS
