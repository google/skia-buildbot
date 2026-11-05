// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !dev

package main

import (
	"go.temporal.io/sdk/worker"
)

func registerMockActivities(w worker.Worker) {
	// No-op in production/non-dev builds.
}

const devMode = false
