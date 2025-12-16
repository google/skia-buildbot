package swarmingv2

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/prpc"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	apigrpcpb "go.chromium.org/luci/swarming/proto/api_v2/grpcpb"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

// SwarmingV2Client wraps a Swarming BotsClient and TasksClient.
type SwarmingV2Client interface {
	apigrpcpb.BotsClient
	apigrpcpb.TasksClient
}

// WrappedClient implements SwarmingV2Client by wrapping a BotsClient and a
// TasksClient
type WrappedClient struct {
	apigrpcpb.BotsClient
	apigrpcpb.TasksClient
}

// NewClient returns a SwarmingV2Client implementation.
func NewClient(prpcClient *prpc.Client) *WrappedClient {
	return &WrappedClient{
		BotsClient:  apigrpcpb.NewBotsClient(prpcClient),
		TasksClient: apigrpcpb.NewTasksClient(prpcClient),
	}
}

// DefaultPRPCClient returns a prpc.Client using default settings.
func DefaultPRPCClient(httpClient *http.Client, swarmingServer string) *prpc.Client {
	return &prpc.Client{
		C:    httpClient,
		Host: swarmingServer,
		Options: &prpc.Options{
			Retry: func() retry.Iterator {
				return &retry.ExponentialBackoff{
					MaxDelay: time.Minute,
					Limited: retry.Limited{
						Delay:   time.Second,
						Retries: 10,
					},
				}
			},
			// The swarming server has an internal 60-second deadline for responding to
			// requests, so 90 seconds shouldn't cause any requests to fail that would
			// otherwise succeed.
			PerRPCTimeout: 90 * time.Second,
		},
	}
}

// NewDefaultClient returns a SwarmingV2Client implementation using default
// PRPC client settings.
func NewDefaultClient(httpClient *http.Client, swarmingServer string) *WrappedClient {
	return NewClient(DefaultPRPCClient(httpClient, swarmingServer))
}

// Assert that WrappedClient implements SwarmingV2Client.
var _ SwarmingV2Client = &WrappedClient{}

// ListBotsHelper makes multiple paginated requests to ListBots to retrieve all
// results.
func ListBotsHelper(ctx context.Context, c apigrpcpb.BotsClient, req *apipb.BotsRequest) ([]*apipb.BotInfo, error) {
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

// ListBotsForPool retrieves all of the bots in the given pool.
func ListBotsForPool(ctx context.Context, c apigrpcpb.BotsClient, pool string) ([]*apipb.BotInfo, error) {
	return ListBotsHelper(ctx, c, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: swarming.DIMENSION_POOL_KEY, Value: pool},
		},
	})
}

// ListTasksHelper makes multiple paginated requests to ListTasks to retrieve
// all results.
func ListTasksHelper(ctx context.Context, c apigrpcpb.TasksClient, req *apipb.TasksWithPerfRequest) ([]*apipb.TaskResultResponse, error) {
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

// GetRequestMetadataForTasks returns the apipb.TaskRequestMetadataResponse for
// each of the given apipb.TaskResultResponses.
func GetRequestMetadataForTasks(ctx context.Context, c apigrpcpb.TasksClient, tasks []*apipb.TaskResultResponse) ([]*apipb.TaskRequestMetadataResponse, error) {
	rv := make([]*apipb.TaskRequestMetadataResponse, len(tasks))
	g := multierror.Group{}
	for idx, task := range tasks {
		idx := idx // https://golang.org/doc/faq#closures_and_goroutines
		task := task
		g.Go(func() error {
			request, err := c.GetRequest(ctx, &apipb.TaskIdRequest{
				TaskId: task.TaskId,
			})
			if err != nil {
				return err
			}
			rv[idx] = &apipb.TaskRequestMetadataResponse{
				Request:    request,
				TaskId:     task.TaskId,
				TaskResult: task,
			}
			return nil
		})
	}
	if err := g.Wait().ErrorOrNil(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// ListTaskRequestMetadataHelper is like ListTasksHelper but retrieves the
// apipb.TaskRequestMetadataResponse for each of the task. This is significantly
// more expensive, so it should only be used where necessary.
func ListTaskRequestMetadataHelper(ctx context.Context, c apigrpcpb.TasksClient, req *apipb.TasksWithPerfRequest) ([]*apipb.TaskRequestMetadataResponse, error) {
	tasks, err := ListTasksHelper(ctx, c, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return GetRequestMetadataForTasks(ctx, c, tasks)
}

// DeleteBots performs multiple calls to DeleteBot in parallel.
func DeleteBots(ctx context.Context, c apigrpcpb.BotsClient, botIds []string) error {
	g := multierror.Group{}
	for _, botId := range botIds {
		botId := botId // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			resp, err := c.DeleteBot(ctx, &apipb.BotRequest{
				BotId: botId,
			})
			if err != nil {
				return skerr.Wrap(err)
			}
			if !resp.Deleted {
				return skerr.Fmt("could not delete bot %q", botId)
			}
			return nil
		})
	}
	return skerr.Wrap(g.Wait().ErrorOrNil())
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

// StringMapToBotDimensions converts Swarming bot dimensions from a
// map[string][]string to their Swarming API representation.
func StringMapToBotDimensions(dims map[string][]string) []*apipb.StringListPair {
	dimensions := make([]*apipb.StringListPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &apipb.StringListPair{
			Key:   k,
			Value: v,
		})
	}
	return dimensions
}

