package swarmingv2

import (
	"context"

	"go.chromium.org/luci/grpc/prpc"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
)

// SwarmingV2Client wraps a Swarming BotsClient and TasksClient.
type SwarmingV2Client interface {
	apipb.BotsClient
	apipb.TasksClient
}

// WrappedClient implements SwarmingV2Client by wrapping a BotsClient and a
// TasksClient
type WrappedClient struct {
	apipb.BotsClient
	apipb.TasksClient
}

// NewClient returns a SwarmingV2Client implementation.
func NewClient(prpcClient *prpc.Client) *WrappedClient {
	return &WrappedClient{
		BotsClient:  apipb.NewBotsClient(prpcClient),
		TasksClient: apipb.NewTasksClient(prpcClient),
	}
}

// Assert that WrappedClient implements SwarmingV2Client.
var _ SwarmingV2Client = &WrappedClient{}

// ListBotsHelper makes multiple paginated requests to ListBots to retrieve all
// results.
func ListBotsHelper(ctx context.Context, c apipb.BotsClient, req *apipb.BotsRequest) ([]*apipb.BotInfo, error) {
	// Taken from https://source.chromium.org/chromium/infra/infra/+/main:go/src/go.chromium.org/luci/swarming/client/swarming/client.go;l=473
	req.Limit = 1000
	req.Cursor = ""
	rv := make([]*apipb.BotInfo, 0, req.Limit)
	for {
		resp, err := c.ListBots(ctx, req)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, resp.Items...)
		req.Cursor = resp.Cursor
		if req.Cursor == "" {
			break
		}
	}
	return rv, nil
}

// ListTasksHelper makes multiple paginated requests to ListTasks to retrieve
// all results.
func ListTasksHelper(ctx context.Context, c apipb.TasksClient, req *apipb.TasksWithPerfRequest) ([]*apipb.TaskResultResponse, error) {
	// Taken from https://source.chromium.org/chromium/infra/infra/+/main:go/src/go.chromium.org/luci/swarming/client/swarming/client.go;l=282
	req.Limit = 1000
	req.Cursor = ""
	rv := make([]*apipb.TaskResultResponse, 0, req.Limit)
	for {
		resp, err := c.ListTasks(ctx, req)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, resp.Items...)
		req.Cursor = resp.Cursor
		if req.Cursor == "" {
			break
		}
	}
	return rv, nil
}

// BotDimensionsToStringMap converts Swarming bot dimensions as represented in
// the Swarming API to a map[string][]string.
func BotDimensionsToStringMap(dims []*apipb.StringListPair) map[string][]string {
	m := make(map[string][]string, len(dims))
	for _, pair := range dims {
		m[pair.Key] = append(m[pair.Key], pair.Value...)
	}
	return m
}

// BotDimensionsToStringSlice converts Swarming bot dimensions as represented
// in the Swarming API to a []string.
func BotDimensionsToStringSlice(dims []*apipb.StringListPair) []string {
	return swarming.PackageDimensions(BotDimensionsToStringMap(dims))
}

// MakeCASReference returns a CASReference which can be used as input to
// a Swarming task.
func MakeCASReference(digest, casInstance string) (*apipb.CASReference, error) {
	hash, size, err := rbe.StringToDigest(digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &apipb.CASReference{
		CasInstance: casInstance,
		Digest: &apipb.Digest{
			Hash:      hash,
			SizeBytes: size,
		},
	}, nil
}
