package regression

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
)

// Run takes a ClusterRequest and runs it to completion before returning the results.
func Run(ctx context.Context, req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) ([]*ClusterResponse, error) {
	proc := newProcess(req, git, cidl, dfBuilder)
	proc.Run(ctx)
	if proc.state == PROCESS_ERROR {
		return nil, fmt.Errorf("Failed to complete clustering: %s", proc.message)
	}
	return proc.Responses(), nil
}
