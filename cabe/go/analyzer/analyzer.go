package analyzer

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	cpb "go.skia.org/infra/cabe/go/proto"
)

// CASResultReader is an interface for getting PerfResults for CAS instance and root digest values.
type CASResultReader func(context.Context, string, string) (map[string]PerfResults, error)

// SwarmingTaskReader is an interface for getting Swarming task metadata associated with a pinpoint job.
type SwarmingTaskReader func(context.Context) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error)

// Options configure one or more fields of an Analyzer instance.
type Options func(*Analyzer)

// WithCASResultReader configures an Analyzer instance to use the given CASResultReader.
func WithCASResultReader(r CASResultReader) Options {
	return func(e *Analyzer) {
		e.readCAS = r
	}
}

// WithTaskResultsReader configures an Analyzer instance to use the given TaskResultsReader.
func WithSwarmingTaskReader(r SwarmingTaskReader) Options {
	return func(e *Analyzer) {
		e.readSwarmingTasks = r
	}
}

// New returns a new instance of Analyzer. Set either pinpointJobID, or controlDigests and treatmentDigests.
func New(opts ...Options) *Analyzer {
	ret := &Analyzer{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Analyzer encapsulates the state of an Analyzer process execution. Its lifecycle follows a request
// to process all of the output of an A/B benchmark experiment run.
// Users of Analyzer must instantiate and attach the necessary service dependencies.
type Analyzer struct {
	readCAS           CASResultReader
	readSwarmingTasks SwarmingTaskReader
	results           []RResult
}

// RResult encapsulates a response from the R lamprey after it has been
// extracted from the rexp.Value type it returns.
type RResult struct {
	// Benchmark is the name of a perf benchmark suite, such as Speedometer2 or JetStream
	Benchmark string
	// Workload is the name of a benchmark-specific workload, such as TodoMVC-ReactJS
	WorkLoad string
	// BuildConfig is the name of a build configuration, e.g. "Mac arm Builder Perf PGO"
	BuildConfig string
	// RunConfig is the name of a run configuration, e.g. "Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0"
	RunConfig string
	// Statistics summarizes the difference between the treatment and control arms for the given
	// Benchmark and Workload on the hardware described by RunConfig, using the binary built using
	// the given BuildConfig.
	// Statistics Statistics
}

// Results returns the results of the Analyzer process.
func (a *Analyzer) Results() []RResult {
	return a.results
}

// AnalysisResults returns a slice of AnalysisResult protos populated with data from the
// experiment.
func (a *Analyzer) AnalysisResults() []*cpb.AnalysisResult {
	ret := []*cpb.AnalysisResult{}

	return ret
}

// Run executes the whole Analyzer process for a single, complete experiment.
func (a *Analyzer) Run(ctx context.Context) error {
	return fmt.Errorf("Not implemented yet")
}
