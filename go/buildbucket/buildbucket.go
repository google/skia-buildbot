// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"context"
	"net/http"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/buildbucket/common"
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

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error) {
	pred, err := common.GetTrybotsForCLPredicate(issue, patchset, gerritUrl)
	if err != nil {
		return nil, err
	}
	return c.Search(ctx, pred)
}

// Make sure Client fulfills the BuildBucketInterface interface.
var _ BuildBucketInterface = (*Client)(nil)
