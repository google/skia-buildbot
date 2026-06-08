//go:build !dev

package main

import (
	"go.temporal.io/sdk/worker"
)

func registerMockActivities(w worker.Worker) {
	// No-op in production/non-dev builds.
}

const devMode = false
