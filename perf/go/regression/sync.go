package regression

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/shortcut"
)

// RegressionDetectionResponseProcessor is a callback that is called with RegressionDetectionResponses as a RegressionDetectionRequest is being processed.
type RegressionDetectionResponseProcessor func(*RegressionDetectionRequest, []*RegressionDetectionResponse)

// Run takes a RegressionDetectionRequest and runs it to completion before returning the results.
func Run(ctx context.Context, req *RegressionDetectionRequest, vcs vcsinfo.VCS, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder, shortcutStore shortcut.Store, responseProcessor RegressionDetectionResponseProcessor) ([]*RegressionDetectionResponse, error) {
	proc, err := newProcess(ctx, req, vcs, cidl, dfBuilder, shortcutStore, responseProcessor)
	if err != nil {
		return nil, fmt.Errorf("Failed to start new regression detection process: %s", err)
	}
	proc.Run()
	if proc.state == ProcessError {
		return nil, fmt.Errorf("Failed to complete regression detection: %s", proc.message)
	}
	return proc.Responses(), nil
}