// StringMapToTaskDimensions converts Swarming task dimensions from a
// map[string]string to their Swarming API representation.
func StringMapToTaskDimensions(dims map[string]string) []*apipb.StringPair {
	dimensions := make([]*apipb.StringPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &apipb.StringPair{
			Key:   k,
			Value: v,
		})
	}
	return dimensions
}

// ConvertCIPDInput converts a slice of cipd.Package to a SwarmingRpcsCipdInput.
func ConvertCIPDInput(pkgs []*cipd.Package) *apipb.CipdInput {
	rv := &apipb.CipdInput{
		Packages: []*apipb.CipdPackage{},
	}
	for _, pkg := range pkgs {
		rv.Packages = append(rv.Packages, &apipb.CipdPackage{
			PackageName: pkg.Name,
			Path:        pkg.Path,
			Version:     pkg.Version,
		})
	}
	return rv
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

// GetTaskRequestProperties returns the SwarmingRpcsTaskProperties for the given
// SwarmingRpcsTaskRequestMetadata.
func GetTaskRequestProperties(t *apipb.TaskRequestResponse) *apipb.TaskProperties {
	if len(t.TaskSlices) > 0 {
		// TODO(borenet): It would probably be better to determine which
		// (if any) of the TaskSlices actually ran, rather than assuming
		// it was the first.
		return t.TaskSlices[0].Properties
	}
	return t.Properties
}

var retriesRE = regexp.MustCompile("retries:([0-9])*")

// RetryTask duplicates fields from the given request and triggers a new task
// as a retry of it.
func RetryTask(ctx context.Context, c SwarmingV2Client, req *apipb.TaskRequestResponse) (*apipb.TaskRequestMetadataResponse, error) {
	// Swarming API does not have a way to Retry commands. This was done
	// intentionally by swarming-eng to reduce API surface.
	newReq := &apipb.NewTaskRequest{}
	newReq.Name = fmt.Sprintf("%s (retry)", req.Name)
	newReq.ParentTaskId = req.ParentTaskId
	newReq.Priority = req.Priority
	newReq.PubsubTopic = req.PubsubTopic
	newReq.PubsubUserdata = req.PubsubUserdata
	newReq.User = req.User
	newReq.TaskSlices = req.TaskSlices
	if newReq.TaskSlices == nil {
		newReq.ExpirationSecs = req.ExpirationSecs
		newReq.Properties = req.Properties
	}

	newReq.Tags = req.Tags
	// Add retries tag. Increment it if it already exists.
	foundRetriesTag := false
	for i, tag := range newReq.Tags {
		if retriesRE.FindString(tag) != "" {
			n, err := strconv.Atoi(strings.Split(tag, ":")[1])
			if err != nil {
				sklog.Errorf("retries value in %s is not numeric: %s", tag, err)
				continue
			}
			newReq.Tags[i] = fmt.Sprintf("retries:%d", (n + 1))
			foundRetriesTag = true
		}
	}
	if !foundRetriesTag {
		newReq.Tags = append(newReq.Tags, "retries:1")
	}

	return c.NewTask(ctx, newReq)
}
