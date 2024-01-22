// Package workflow contains const and types to invoke Workflows.
package workflows

import (
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
)

// Workflow name definitions.
//
// Those are used to invoke the workflows. This is meant to decouple the
// souce code dependencies such that the client doesn't need to link with
// the actual implementation.
const (
	BuildChrome = "perf.build_chrome"
)

// Workflow params definitions.
//
// Each workflow defines its own struct for the params, this will ensure
// the input parameter type safety, as well as expose them in a structured way.
type BuildChromeParams struct {
	// PinpointJobID is the Job ID to associate with the build.
	PinpointJobID string
	// Commit is the chromium commit hash.
	Commit string
	// Device is the name of the device, e.g. "linux-perf".
	Device string
	// Target is name of the build isolate target
	// e.g. "performance_test_suite".
	Target string
	// Patch is the Gerrit patch included in the build.
	Patch []*buildbucketpb.GerritChange
}
