// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucketgrpcpb "go.chromium.org/luci/buildbucket/proto/grpcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	structpb "google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/buildbucket/common"
	"go.skia.org/infra/go/skerr"
)

const (
	BUILD_URL_TMPL = "https://%s/build/%d"
	DEFAULT_HOST   = "cr-buildbucket.appspot.com"
	headerToken    = "x-buildbucket-token"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

type BuildBucketInterface interface {
	// CancelBuild cancels the specified build with the specified
	// SummaryMarkdown.
	CancelBuild(ctx context.Context, buildID int64, summaryMarkdown string) (*buildbucketpb.Build, error)
	// CancelBuilds cancels the specified buildIDs with the specified
	// summaryMarkdown. Builds are cancelled with one batch request
	// to buildbucket.
	CancelBuilds(ctx context.Context, buildIDs []int64, summaryMarkdown string) ([]*buildbucketpb.Build, error)
	// GetBuild retrieves the build with the given ID.
	GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error)
	// GetTrybotsForCL retrieves trybot results for the given CL using the
	// optional tags.
	GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string, tags map[string]string) ([]*buildbucketpb.Build, error)
	// ScheduleBuilds schedules the specified builds on the given CL. Builds are
	// scheduled with one batch request to buildbucket.
	// builds is the slice of which builds should be scheduled by buildbucket.
	// Eg: ["Infra-PerCommit-Race", "Infra-PerCommit-Small"].
	// buildsToTags is the map of which tags to use when scheduling
	// some of the builds. Eg: {"Infra-PerCommit-Race": {"triggered_by": "skcq"}}
	// means that the Infra-PerCommit-Race build should be scheduled with the
	// "triggered_by: skcq" tag.
	ScheduleBuilds(ctx context.Context, builds []string, buildsToTags map[string]map[string]string, issue, patchset int64, gerritUrl, repo, bbProject, bbBucket string) ([]*buildbucketpb.Build, error)
	// Search retrieves Builds which match the given criteria.
	Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error)
	// UpdateBuild sends an update for the given build.
	UpdateBuild(ctx context.Context, build *buildbucketpb.Build, token string) error
	// StartBuild notifies Buildbucket that the build has started. Returns the
	// token which should be passed to UpdateBuild for subsequent calls.
	StartBuild(ctx context.Context, buildId int64, taskId, token string) (string, error)

	// Builder related functionality.
	GetBuilder(ctx context.Context, in *buildbucketpb.GetBuilderRequest, opts ...grpc.CallOption) (*buildbucketpb.BuilderItem, error)
}

// Client is used for interacting with the BuildBucket API.
type Client struct {
	builds   buildbucketgrpcpb.BuildsClient
	builders buildbucketgrpcpb.BuildersClient
	host     string
}

// NewClient returns an authenticated Client instance.
func NewClient(c *http.Client) *Client {
	host := DEFAULT_HOST
	prpcClient := &prpc.Client{
		C:    c,
		Host: host,
	}
	return &Client{
		builds:   buildbucketgrpcpb.NewBuildsClient(prpcClient),
		builders: buildbucketgrpcpb.NewBuildersClient(prpcClient),
		host:     host,
	}
}

// NewTestingClient lets the MockClient inject a mock BuildsClient and host.
func NewTestingClient(bc buildbucketgrpcpb.BuildsClient, host string) *Client {
	return &Client{
		builds: bc,
		host:   host,
	}
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error) {
	b, err := c.builds.GetBuild(ctx, &buildbucketpb.GetBuildRequest{
		Id:     buildId,
		Fields: common.GetBuildFields,
	})
	return b, skerr.Wrap(err)
}

// ScheduleBuilds implements the BuildBucketInterface.
func (c *Client) ScheduleBuilds(ctx context.Context, builds []string, buildsToTags map[string]map[string]string, issue, patchset int64, gerritURL, repo, bbProject, bbBucket string) ([]*buildbucketpb.Build, error) {
	requests := []*buildbucketpb.BatchRequest_Request{}
	for _, b := range builds {
		tagStringPairs := []*buildbucketpb.StringPair{}
		tags, ok := buildsToTags[b]
		if ok {
			for n, v := range tags {
				stringPair := &buildbucketpb.StringPair{
					Key:   n,
					Value: v,
				}
				tagStringPairs = append(tagStringPairs, stringPair)
			}
		}
		request := &buildbucketpb.BatchRequest_Request{
			Request: &buildbucketpb.BatchRequest_Request_ScheduleBuild{
				ScheduleBuild: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{
						Project: bbProject,
						Bucket:  bbBucket,
						Builder: b,
					},
					GerritChanges: []*buildbucketpb.GerritChange{
						{
							Host:     gerritURL,
							Project:  repo,
							Change:   issue,
							Patchset: patchset,
						},
					},
					Properties: &structpb.Struct{},
					Tags:       tagStringPairs,
					Fields:     common.GetBuildFields,
				},
			},
		}
		requests = append(requests, request)
	}

	resp, err := c.builds.Batch(ctx, &buildbucketpb.BatchRequest{
		Requests: requests,
	})
	if err != nil {
		return nil, skerr.Fmt("Could not schedule builds on buildbucket: %s", err)
	}
	if len(resp.Responses) != len(builds) {
		return nil, skerr.Fmt("Buildbucket gave %d responses for %d builders", len(resp.Responses), len(builds))
	}

	respBuilds := []*buildbucketpb.Build{}
	for _, r := range resp.Responses {
		respBuilds = append(respBuilds, r.GetScheduleBuild())
	}
	return respBuilds, nil
}

