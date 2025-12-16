package buildbucket_taskbackend

import (
	"net/http"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto/grpcpb"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/router"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/task_scheduler/go/db"
)

// HandlerWith creates an http.Handler which uses the given TaskBackend instance
// to serve TaskBackend HTTP endpoints.
func HandlerWith(tb *TaskBackend) http.Handler {
	srv := &prpc.Server{}
	buildbucketpb.RegisterTaskBackendServer(srv, tb)
	r := router.New()
	srv.InstallHandlers(r, nil)
	return r
}

// Handler creates an http.Handler which serves TaskBackend HTTP endpoints.
func Handler(buildbucketTarget, taskSchedulerHost string, projectRepoMapping map[string]string, d db.JobDB, bb2 buildbucket.BuildBucketInterface) http.Handler {
	return HandlerWith(NewTaskBackend(buildbucketTarget, taskSchedulerHost, projectRepoMapping, d, bb2))
}
