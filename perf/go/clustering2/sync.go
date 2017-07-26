package clustering2

import (
	"fmt"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
)

// Run takes a ClusterRequest and runs it to completion before returning the results.
func Run(req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup) (*ClusterResponse, error) {
	proc := newProcess(req, git, cidl)
	proc.Run()
	if proc.state == PROCESS_ERROR {
		return nil, fmt.Errorf("Failed to complete clustering: %s", proc.message)
	}
	return proc.response, nil
}
