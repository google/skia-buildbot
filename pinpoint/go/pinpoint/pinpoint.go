package pinpoint

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/pinpoint/internal"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type Client struct {
	legacyClient     *internal.LegacyClient
	gerritHttpClient *http.Client
}

// New returns a new PinpointClient instance.
func New(ctx context.Context) (*Client, error) {
	legacyClient, err := internal.NewLegacyClient(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create Gerrit token source")
	}
	gerritHttpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	return &Client{
		legacyClient:     legacyClient,
		gerritHttpClient: gerritHttpClient,
	}, nil
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

// GetPatch retrieves details of a Gerrit patch.
func (c *Client) GetPatch(
	ctx context.Context,
	req *pb.GetPatchRequest,
) (*pb.GetPatchResponse, error) {
	if !isValidGerritHost(req.Host) {
		return nil, skerr.Fmt("Invalid or untrusted Gerrit host: %s", req.Host)
	}

	gclient, err := gerrit.NewGerrit(req.Host, c.gerritHttpClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create Gerrit client for %s", req.Host)
	}

	changeInfo, err := gclient.GetChange(ctx, strconv.FormatInt(req.Change, 10))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get change %d from Gerrit", req.Change)
	}

	targetRevision, err := getTargetRevision(changeInfo, req.Patchset)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &pb.GetPatchResponse{
		Host:     req.Host,
		Change:   req.Change,
		Patchset: targetRevision.Number,
		Project:  changeInfo.Project,
		Author:   changeInfo.Owner.Email,
		Subject:  changeInfo.Subject,
		Created:  timestamppb.New(targetRevision.Created),
	}, nil
}

func isValidGerritHost(host string) bool {
	u, err := url.Parse(host)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	return strings.HasSuffix(u.Hostname(), ".googlesource.com") ||
		strings.HasSuffix(u.Hostname(), ".git.corp.google.com")
}

func getTargetRevision(changeInfo *gerrit.ChangeInfo, patchset *int64) (*gerrit.Revision, error) {
	if len(changeInfo.Revisions) == 0 {
		return nil, skerr.Fmt("No patchsets found in change %d", changeInfo.Issue)
	}

	if patchset != nil {
		for _, rev := range changeInfo.Revisions {
			if rev.Number == *patchset {
				return rev, nil
			}
		}
		return nil, skerr.Fmt("Patchset %d not found in change %d", *patchset, changeInfo.Issue)
	}

	var targetRevision *gerrit.Revision
	for _, rev := range changeInfo.Revisions {
		if targetRevision == nil || rev.Number > targetRevision.Number {
			targetRevision = rev
		}
	}
	return targetRevision, nil
}
