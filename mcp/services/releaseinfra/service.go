package releaseinfra

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/common"

	fmpb "google.golang.org/protobuf/types/known/fieldmaskpb"
	tspb "google.golang.org/protobuf/types/known/timestamppb"

	// The main Buildbucket v2 client library.
	bbpb "go.chromium.org/luci/buildbucket/proto"
	// The PRPC client is used to communicate with LUCI services.

	"go.chromium.org/luci/grpc/prpc"
	"golang.org/x/oauth2/google"
)

var builders = map[string]*bbpb.BuilderID{
	"chrome-best-revision-continuous": {
		Project: "chrome",
		Bucket:  "official.infra",
		Builder: "chrome-best-revision-continuous",
	},
	"linux64-tbi": {
		Project: "chrome",
		Bucket:  "official.tbi",
		Builder: "linux64-tbi",
	},
}

const (
	BuildbucketDefaultHost = "cr-buildbucket.appspot.com"

	ArgStartTime = "range_start_time"
	ArgEndTime   = "range_end_time"
	ArgBuilder   = "builder"
)

type ReleaseInfraService struct {
	// The Buildbucket client used to interact with the Buildbucket service.
	BuildsClient bbpb.BuildsClient
}

// Initialize the service with the provided arguments.
func (s *ReleaseInfraService) Init(serviceArgs string) error {
	ctx := context.Background()
	var err error
	s.BuildsClient, err = CreateBuildsClient(ctx)
	if err != nil {
		return err
	}
	return nil
}

// GetTools returns the supported tools by the service.
func (s ReleaseInfraService) GetTools() []common.Tool {
	return []common.Tool{
		{
			Name:        "find_builds",
			Description: "Find Chrome official builds by builder name and range of timestamps.",
			Arguments: []common.ToolArgument{
				{
					Name:        ArgBuilder,
					Description: "[Required] Name of the LUCI builder name",
					Required:    true,
				},
				{
					Name:        ArgStartTime,
					Description: "[Optional] Start of the time range to search for builds, Epoch seconds as INT64.",
					Required:    false,
				},
				{
					Name:        ArgEndTime,
					Description: "[Optional] End of the time range to search for builds, Epoch seconds as INT64.",
					Required:    false,
				},
			},
			Handler: s.FindBuildsHandler,
		},
	}
}

func (s *ReleaseInfraService) GetResources() []common.Resource {
	return []common.Resource{}
}

func (s *ReleaseInfraService) Shutdown() error {
	return nil
}

func (s ReleaseInfraService) FindBuildsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	builder, err := request.RequireString(ArgBuilder)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rangeStartSecs := request.GetString(ArgStartTime, "")
	rangeEndSecs := request.GetString(ArgEndTime, "")
	sklog.Debugf("Arg %s is: %s", ArgBuilder, builder)
	sklog.Debugf("Arg %s is: %s", ArgStartTime, rangeStartSecs)
	sklog.Debugf("Arg %s is: %s", ArgEndTime, rangeEndSecs)

	builderID, ok := builders[builder]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown builder: %s", builder)), nil
	}

	// --- Define the Search Predicate ---
	// The predicate defines what you are searching for.
	buildPredicate := &bbpb.BuildPredicate{
		Builder: builderID,
	}
	// TODO(keybo): implement page fetch with smaller page size. 1000 is the upper limit.
	pageSize := 10

	// Set the optional filter on Create_Time.
	if rangeStartSecs != "" || rangeEndSecs != "" {
		buildPredicate.CreateTime = &bbpb.TimeRange{}
		if rangeStartSecs != "" {
			start, err := strconv.ParseInt(rangeStartSecs, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("%s not an INT64: %s", ArgStartTime, rangeStartSecs)), nil
			}
			buildPredicate.CreateTime.StartTime = &tspb.Timestamp{
				Seconds: start,
			}
		}
		if rangeEndSecs != "" {
			end, err := strconv.ParseInt(rangeEndSecs, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("%s not an INT64: %s", ArgEndTime, rangeEndSecs)), nil
			}
			buildPredicate.CreateTime.EndTime = &tspb.Timestamp{
				Seconds: end,
			}
		}
		pageSize = 1000
	}

	// --- Create and Send the SearchBuilds Request ---
	req := &bbpb.SearchBuildsRequest{
		Predicate: buildPredicate,
		PageSize:  int32(pageSize),
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: []string{"id", "builder", "number", "status", "summary_markdown", "create_time", "input", "output"},
			},
		},
	}

	sklog.Info("Sending search request...")
	var buildsFound []*bbpb.Build
	resp, err := s.BuildsClient.SearchBuilds(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search builds: %v", err)), nil
	}

	sklog.Info("Processing response...")
	if len(resp.Builds) > 0 {
		buildsFound = append(buildsFound, resp.Builds...)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found builds: %v", buildsFound)), nil
}

func CreateBuildsClient(ctx context.Context) (bbpb.BuildsClient, error) {
	// --- Authenticate and create an HTTP client ---
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()

	return bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       c,
		Host:    BuildbucketDefaultHost,
		Options: &prpc.Options{Insecure: false}, // Always use TLS/SSL
	}), nil
}