// CancelBuild implements BuildbucketInterface.
func (c *Client) CancelBuild(ctx context.Context, buildID int64, summaryMarkdown string) (*buildbucketpb.Build, error) {
	build, err := c.builds.CancelBuild(ctx, &buildbucketpb.CancelBuildRequest{
		Id:              buildID,
		SummaryMarkdown: summaryMarkdown,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return build, nil
}

// CancelBuilds implements the BuildBucketInterface.
func (c *Client) CancelBuilds(ctx context.Context, buildIDs []int64, summaryMarkdown string) ([]*buildbucketpb.Build, error) {
	requests := []*buildbucketpb.BatchRequest_Request{}
	for _, bID := range buildIDs {
		request := &buildbucketpb.BatchRequest_Request{
			Request: &buildbucketpb.BatchRequest_Request_CancelBuild{
				CancelBuild: &buildbucketpb.CancelBuildRequest{
					Id:              bID,
					SummaryMarkdown: summaryMarkdown,
				},
			},
		}
		requests = append(requests, request)
	}

	resp, err := c.builds.Batch(ctx, &buildbucketpb.BatchRequest{
		Requests: requests,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not cancel builds on buildbucket")
	}
	if len(resp.Responses) != len(buildIDs) {
		return nil, skerr.Fmt("Buildbucket gave %d responses for %d builders", len(resp.Responses), len(buildIDs))
	}

	respBuilds := []*buildbucketpb.Build{}
	for _, r := range resp.Responses {
		respBuilds = append(respBuilds, r.GetCancelBuild())
	}
	return respBuilds, nil
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error) {
	rv := []*buildbucketpb.Build{}
	cursor := ""
	for {
		req := &buildbucketpb.SearchBuildsRequest{
			Fields:    common.SearchBuildsFields,
			PageToken: cursor,
			Predicate: pred,
		}
		resp, err := c.builds.SearchBuilds(ctx, req)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if resp == nil {
			break
		}
		rv = append(rv, resp.Builds...)
		cursor = resp.NextPageToken
		if cursor == "" {
			break
		}
	}
	return rv, nil
}

// GetTrybotsForCL implements the BuildBucketInterface.
func (c *Client) GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string, tags map[string]string) ([]*buildbucketpb.Build, error) {
	pred, err := common.GetTrybotsForCLPredicate(issue, patchset, gerritUrl, tags)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.Search(ctx, pred)
}

func contextWithTokenMetadata(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, map[string][]string{
		headerToken: {token},
	})
}

// StartBuild implements BuildbucketInterface.
func (c *Client) StartBuild(ctx context.Context, buildId int64, taskId, token string) (string, error) {
	resp, err := c.builds.StartBuild(contextWithTokenMetadata(ctx, token), &buildbucketpb.StartBuildRequest{
		RequestId: uuid.New().String(),
		BuildId:   buildId,
		TaskId:    taskId,
	})
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if resp.UpdateBuildToken == "" {
		return "", skerr.Fmt("StartBuild returned an empty UpdateBuildToken")
	}
	return resp.UpdateBuildToken, nil
}

// UpdateBuild implements BuildbucketInterface.
func (c *Client) UpdateBuild(ctx context.Context, build *buildbucketpb.Build, token string) error {
	var updatePaths []string
	if build.Output != nil {
		if build.Output.Properties != nil {
			updatePaths = append(updatePaths, "build.output.properties")
		}
		if build.Output.GitilesCommit != nil {
			updatePaths = append(updatePaths, "build.output.gitiles_commit")
		}
		if build.Output.Status != buildbucketpb.Status_STATUS_UNSPECIFIED {
			updatePaths = append(updatePaths, "build.output.status")
		}
		if build.Output.StatusDetails != nil {
			updatePaths = append(updatePaths, "build.output.status_details")
		}
		if build.Output.SummaryMarkdown != "" {
			updatePaths = append(updatePaths, "build.output.summary_markdown")
		}
	}
	if build.Status != buildbucketpb.Status_STATUS_UNSPECIFIED {
		updatePaths = append(updatePaths, "build.status")
	}
	if build.StatusDetails != nil {
		updatePaths = append(updatePaths, "build.status_details")
	}
	if len(build.Steps) > 0 {
		updatePaths = append(updatePaths, "build.steps")
	}
	if build.SummaryMarkdown != "" {
		updatePaths = append(updatePaths, "build.summary_markdown")
	}
	if len(build.Tags) > 0 {
		updatePaths = append(updatePaths, "build.tags")
	}
	if build.Infra != nil && build.Infra.Buildbucket != nil && build.Infra.Buildbucket.Agent != nil {
		if build.Infra.Buildbucket.Agent.Output != nil {
			updatePaths = append(updatePaths, "build.infra.buildbucket.agent.output")
		}
		if build.Infra.Buildbucket.Agent.Purposes != nil {
			updatePaths = append(updatePaths, "build.infra.buildbucket.agent.purposes")
		}
	}
	_, err := c.builds.UpdateBuild(contextWithTokenMetadata(ctx, token), &buildbucketpb.UpdateBuildRequest{
		Build: build,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: updatePaths,
		},
	})
	return skerr.Wrap(err)
}

func (c *Client) GetBuilder(ctx context.Context, in *buildbucketpb.GetBuilderRequest, opts ...grpc.CallOption) (*buildbucketpb.BuilderItem, error) {
	return c.builders.GetBuilder(ctx, in)
}

// Make sure Client fulfills the BuildBucketInterface interface.
var _ BuildBucketInterface = (*Client)(nil)
