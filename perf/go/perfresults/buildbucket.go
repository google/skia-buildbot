package perfresults

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	bpb "go.chromium.org/luci/buildbucket/proto"
	bgrpbpb "go.chromium.org/luci/buildbucket/proto/grpcpb"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	buildbucketHost  = "cr-buildbucket.appspot.com:443"
	swarmingProtocol = "swarming://"
)

// bbClient wraps bpb.BuildsClient to provide convenient functions
type bbClient struct {
	bgrpbpb.BuildsClient
}

// BuildInfo contains info that are useful for identifying the perf results.
type BuildInfo struct {
	// The swarming instance that runs this build.
	SwarmingInstance string

	// The builder name.
	BuilderName string

	// The perf machine group, see bit.ly/perf-dashboard-machine-group.
	//
	// This groups similar builders, and is defined as a builder property
	// "perf_dashboard_machine_group".
	MachineGroup string

	// The swarming task ID that runs this build.
	TaskID string

	// The git hash Revision that this build was built at.
	//
	// Note the patches and other source info is not added here as we don't need them now. More info
	// can be expanded as needed later. We should try to keep this simple.
	Revision string

	// The commit position that this build was built at.
	CommitPosisition string
}

func (bi BuildInfo) GetPosition() string {
	cp := regexp.MustCompile(`\d+`).FindString(bi.CommitPosisition)
	// Return the git hash if no position is found
	if cp == "" {
		return bi.Revision
	}
	return "CP:" + cp
}

func newBuildsClient(ctx context.Context, client *http.Client) (*bbClient, error) {
	if client == nil {
		ts, err := google.DefaultTokenSource(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "unable to fetch token source")
		}

		client = httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	}

	return &bbClient{
		BuildsClient: bgrpbpb.NewBuildsClient(&prpc.Client{
			C:    client,
			Host: buildbucketHost,
			Options: &prpc.Options{
				Retry: func() retry.Iterator {
					return &retry.ExponentialBackoff{
						MaxDelay: time.Minute,
						Limited: retry.Limited{
							Delay:   time.Second,
							Retries: 1,
						},
					}
				},
				PerRPCTimeout: 90 * time.Second,
			},
		}),
	}, nil
}

// findBuildInfo returns the swarming backend instance and taskId.
func (bc bbClient) findBuildInfo(ctx context.Context, buildID int64) (BuildInfo, error) {
	build, err := bc.GetBuild(ctx, &bpb.GetBuildRequest{
		Id: buildID,
		Mask: &bpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{"builder", "status", "infra.backend.task.id", "output.properties", "input.properties"},
			},
		},
	})
	if err != nil {
		return BuildInfo{}, skerr.Wrapf(err, "unable to get build info (%v)", buildID)
	}

	if build.GetStatus()&bpb.Status_ENDED_MASK == 0 {
		return BuildInfo{}, skerr.Fmt("build (%v) is not ended", buildID)
	}
	t := build.GetInfra().GetBackend().GetTask().GetId()
	if t == nil {
		return BuildInfo{}, skerr.Fmt("unable to get swarming task info for build (%v)", buildID)
	}
	if !strings.HasPrefix(t.GetTarget(), swarmingProtocol) {
		return BuildInfo{}, skerr.Fmt("incorrect swarming instance (%v) for build (%v)", t.GetTarget(), buildID)
	}
	sh := t.GetTarget()[len(swarmingProtocol):] + ".appspot.com"

	props := build.GetOutput().GetProperties().AsMap()

	input := build.GetInput().GetProperties().AsMap()
	machineGroup, _ := input["perf_dashboard_machine_group"].(string)
	return BuildInfo{
		SwarmingInstance: sh,
		BuilderName:      build.GetBuilder().GetBuilder(),
		MachineGroup:     machineGroup,
		TaskID:           t.GetId(),
		Revision:         props["got_revision"].(string),
		CommitPosisition: props["got_revision_cp"].(string),
	}, nil
}
