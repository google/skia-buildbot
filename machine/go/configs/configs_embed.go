// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"embed" // Enable go:embed.
)

// Configs is a filesystem with all the config files.
//
//go:embed *.json
var Configs embed.FS
