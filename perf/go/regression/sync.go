package regression

import (
	"context"
	"fmt"

	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
)

// RegressionDetectionResponseProcessor is a callback that is called with RegressionDetectionResponses as a RegressionDetectionRequest is being processed.
type RegressionDetectionResponseProcessor func(*RegressionDetectionRequest, []*RegressionDetectionResponse, string)

// Run takes a RegressionDetectionRequest and runs it to completion before returning the results.
//
// TODO(jcgregorio) Make a member of RunningRegressionDetectionRequests.
func Run(
	ctx context.Context,
	req *RegressionDetectionRequest,
	perfGit *perfgit.Git,
	dfBuilder dataframe.DataFrameBuilder,
	shortcutStore shortcut.Store,
	responseProcessor RegressionDetectionResponseProcessor,
	progressCallback types.ProgressCallback,
) ([]*RegressionDetectionResponse, error) {
	proc, err := newProcess(ctx, req, perfGit, dfBuilder, shortcutStore, responseProcessor, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("Failed to start new regression detection process: %s", err)
	}
	proc.run()
	if proc.state == ProcessError {
		return nil, fmt.Errorf("Failed to complete regression detection: %s", proc.message)
	}
	return proc.responses(), nil
}
