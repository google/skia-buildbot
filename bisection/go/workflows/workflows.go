package workflows

// Package workflow contains const and types to invoke Workflows.

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
	Builder string
}
