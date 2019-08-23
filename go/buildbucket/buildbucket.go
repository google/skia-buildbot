// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"context"
	fmt "fmt"
	"net/http"
	"strconv"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/buildbucket/common"
)

const (
	BUILD_URL_TMPL = "https://%s/build/%d"
	apiUrl         = "cr-buildbucket.appspot.com"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

const (
	// Possible values for the Build.Status field.
	// See: https://chromium.googlesource.com/infra/luci/luci-go/+/master/common/api/buildbucket/buildbucket/v1/buildbucket-gen.go#317
	STATUS_COMPLETED = "COMPLETED"
	STATUS_SCHEDULED = "SCHEDULED"
	STATUS_STARTED   = "STARTED"

	// Possible values for the Build.Result field.
	// See: https://chromium.googlesource.com/infra/luci/luci-go/+/master/common/api/buildbucket/buildbucket/v1/buildbucket-gen.go#305
	RESULT_CANCELED = "CANCELED"
	RESULT_FAILURE  = "FAILURE"
	RESULT_SUCCESS  = "SUCCESS"
)

// Properties contains extra properties set when a Build is requested, as a
// blob of JSON data. These are set by the CQ or "git cl try" when requesting
// try jobs.
type Properties struct {
	Category       string `json:"category"`
	Gerrit         string `json:"patch_gerrit_url"`
	GerritIssue    int64  `json:"patch_issue"`
	GerritPatchset string `json:"patch_ref"`
	PatchProject   string `json:"patch_project"`
	PatchStorage   string `json:"patch_storage"`
	Reason         string `json:"reason"`
	Revision       string `json:"revision"`
	TryJobRepo     string `json:"try_job_repo"`
}

// Parameters provide extra information about a Build.
type Parameters struct {
	BuilderName string     `json:"builder_name"`
	Properties  Properties `json:"properties"`
}

// Build is a struct containing information about a build in BuildBucket.
type Build struct {
	Bucket     string      `json:"bucket"`
	Completed  time.Time   `json:"completed_ts"`
	CreatedBy  string      `json:"created_by"`
	Created    time.Time   `json:"created_ts"`
	Id         string      `json:"id"`
	Url        string      `json:"url"`
	Parameters *Parameters `json:"parameters"`
	Result     string      `json:"result"`
	Status     string      `json:"status"`
}

type BuildBucketInterface interface {
	// GetBuild retrieves the build with the given ID.
	GetBuild(ctx context.Context, buildId string) (*Build, error)
	// Search retrieves Builds which match the given criteria.
	Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*Build, error)
	// GetTrybotsForCL retrieves trybot results for the given CL.
	GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*Build, error)
}

// Client is used for interacting with the BuildBucket API.
type Client struct {
	bc   buildbucketpb.BuildsClient
	host string
}

// NewClient returns an authenticated Client instance.
func NewClient(c *http.Client) *Client {
	host := apiUrl
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

func (c *Client) convertBuild(b *buildbucketpb.Build) *Build {
	status := ""
	result := ""
	switch b.Status {
	case buildbucketpb.Status_STATUS_UNSPECIFIED:
		// ???
	case buildbucketpb.Status_SCHEDULED:
		status = STATUS_SCHEDULED
	case buildbucketpb.Status_STARTED:
		status = STATUS_STARTED
	case buildbucketpb.Status_SUCCESS:
		status = STATUS_COMPLETED
		result = RESULT_SUCCESS
	case buildbucketpb.Status_FAILURE:
		status = STATUS_COMPLETED
		result = RESULT_FAILURE
	case buildbucketpb.Status_INFRA_FAILURE:
		status = STATUS_COMPLETED
		result = RESULT_FAILURE
	case buildbucketpb.Status_CANCELED:
		status = STATUS_COMPLETED
		result = RESULT_CANCELED
	}
	created := time.Time{}
	if b.CreateTime != nil {
		created = time.Unix(b.CreateTime.Seconds, int64(b.CreateTime.Nanos)).UTC()
	}
	completed := time.Time{}
	if b.EndTime != nil {
		completed = time.Unix(b.EndTime.Seconds, int64(b.EndTime.Nanos)).UTC()
	}
	return &Build{
		Bucket:    b.Builder.Bucket,
		Completed: completed,
		CreatedBy: b.CreatedBy,
		Created:   created,
		Id:        fmt.Sprintf("%d", b.Id),
		Url:       fmt.Sprintf(BUILD_URL_TMPL, c.host, b.Id),
		Parameters: &Parameters{
			BuilderName: b.Builder.Builder,
			Properties: Properties{
				Category:       b.Input.Properties.Fields["category"].GetStringValue(),
				Gerrit:         b.Input.Properties.Fields["patch_gerrit_url"].GetStringValue(),
				GerritIssue:    int64(b.Input.Properties.Fields["patch_issue"].GetNumberValue()),
				GerritPatchset: fmt.Sprintf("%d", int64(b.Input.Properties.Fields["patch_set"].GetNumberValue())),
				PatchProject:   b.Input.Properties.Fields["patch_project"].GetStringValue(),
				PatchStorage:   b.Input.Properties.Fields["patch_storage"].GetStringValue(),
				Reason:         b.Input.Properties.Fields["reason"].GetStringValue(),
				Revision:       b.Input.Properties.Fields["revision"].GetStringValue(),
				TryJobRepo:     b.Input.Properties.Fields["try_job_repo"].GetStringValue(),
			},
		},
		Result: result,
		Status: status,
	}
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetBuild(ctx context.Context, buildId string) (*Build, error) {
	id, err := strconv.ParseInt(buildId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse build ID as int64: %s", err)
	}
	b, err := c.bc.GetBuild(ctx, &buildbucketpb.GetBuildRequest{
		Id:     id,
		Fields: common.GetBuildFields,
	})
	if err != nil {
		return nil, err
	}
	return c.convertBuild(b), nil
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*Build, error) {
	rv := []*Build{}
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
		for _, b := range resp.Builds {
			rv = append(rv, c.convertBuild(b))
		}
		cursor = resp.NextPageToken
		if cursor == "" {
			break
		}
	}
	return rv, nil
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*Build, error) {
	pred, err := common.GetTrybotsForCLPredicate(issue, patchset, gerritUrl)
	if err != nil {
		return nil, err
	}
	return c.Search(ctx, pred)
}

// Make sure Client fulfills the BuildBucketInterface interface.
var _ BuildBucketInterface = (*Client)(nil)
