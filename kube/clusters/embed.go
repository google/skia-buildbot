// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clusters contains the current cluster config json file as an embedded
// string.
package clusters

import (
	_ "embed"
)

//go:embed config.json
var ClusterConfig string
