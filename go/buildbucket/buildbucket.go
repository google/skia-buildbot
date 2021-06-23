// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"context"
	"net/http"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	structpb "google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/buildbucket/common"
	"go.skia.org/infra/go/skerr"
)

const (
	BUILD_URL_TMPL = "https://%s/build/%d"
	DEFAULT_HOST   = "cr-buildbucket.appspot.com"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

type BuildBucketInterface interface {
	// GetBuild retrieves the build with the given ID.
	GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error)
	// Search retrieves Builds which match the given criteria.
	Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error)
	// GetTrybotsForCL retrieves trybot results for the given CL.
	GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error)
	// ScheduleBuilds schedules the specified builds on the given CL. Builds are
	// scheduled with one batch request to buildbucket.
	// builds is the slice of which builds should be scheduled by buildbucket.
	// Eg: ["Infra-PerCommit-Race", "Infra-PerCommit-Small"].
	// buildsToTags is the map of which tags to use when scheduling
	// some of the builds. Eg: {"Infra-PerCommit-Race": {"triggered_by": "skcq"}}
	// means that the Infra-PerCommit-Race build should be scheduled with the
	// "triggered_by: skcq" tag.
	ScheduleBuilds(ctx context.Context, builds []string, buildsToTags map[string]map[string]string, issue, patchset int64, gerritUrl, repo, bbProject, bbBucket string) ([]*buildbucketpb.Build, error)
	// CancelBuilds cancels the specified buildIDs with the specified
	// summaryMarkdown. Builds are cancelled with one batch request
	// to buildbucket.
	CancelBuilds(ctx context.Context, buildIDs []int64, summaryMarkdown string) ([]*buildbucketpb.Build, error)
}

// Client is used for interacting with the BuildBucket API.
type Client struct {
	bc   buildbucketpb.BuildsClient
	host string
}

// NewClient returns an authenticated Client instance.
func NewClient(c *http.Client) *Client {
	host := DEFAULT_HOST
	return &Client{
		bc: buildbucketpb.NewBuildsPRPCClient(&prpc.Client{
			C:    c,
			Host: host,
		}),
		host: host,
	}
}

// NewTestingClient lets the MockClient inject a mock BuildsClient and host.
func NewTestingClient(bc buildbucketpb.BuildsClient, host string) *Client {
	return &Client{
		bc:   bc,
		host: host,
	}
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error) {
	b, err := c.bc.GetBuild(ctx, &buildbucketpb.GetBuildRequest{
		Id:     buildId,
		Fields: common.GetBuildFields,
	})
	return b, err
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

	resp, err := c.bc.Batch(ctx, &buildbucketpb.BatchRequest{
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

	resp, err := c.bc.Batch(ctx, &buildbucketpb.BatchRequest{
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
		resp, err := c.bc.SearchBuilds(ctx, req)
		if err != nil {
			return nil, err
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
func (c *Client) GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error) {
	pred, err := common.GetTrybotsForCLPredicate(issue, patchset, gerritUrl)
	if err != nil {
		return nil, err
	}
	return c.Search(ctx, pred)
}

// Make sure Client fulfills the BuildBucketInterface interface.
var _ BuildBucketInterface = (*Client)(nil)
