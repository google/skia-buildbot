package regression

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
)

// ClusterResponseProcessor is a callback that is called with ClusterResponses as a ClusterRequest is being processed.
type ClusterResponseProcessor func([]*ClusterResponse)

// Run takes a ClusterRequest and runs it to completion before returning the results.
func Run(ctx context.Context, req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder, clusterResponseProcessor ClusterResponseProcessor) ([]*ClusterResponse, error) {
	proc := newProcess(ctx, req, git, cidl, dfBuilder, clusterResponseProcessor)
	proc.Run(ctx)
	if proc.state == PROCESS_ERROR {
		return nil, fmt.Errorf("Failed to complete clustering: %s", proc.message)
	}
	return proc.Responses(), nil
}
