package perfresults

import (
	"context"
	"net/http"
	"strings"
	"time"

	bpb "go.chromium.org/luci/buildbucket/proto"
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
	bpb.BuildsClient
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
		BuildsClient: bpb.NewBuildsPRPCClient(&prpc.Client{
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

// findTaskRunID returns the swarming backend instance and taskId.
func (bc bbClient) findTaskRunID(ctx context.Context, buildID int64) (string, string, error) {
	build, err := bc.GetBuild(ctx, &bpb.GetBuildRequest{
		Id: buildID,
		Mask: &bpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{"status", "infra.backend.task.id"},
			},
		},
	})
	if err != nil {
		return "", "", skerr.Wrapf(err, "unable to get build info (%v)", buildID)
	}

	if build.GetStatus()&bpb.Status_ENDED_MASK == 0 {
		return "", "", skerr.Fmt("build (%v) is not ended", buildID)
	}
	t := build.GetInfra().GetBackend().GetTask().GetId()
	if t == nil {
		return "", "", skerr.Fmt("unable to get swarming task info for build (%v)", buildID)
	}
	if !strings.HasPrefix(t.GetTarget(), swarmingProtocol) {
		return "", "", skerr.Fmt("incorrect swarming instance (%v) for build (%v)", t.GetTarget(), buildID)
	}
	sh := t.GetTarget()[len(swarmingProtocol):] + ".appspot.com"

	return sh, t.GetId(), nil
}
