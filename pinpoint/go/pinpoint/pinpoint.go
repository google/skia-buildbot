package pinpoint

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/pinpoint/internal"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type Client struct {
	legacyClient *internal.LegacyClient
}

// New returns a new PinpointClient instance.
func New(ctx context.Context) (*Client, error) {
	legacyClient, err := internal.NewLegacyClient(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Client{legacyClient: legacyClient}, nil
}

// CreateTryJob calls the legacy pinpoint API to create a try job.
func (c *Client) CreateTryJob(
	ctx context.Context,
	req *TryJobCreateRequest,
) (*CreatePinpointResponse, error) {
	resp, err := c.legacyClient.CreateTryJob(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// CreateBisect calls pinpoint API to create bisect job.
func (c *Client) CreateBisect(
	ctx context.Context,
	req *BisectJobCreateRequest,
	isNewAnomaly bool,
) (*CreatePinpointResponse, error) {
	resp, err := c.legacyClient.CreateBisect(ctx, req, isNewAnomaly)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// FetchJobState retrieve job state details.
func (c *Client) FetchJobState(
	ctx context.Context,
	req internal.FetchJobStateRequest,
) (*FetchJobStateResponse, error) {
	resp, err := c.legacyClient.FetchJobState(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// QueryJobList retrieves a list of Pinpoint jobs using the QueryJobListRequest filters.
func (c *Client) QueryJobList(
	ctx context.Context,
	req *pb.QueryJobListRequest,
) (*pb.QueryJobListResponse, error) {
	resp, err := c.legacyClient.QueryJobList(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// CreatePinpointTryJob calls the legacy pinpoint API to create a try job.
func (c *Client) CreatePinpointTryJob(
	ctx context.Context,
	req *pb.CreateTryJobRequest,
) (*pb.CreateJobResponse, error) {
	resp, err := c.legacyClient.CreatePinpointTryJob(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// ListBotConfigurations retrieves the list of available bots.
func (c *Client) ListBotConfigurations(ctx context.Context) ([]string, error) {
	resp, err := c.legacyClient.ListBotConfigurations(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// ListBenchmarks retrieves the list of available benchmarks.
func (c *Client) ListBenchmarks(ctx context.Context) ([]string, error) {
	resp, err := c.legacyClient.ListBenchmarks(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

// GetBenchmarkInfo retrieves stories and story tags for a given benchmark.
func (c *Client) GetBenchmarkInfo(ctx context.Context, benchmark string) (*BenchmarkInfo, error) {
	info, err := c.legacyClient.GetBenchmarkInfo(ctx, benchmark)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return info, nil
}

// ListRecentBuilds retrieves the recent builds for a given configuration/bot.
func (c *Client) ListRecentBuilds(ctx context.Context, configuration string) ([]*pb.BuildInfo, error) {
	builds, err := c.legacyClient.ListRecentBuilds(ctx, configuration)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return builds, nil
}

// GetCommit retrieves details of a git commit.
func (c *Client) GetCommit(
	ctx context.Context,
	commit string,
) (*pb.GetCommitResponse, error) {
	resp, err := c.legacyClient.GetCommit(ctx, commit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}
