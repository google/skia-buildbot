package clustering2

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
)

// Run takes a ClusterRequest and runs it to completion before returning the results.
func Run(req *ClusterRequest, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, interesting float32) (*ClusterResponse, error) {
	proc := &ClusterRequestProcess{
		request:     req,
		git:         git,
		cidl:        cidl,
		lastUpdate:  time.Now(),
		state:       PROCESS_RUNNING,
		message:     "Running",
		interesting: interesting,
	}
	proc.Run()
	if proc.state == PROCESS_ERROR {
		return nil, fmt.Errorf("Failed to complete clustering: %s", proc.message)
	}
	return proc.response, nil
}
