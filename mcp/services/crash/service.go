package crash

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/common"
	"google.golang.org/protobuf/encoding/protojson"

	pb "go.skia.org/infra/mcp/services/crash/proto"
)

type CrashService struct {
}

// Initialize the service with the provided arguments.
func (s CrashService) Init(serviceArgs string) error {
	return nil
}

// GetTools returns the supported tools by the service.
func (s CrashService) GetTools() []common.Tool {
	return []common.Tool{
		{
			Name:        "Predator",
			Description: "Sends a Predator request to search for culprit CLs in a regression range.",
			Arguments: []common.ToolArgument{
				{
					Name: "stacktrace",
					Description: "The stacktrace to find a culprit for." +
						" Each frame in the stack has fields " +
						"<frame index> <address> in <symbol> <source file>:<line number> e.g. " +
						"#0 0x12125186c in foo() /path/to/foo.cc:12. Prepare the stacktrace in this format.",
					Required: true,
				},
				{
					Name: "last_good_version",
					Description: "The last Chrome version without the crash. Chrome versions are formatted like " +
						"123.0.456.0.",
					Required: true,
				},
				{
					Name: "first_bad_version",
					Description: "The first Chrome version with the crash.  Chrome versions are formatted like " +
						"123.0.456.0.",
					Required: true,
				},
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				stacktrace, err := request.RequireString("stacktrace")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				stacktrace = "CRASHED [ERROR @ 0x123]\n" + stacktrace
				last_good_version, err := request.RequireString("last_good_version")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				first_bad_version, err := request.RequireString("first_bad_version")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				dashboard_url, err := sendPredatorRequest(stacktrace, last_good_version, first_bad_version)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				return mcp.NewToolResultText(fmt.Sprintf(
					"Looking between %s and %s for a culprit for %s."+
						"Your results should appear at https://predator-for-me.appspot.com/cracas/dashboard with signature %s",
					last_good_version, first_bad_version, stacktrace, dashboard_url)), nil
			},
		},
	}
}

func (s *CrashService) Shutdown() error {
	return nil
}

func (s *CrashService) GetResources() []common.Resource {
	return []common.Resource{}
}

func sendPredatorRequest(stacktrace, last_good_version, first_bad_version string) (string, error) {
	projectID := "predator-for-me"
	topicID := "cracas"

	last_good_version_metadata := &pb.PredatorRequest_CustomizedData_HistoricalMetaData{
		CrashCount:    0,
		Cpm:           0.01,
		ChromeVersion: last_good_version,
	}

	first_bad_version_metadata := &pb.PredatorRequest_CustomizedData_HistoricalMetaData{
		CrashCount:    1,
		Cpm:           1,
		ChromeVersion: first_bad_version,
	}

	id := uuid.NewString()

	predator_request := &pb.PredatorRequest{
		StackTrace:    []string{stacktrace},
		ChromeVersion: first_bad_version,
		Platform:      "all",
		ClientId:      "cracas",
		Signature:     id,
		CustomizedData: &pb.PredatorRequest_CustomizedData{
			Channel: "dev",
			HistoricalMetadata: []*pb.PredatorRequest_CustomizedData_HistoricalMetaData{
				last_good_version_metadata,
				first_bad_version_metadata,
			},
		},
		CrashIdentifiers: &pb.CrashIdentifiers{
			Uuid: id,
		},
	}
	jsonBytes, err := protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "    ",
		UseProtoNames: true,
	}.Marshal(predator_request)
	if err != nil {
		return "", err
	}

	err = publishMessage(projectID, topicID, jsonBytes)
	if err != nil {
		return "", err
	}
	return id, nil
}

// publishMessage publishes a message to a Pub/Sub topic.
func publishMessage(projectID, topicID string, msg []byte) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	t := client.Topic(topicID)
	result := t.Publish(ctx, &pubsub.Message{
		Data: []byte(msg),
	})

	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("Get: %v", err)
	}
	sklog.Infof("Published a message %s; msg ID: %v\n", string(msg), id)
	return nil
}
